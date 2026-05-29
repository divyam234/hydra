package hydra

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
)

type probeInfo struct {
	URL           string
	Size          int64
	AcceptRanges  bool
	ETag          string
	LastModified  string
	Filename      string
	ContentType   string
	SupportsProbe bool
}

func (d *Downloader) probe(ctx context.Context, urls []string, headers http.Header) (probeInfo, error) {
	var last error
	for _, raw := range urls {
		info, err := d.probeOne(ctx, raw, headers)
		if err == nil {
			return info, nil
		}
		last = err
	}
	return probeInfo{}, last
}

func (d *Downloader) probeOne(ctx context.Context, raw string, headers http.Header) (probeInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
	if err != nil {
		return probeInfo{}, err
	}
	d.applyHeaders(req, headers)
	resp, err := d.client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			info := probeFromResponse(raw, resp, true)
			// Some servers support Range but do not advertise Accept-Ranges on HEAD.
			// Resume must not depend only on that header, so verify with
			// a one-byte ranged GET when the size is known.
			if d.opts.Split > 1 && !info.AcceptRanges && info.Size > 0 {
				if ranged, ok := d.probeRange(ctx, raw, headers, info); ok {
					return ranged, nil
				}
			}
			return info, nil
		}
		if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusForbidden {
			return probeInfo{}, &HTTPStatusError{URL: raw, StatusCode: resp.StatusCode, Status: resp.Status}
		}
	}

	// Fallback for servers/CDNs that block HEAD. Ask for one byte only.
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return probeInfo{}, err
	}
	d.applyHeaders(req, headers)
	req.Header.Set("Range", "bytes=0-0")
	resp, err = d.client.Do(req)
	if err != nil {
		return probeInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && (resp.StatusCode < 200 || resp.StatusCode >= 400) {
		return probeInfo{}, &HTTPStatusError{URL: raw, StatusCode: resp.StatusCode, Status: resp.Status}
	}
	info := probeFromResponse(raw, resp, true)
	if resp.StatusCode == http.StatusPartialContent {
		info.AcceptRanges = true
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			if slash := strings.LastIndex(cr, "/"); slash >= 0 {
				if n, err := strconv.ParseInt(strings.TrimSpace(cr[slash+1:]), 10, 64); err == nil {
					info.Size = n
				}
			}
		}
	}
	return info, nil
}

func (d *Downloader) probeRange(ctx context.Context, raw string, headers http.Header, base probeInfo) (probeInfo, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return probeInfo{}, false
	}
	d.applyHeaders(req, headers)
	req.Header.Set("Range", "bytes=0-0")
	resp, err := d.client.Do(req)
	if err != nil {
		return probeInfo{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return probeInfo{}, false
	}
	info := probeFromResponse(raw, resp, true)
	info.AcceptRanges = true
	if info.Filename == "download.bin" && base.Filename != "" {
		info.Filename = base.Filename
	}
	if info.ETag == "" {
		info.ETag = base.ETag
	}
	if info.LastModified == "" {
		info.LastModified = base.LastModified
	}
	if info.ContentType == "" {
		info.ContentType = base.ContentType
	}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if slash := strings.LastIndex(cr, "/"); slash >= 0 {
			if n, err := strconv.ParseInt(strings.TrimSpace(cr[slash+1:]), 10, 64); err == nil {
				info.Size = n
			}
		}
	}
	if info.Size <= 0 {
		info.Size = base.Size
	}
	return info, true
}

func probeFromResponse(raw string, resp *http.Response, ok bool) probeInfo {
	info := probeInfo{
		URL:           resp.Request.URL.String(),
		Size:          resp.ContentLength,
		AcceptRanges:  strings.EqualFold(resp.Header.Get("Accept-Ranges"), "bytes"),
		ETag:          resp.Header.Get("ETag"),
		LastModified:  resp.Header.Get("Last-Modified"),
		Filename:      filenameFromResponse(raw, resp.Header),
		ContentType:   resp.Header.Get("Content-Type"),
		SupportsProbe: ok,
	}
	return info
}

func filenameFromResponse(raw string, h http.Header) string {
	if cd := h.Get("Content-Disposition"); cd != "" {
		_, params, err := mime.ParseMediaType(cd)
		if err == nil {
			if fn := strings.TrimSpace(params["filename*"]); fn != "" {
				return path.Base(fn)
			}
			if fn := strings.TrimSpace(params["filename"]); fn != "" {
				return path.Base(fn)
			}
		}
	}
	u, err := url.Parse(raw)
	if err == nil {
		base := path.Base(u.Path)
		if base != "." && base != "/" && base != "" {
			return base
		}
	}
	return "download.bin"
}
