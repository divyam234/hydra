package hydra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Cancel a transfer after a fast worker steals a slow worker's tail.
// Resume must verify and reuse completed blocks, and fill all remaining
// bytes with no overlap or corruption from the interrupted response.
func TestAdaptiveStealCancellationAndResume(t *testing.T) {
	const mib = 1 << 20
	payload := make([]byte, 16*mib)
	for i := range payload {
		payload[i] = byte(i*13 + i/131)
	}
	stolenCh := make(chan struct{}, 1)
	var mu sync.Mutex
	var seenSteal bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"stable-resume-steal-v1"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		if r.Method == http.MethodHead {
			return
		}
		var start, end int
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil || start < 0 || end < start || end >= len(payload) {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if start > 8*mib {
			mu.Lock()
			seenSteal = true
			mu.Unlock()
			select {
			case stolenCh <- struct{}{}:
			default:
			}
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		if start == 8*mib {
			for at := start; at <= end; at += 32 << 10 {
				last := min(end+1, at+(32<<10))
				if _, err := w.Write(payload[at:last]); err != nil {
					return
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(3 * time.Millisecond)
			}
			return
		}
		_, _ = w.Write(payload[start : end+1])
	}))
	defer server.Close()

	opts := DefaultOptions()
	opts.Split = 2
	opts.MaxConnectionsPerServer = 2
	opts.MinSplitSize = mib
	opts.ResumeBlockSize = mib
	dir := t.TempDir()
	req := Request{URLs: []string{server.URL}, Dir: dir, Out: "resume.bin"}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Download(ctx, req, opts)
		done <- err
	}()

	select {
	case <-stolenCh:
		cancel()
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("no work stealing occurred before cancellation")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: got %v, want context.Canceled", err)
	}
	mu.Lock()
	wasStolen := seenSteal
	mu.Unlock()
	if !wasStolen {
		t.Fatal("did not observe stolen HTTP range")
	}
	final := filepath.Join(dir, "resume.bin")
	meta, err := loadMeta(sidecarPath(final))
	if err != nil {
		t.Fatalf("load resumable sidecar: %v", err)
	}
	if meta.completedBytes() <= 0 || meta.completedBytes() >= int64(len(payload)) {
		t.Fatalf("unexpected checkpointed progress: %d", meta.completedBytes())
	}
	result, err := Download(context.Background(), req, opts)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !result.Resumed {
		t.Fatal("expected resumable, verified blocks")
	}
	got, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("resumed content differs from source")
	}
}
