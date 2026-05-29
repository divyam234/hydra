package hydra

import (
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// Request describes one file download.
type Request struct {
	// ID is optional but useful for UIs/queues. Manager auto-fills it.
	ID string

	URLs []string
	Dir  string
	Out  string

	// Per-download headers. Downloader Options.Headers are applied first.
	Headers http.Header

	// Checksum verifies the final file after download/skip. It may be filled
	// directly or parsed from CLI/API input with ParseChecksum.
	Checksum Checksum

	// ExpectedSize optionally rejects unexpected remote sizes when the server can
	// report size. Zero means unknown/unconstrained.
	ExpectedSize int64

	// ExistingFile optionally overrides Options.ExistingFile for this request.
	ExistingFile ExistingFilePolicy
}

func (r Request) normalized() (Request, error) {
	if len(r.URLs) == 0 {
		return r, errors.New("at least one URL is required")
	}
	clean := make([]string, 0, len(r.URLs))
	for _, raw := range r.URLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			return r, err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return r, errors.New("only http/https URLs are supported: " + raw)
		}
		if u.Host == "" {
			return r, errors.New("URL missing host: " + raw)
		}
		clean = append(clean, raw)
	}
	if len(clean) == 0 {
		return r, errors.New("at least one non-empty URL is required")
	}
	r.URLs = clean
	if r.Dir == "" {
		r.Dir = "."
	}
	if r.Headers == nil {
		r.Headers = make(http.Header)
	}
	if r.Out != "" && filepath.IsAbs(r.Out) {
		r.Dir = filepath.Dir(r.Out)
		r.Out = filepath.Base(r.Out)
	}
	if r.ExpectedSize < 0 {
		return r, errors.New("ExpectedSize must be >= 0")
	}
	if !r.Checksum.Empty() {
		if _, err := r.Checksum.normalized().newHash(); err != nil {
			return r, err
		}
	}
	return r, nil
}
