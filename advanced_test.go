package hydra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestChecksumVerification(t *testing.T) {
	data := testData(256 << 10)
	sum := sha256.Sum256(data)
	srv := rangeServer(data, true, nil)
	defer srv.Close()

	dir := t.TempDir()
	res, err := Download(context.Background(), Request{
		URLs:     []string{srv.URL + "/sum.bin"},
		Dir:      dir,
		Checksum: Checksum{Algorithm: "sha256", Value: fmt.Sprintf("%x", sum)},
	}, Options{Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Verified || res.ChecksumActual == "" {
		t.Fatalf("expected verified result: %+v", res)
	}

	_, err = Download(context.Background(), Request{
		URLs:         []string{srv.URL + "/sum-bad.bin"},
		Dir:          dir,
		Out:          "bad.bin",
		Checksum:     Checksum{Algorithm: "sha256", Value: stringsRepeat("0", 64)},
		ExistingFile: ExistingFileOverwrite,
	}, Options{Split: 1})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestExistingFilePolicies(t *testing.T) {
	data := testData(128 << 10)
	srv := rangeServer(data, true, nil)
	defer srv.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/exists.bin"}, Dir: dir, Out: "exists.bin"}, Options{ExistingFile: ExistingFileSkip})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped || res.Size != int64(len(data)) {
		t.Fatalf("expected skipped existing result: %+v", res)
	}

	_, err = Download(context.Background(), Request{URLs: []string{srv.URL + "/exists.bin"}, Dir: dir, Out: "exists.bin"}, Options{ExistingFile: ExistingFileError})
	if err == nil {
		t.Fatal("expected existing file error")
	}
}

func TestManagerQueue(t *testing.T) {
	data := testData(64 << 10)
	srv := rangeServer(data, true, nil)
	defer srv.Close()
	dir := t.TempDir()

	m, err := NewManager(context.Background(), Options{MaxConcurrentDownloads: 2, Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	id1, err := m.Enqueue(Request{ID: "a", URLs: []string{srv.URL + "/a.bin"}, Dir: dir, Out: "a.bin"})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := m.Enqueue(Request{ID: "b", URLs: []string{srv.URL + "/b.bin"}, Dir: dir, Out: "b.bin"})
	if err != nil {
		t.Fatal(err)
	}
	for _, jr := range m.WaitAll(context.Background(), id1, id2) {
		if jr.Err != nil {
			t.Fatalf("job %s failed: %v", jr.ID, jr.Err)
		}
		got, err := os.ReadFile(jr.Result.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data) {
			t.Fatalf("job %s mismatch", jr.ID)
		}
	}
}

func TestRetryEventAndRecovery(t *testing.T) {
	data := testData(96 << 10)
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		if gets.Add(1) == 1 {
			http.Error(w, "try again", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	var retryEvents atomic.Int64
	dir := t.TempDir()
	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/retry.bin"}, Dir: dir}, Options{
		Split: 1, MaxRetries: 2, RetryWait: time.Millisecond,
		OnEvent: func(e Event) {
			if e.Kind == EventRetrying {
				retryEvents.Add(1)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(res.Path)
	if !bytes.Equal(got, data) {
		t.Fatal("retry recovery mismatch")
	}
	if retryEvents.Load() == 0 {
		t.Fatal("expected retry event")
	}
}

func TestHTTPProxy(t *testing.T) {
	data := testData(512 << 10)
	origin := rangeServer(data, true, nil)
	defer origin.Close()
	var proxied atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Add(1)
		out, err := http.NewRequestWithContext(r.Context(), r.Method, r.URL.String(), nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out.Header = r.Header.Clone()
		resp, err := http.DefaultTransport.RoundTrip(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vals := range resp.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	defer proxy.Close()

	dir := t.TempDir()
	res, err := Download(context.Background(), Request{URLs: []string{origin.URL + "/proxy.bin"}, Dir: dir}, Options{Proxy: proxy.URL, Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(res.Path)
	if !bytes.Equal(got, data) {
		t.Fatal("proxy mismatch")
	}
	if proxied.Load() == 0 {
		t.Fatal("expected traffic through proxy")
	}
}

func stringsRepeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func TestEventsAreEmitted(t *testing.T) {
	data := testData(64 << 10)
	srv := rangeServer(data, true, nil)
	defer srv.Close()
	var mu sync.Mutex
	seen := map[EventKind]bool{}
	_, err := Download(context.Background(), Request{ID: "events", URLs: []string{srv.URL + "/events.bin"}, Dir: t.TempDir()}, Options{
		Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1,
		OnEvent: func(e Event) {
			mu.Lock()
			seen[e.Kind] = true
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []EventKind{EventStarted, EventProbing, EventDownloading, EventPieceDone, EventCompleted} {
		if !seen[k] {
			t.Fatalf("missing event %s in %#v", k, seen)
		}
	}
}

func TestProgressRollingSpeedDoesNotUsePieceEmitInterval(t *testing.T) {
	tracker := newProgressTracker("job", 100<<20, 4, 100, "file.bin", time.Hour, nil)
	tracker.setCompleted(0)
	tracker.add(1 << 20)
	// Force many piece/block emits; these used to reset instantaneous speed and
	// make the next terminal sample jump wildly.
	for i := 0; i < 20; i++ {
		tracker.addPiecesDone(1)
	}
	time.Sleep(5 * time.Millisecond)
	tracker.add(1 << 20)
	p := tracker.emit(StateDownloading, "", true)
	if p.Speed <= 0 {
		t.Fatalf("expected rolling speed > 0")
	}
	if p.Downloaded != 2<<20 {
		t.Fatalf("downloaded=%d, want only current-process bytes", p.Downloaded)
	}
}

func TestCustomOutputFilename(t *testing.T) {
	data := testData(128 << 10)
	srv := rangeServer(data, true, nil)
	defer srv.Close()

	dir := t.TempDir()
	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/generated-name"}, Dir: dir, Out: "custom-output.bin"}, Options{Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(res.Path) != "custom-output.bin" {
		t.Fatalf("path=%q, want custom output filename", res.Path)
	}
	got, err := os.ReadFile(filepath.Join(dir, "custom-output.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("custom output file mismatch")
	}
}
