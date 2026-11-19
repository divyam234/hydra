package hydra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAdaptiveSchedulerStealsOnlyUnreservedAlignedTail(t *testing.T) {
	const block = int64(1 << 20)
	initial := []piece{
		{Index: 0, Start: 0, End: 8*block - 1},
		{Index: 1, Start: 8 * block, End: 16*block - 1},
	}
	s := newRangeScheduler(initial, block, block)
	left, right := s.claim(), s.claim()
	if left == nil || right == nil {
		t.Fatal("did not assign both initial ranges")
	}
	if got := right.reserve(right.job.Start, int(block/4)); got != int(block/4) {
		t.Fatalf("reserve got %d", got)
	}
	// Left finishes first; it can now steal a checkpoint-aligned, unwritten
	// tail of the still-active right-hand lease.
	s.finish(left)
	stolen := s.claim()
	if stolen == nil {
		t.Fatal("idle worker could not steal slow worker's unread tail")
	}
	if stolen.job.Start%block != 0 || stolen.job.Start <= right.reservedEnd {
		t.Fatalf("stolen range overlaps reserved bytes: %+v, reserved=%d", stolen.job, right.reservedEnd)
	}
	if got := right.limit(); got+1 != stolen.job.Start {
		t.Fatalf("gap or overlap: old end=%d new start=%d", got, stolen.job.Start)
	}
	if got := right.reserve(stolen.job.Start, int(block)); got != 0 {
		t.Fatalf("old owner could reserve %d bytes already stolen", got)
	}
	if got := stolen.reserve(stolen.job.Start, int(block)); got != int(block) {
		t.Fatalf("new owner cannot reserve stolen bytes: %d", got)
	}
	s.finish(right)
	s.finish(stolen)
	if got := s.claim(); got != nil {
		t.Fatalf("claim on drained scheduler returned %+v", got)
	}
}

func TestAdaptiveSchedulerWonNotSplitOutstandingSocketRead(t *testing.T) {
	const block = int64(1 << 20)
	p := piece{Index: 0, Start: 0, End: 8*block - 1}
	s := newRangeScheduler([]piece{p}, block, block)
	lease := s.claim()
	if lease == nil {
		t.Fatal("missing lease")
	}
	// Simulate an in-flight read while a second goroutine splits its tail.
	// Every byte reserved before the split must remain with the old owner.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for off := int64(0); off < 2*block; off += block / 4 {
			_ = lease.reserve(off, int(block/4))
		}
	}()
	stolen := s.claim()
	wg.Wait()
	if stolen == nil {
		t.Fatal("could not split")
	}
	if stolen.job.Start <= lease.reservedEnd {
		t.Fatalf("overlap: stolen=%d reserved=%d", stolen.job.Start, lease.reservedEnd)
	}
	if stolen.job.Start%block != 0 {
		t.Fatalf("split not checkpoint aligned: %d", stolen.job.Start)
	}
}

// A real HTTP Range server keeps one initial connection slow. The fast
// worker should split and take a tail from the slow one. Range requests may
// be canceled early; the resulting output and each checkpoint must remain
// correct with no overlapping output writes.
func TestAdaptiveDownloadStealsSlowConnectionAndPreservesBytes(t *testing.T) {
	const mib = 1 << 20
	payload := make([]byte, 16*mib)
	for i := range payload {
		payload[i] = byte(i*29 + i/257)
	}
	wantHash := sha256.Sum256(payload)
	var mu sync.Mutex
	var ranges []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"stable-adaptive-test-v1"`)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		if r.Method == http.MethodHead {
			return
		}
		raw := r.Header.Get("Range")
		var start, end int
		if _, err := fmt.Sscanf(raw, "bytes=%d-%d", &start, &end); err != nil || start < 0 || end >= len(payload) || end < start {
			http.Error(w, "invalid range "+raw, http.StatusRequestedRangeNotSatisfiable)
			return
		}
		mu.Lock()
		ranges = append(ranges, raw)
		mu.Unlock()
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
		w.WriteHeader(http.StatusPartialContent)
		if start == 8*mib {
			// Only the original right-side range is slow. A stolen tail
			// gets a fresh fast GET, which demonstrates real work stealing.
			for at := start; at <= end; at += 64 << 10 {
				last := min(end+1, at+(64<<10))
				if _, err := w.Write(payload[at:last]); err != nil {
					return
				}
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(4 * time.Millisecond)
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
	opts.OnProgress = nil
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := Download(ctx, Request{URLs: []string{server.URL}, Dir: dir, Out: "adaptive.bin"}, opts)
	if err != nil {
		t.Fatalf("adaptive download: %v", err)
	}
	if res.Connections != 2 {
		t.Fatalf("connections=%d", res.Connections)
	}
	received, err := os.ReadFile(filepath.Join(dir, "adaptive.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(received, payload) || sha256.Sum256(received) != wantHash {
		t.Fatal("adaptive output did not match source bytes")
	}
	mu.Lock()
	got := append([]string(nil), ranges...)
	mu.Unlock()
	if len(got) < 3 {
		t.Fatalf("no adaptive steal occurred: HTTP ranges=%v", got)
	}
	stolen := false
	for _, raw := range got {
		var start, end int
		if _, err := fmt.Sscanf(strings.TrimSpace(raw), "bytes=%d-%d", &start, &end); err == nil && start > 8*mib {
			stolen = true
		}
	}
	if !stolen {
		t.Fatalf("no late tail range: %v", got)
	}
}
