package hydra

import (
	"fmt"
	"net/http"
	"strings"
)

func strongETag(tag string) bool {
	return len(tag) >= 2 && strings.HasPrefix(tag, `"`) && strings.HasSuffix(tag, `"`)
}

// Pin every body request, including retries and mirrors, to the probed object.
// Weak ETags cannot identify byte-for-byte equivalent range representations.
func representationHeaders(headers http.Header, info probeInfo) http.Header {
	h := cloneHeader(headers)
	if strongETag(info.ETag) {
		h.Set("If-Match", info.ETag)
	} else if info.LastModified != "" {
		h.Set("If-Unmodified-Since", info.LastModified)
	}
	return h
}

func validateRepresentation(headers http.Header, resp *http.Response) error {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil
	}
	if expected := headers.Get("If-Match"); strongETag(expected) {
		if got := resp.Header.Get("ETag"); got != expected {
			return fmt.Errorf("%w: ETag %q, expected %q", ErrRemoteChanged, got, expected)
		}
	}
	if expected := headers.Get("If-Unmodified-Since"); expected != "" {
		if got := resp.Header.Get("Last-Modified"); got != "" && got != expected {
			return fmt.Errorf("%w: Last-Modified %q, expected %q", ErrRemoteChanged, got, expected)
		}
	}
	return nil
}
