package hydra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadRejects204NoContent(t *testing.T) {
	const size = 8
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprint(size))
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	dir := t.TempDir()
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/empty.bin"}, Dir: dir}, Options{Split: 1})
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNoContent {
		t.Fatalf("error=%v, want HTTP 204 error", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "empty.bin")); !os.IsNotExist(statErr) {
		t.Fatalf("final file must not be published after 204, stat error=%v", statErr)
	}
}

func TestSegmentedDownloadRejectsMissingContentRange(t *testing.T) {
	data := []byte("0123456789abcdef")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	_, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/bad-range.bin"}, Dir: t.TempDir()}, Options{
		Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1,
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "content-range") {
		t.Fatalf("error=%v, want missing Content-Range failure", err)
	}
}

func TestResume204PreservesPartAndReturnsHTTPError(t *testing.T) {
	data := []byte("0123456789abcdef")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	dir := t.TempDir()
	final := filepath.Join(dir, "resume.bin")
	part := partPath(final)
	prefix := data[:8]
	if err := os.WriteFile(part, prefix, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/resume.bin"}, Out: final}, Options{Split: 1})
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNoContent {
		t.Fatalf("error=%v, want HTTP 204 error", err)
	}
	got, readErr := os.ReadFile(part)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != string(prefix) {
		t.Fatalf("part changed after 204: got %q want %q", got, prefix)
	}
}

func TestResumeRestartsSafelyWhenServerIgnoresRange(t *testing.T) {
	data := []byte("0123456789abcdef")
	var ranged bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Header.Get("Range") != "" {
			ranged = true
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	dir := t.TempDir()
	final := filepath.Join(dir, "restart.bin")
	if err := os.WriteFile(partPath(final), data[:8], 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/restart.bin"}, Out: final}, Options{Split: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !ranged {
		t.Fatal("expected an initial ranged resume request")
	}
	if res.Resumed {
		t.Fatal("result must report a clean restart, not a resume")
	}
	got, err := os.ReadFile(final)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("restarted download mismatch: got %q want %q", got, data)
	}
}

func TestValidateContentRangeRejectsWrongTotal(t *testing.T) {
	err := validateContentRange("bytes 8-15/32", 8, 15, 16)
	if err == nil || !strings.Contains(err.Error(), "total") {
		t.Fatalf("error=%v, want total mismatch", err)
	}
}
