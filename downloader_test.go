package hydra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSegmentedDownload(t *testing.T) {
	data := testData(3 << 20)
	var ranges atomic.Int64
	srv := rangeServer(data, true, &ranges)
	defer srv.Close()

	dir := t.TempDir()
	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/file.bin"}, Dir: dir}, Options{
		Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 1, RetryWait: time.Millisecond, MaxRetries: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("download mismatch: got %x want %x", sha256.Sum256(got), sha256.Sum256(data))
	}
	if res.Connections != 4 {
		t.Fatalf("connections=%d", res.Connections)
	}
	if ranges.Load() < 3 {
		t.Fatalf("expected range requests, got %d", ranges.Load())
	}
	if _, err := os.Stat(sidecarPath(res.Path)); !os.IsNotExist(err) {
		t.Fatalf("sidecar should be removed, stat err=%v", err)
	}
}

func TestSingleDownloadNoRange(t *testing.T) {
	data := testData(1 << 20)
	srv := rangeServer(data, false, nil)
	defer srv.Close()

	dir := t.TempDir()
	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/plain.bin"}, Dir: dir}, Options{Split: 8, MinSplitSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("download mismatch")
	}
	if res.Connections != 1 {
		t.Fatalf("connections=%d", res.Connections)
	}
}

func TestMetadataUsesCompactBitfield(t *testing.T) {
	data := testData(2 << 20)
	dir := t.TempDir()
	final := filepath.Join(dir, "bitfield.bin")
	info := probeInfo{URL: "http://example.test/bitfield.bin", Size: int64(len(data)), AcceptRanges: true}
	meta := newMeta([]string{info.URL}, final, info, 256<<10)
	if meta.Version != metaVersionBitfield {
		t.Fatalf("version=%d", meta.Version)
	}
	if meta.BlockCount != 8 {
		t.Fatalf("block count=%d", meta.BlockCount)
	}
	if len(meta.Pieces) != 0 {
		t.Fatalf("new metadata should not persist JSON pieces, got %d", len(meta.Pieces))
	}
	if err := meta.markDone(0, sidecarPath(final)); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadMeta(sidecarPath(final))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.completedBytes() != 256<<10 {
		t.Fatalf("completed=%d", loaded.completedBytes())
	}
	if got := loaded.pendingPieces(4, 1)[0].Start; got != 256<<10 {
		t.Fatalf("first missing block starts at %d", got)
	}
}

func TestSegmentedResumeFromMetadata(t *testing.T) {
	data := testData(2 << 20)
	srv := rangeServer(data, true, nil)
	defer srv.Close()
	dir := t.TempDir()
	final := filepath.Join(dir, "resume.bin")

	info := probeInfo{URL: srv.URL + "/resume.bin", Size: int64(len(data)), AcceptRanges: true}
	meta := newMeta([]string{srv.URL + "/resume.bin"}, final, info, 512<<10)
	if err := ensureParent(final); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(final, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(int64(len(data))); err != nil {
		t.Fatal(err)
	}
	first := meta.pendingPieces(4, 1)[0]
	if _, err := f.WriteAt(data[first.Start:first.End+1], first.Start); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := meta.markDone(first.Index, sidecarPath(final)); err != nil {
		t.Fatal(err)
	}

	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/resume.bin"}, Out: final}, Options{
		Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 1, RetryWait: time.Millisecond, MaxRetries: 2, ResumeBlockSize: 512 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed {
		t.Fatal("expected resumed result")
	}
	got, err := os.ReadFile(final)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("resumed content mismatch")
	}
}

func TestSegmentedResumeBitfieldBlockOffset(t *testing.T) {
	data := testData(2 << 20)
	var mu sync.Mutex
	var starts []int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method == http.MethodHead {
			return
		}
		if r.Header.Get("Range") != "" {
			start, end, ok := parseTestRange(r.Header.Get("Range"), int64(len(data)))
			if !ok {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			mu.Lock()
			starts = append(starts, start)
			mu.Unlock()
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[start : end+1])
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	dir := t.TempDir()
	final := filepath.Join(dir, "partial.bin")
	info := probeInfo{URL: srv.URL + "/partial.bin", Size: int64(len(data)), AcceptRanges: true}
	meta := newMeta([]string{srv.URL + "/partial.bin"}, final, info, 512<<10)
	first := meta.pendingPieces(4, 1)[0]
	resumeAt := first.End + 1

	f, err := os.OpenFile(final, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(data[first.Start:first.End+1], first.Start); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := meta.markDone(first.Index, sidecarPath(final)); err != nil {
		t.Fatal(err)
	}

	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/partial.bin"}, Out: final}, Options{
		Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 1, RetryWait: time.Millisecond, MaxRetries: 2, ResumeBlockSize: 512 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed {
		t.Fatal("expected resumed result from bitfield metadata")
	}
	got, err := os.ReadFile(final)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("bitfield resumed content mismatch")
	}

	mu.Lock()
	defer mu.Unlock()
	for _, start := range starts {
		if start == 0 {
			t.Fatalf("piece 0 was restarted from byte 0; starts=%v", starts)
		}
	}
	found := false
	for _, start := range starts {
		if start == resumeAt {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected resumed range to start at bitfield boundary %d; starts=%v", resumeAt, starts)
	}
}

func TestSOCKS5Proxy(t *testing.T) {
	data := testData(512 << 10)
	srv := rangeServer(data, true, nil)
	defer srv.Close()
	proxyAddr, closeProxy := startSocks5Proxy(t)
	defer closeProxy()

	dir := t.TempDir()
	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/socks.bin"}, Dir: dir}, Options{
		Proxy: "socks5://" + proxyAddr, Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("socks download mismatch")
	}
}

func testData(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte((i*31 + 7) % 251)
	}
	return b
}

func rangeServer(data []byte, ranges bool, rangeCount *atomic.Int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if ranges {
			w.Header().Set("Accept-Ranges", "bytes")
		}
		if r.Method == http.MethodHead {
			return
		}
		if ranges && r.Header.Get("Range") != "" {
			if rangeCount != nil {
				rangeCount.Add(1)
			}
			start, end, ok := parseTestRange(r.Header.Get("Range"), int64(len(data)))
			if !ok {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[start : end+1])
			return
		}
		_, _ = w.Write(data)
	}))
}

func parseTestRange(h string, size int64) (int64, int64, bool) {
	if !strings.HasPrefix(h, "bytes=") {
		return 0, 0, false
	}
	parts := strings.SplitN(strings.TrimPrefix(h, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, false
		}
	}
	if start < 0 || end < start || end >= size {
		return 0, 0, false
	}
	return start, end, true
}

func startSocks5Proxy(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					return
				}
			}
			wg.Add(1)
			go func() { defer wg.Done(); handleSocksConn(c) }()
		}
	}()
	closeFn := func() { close(done); _ = ln.Close() }
	return ln.Addr().String(), closeFn
}

func handleSocksConn(c net.Conn) {
	defer c.Close()
	var h [2]byte
	if _, err := io.ReadFull(c, h[:]); err != nil {
		return
	}
	methods := make([]byte, h[1])
	if _, err := io.ReadFull(c, methods); err != nil {
		return
	}
	_, _ = c.Write([]byte{0x05, 0x00})
	var req [4]byte
	if _, err := io.ReadFull(c, req[:]); err != nil {
		return
	}
	if req[1] != 0x01 {
		return
	}
	host := ""
	switch req[3] {
	case 0x01:
		b := make([]byte, 4)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	case 0x03:
		var n [1]byte
		if _, err := io.ReadFull(c, n[:]); err != nil {
			return
		}
		b := make([]byte, n[0])
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = string(b)
	case 0x04:
		b := make([]byte, 16)
		if _, err := io.ReadFull(c, b); err != nil {
			return
		}
		host = net.IP(b).String()
	default:
		return
	}
	var pb [2]byte
	if _, err := io.ReadFull(c, pb[:]); err != nil {
		return
	}
	port := int(pb[0])<<8 | int(pb[1])
	upstream, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		_, _ = c.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()
	_, _ = c.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	errc := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, c); errc <- struct{}{} }()
	go func() { _, _ = io.Copy(c, upstream); errc <- struct{}{} }()
	<-errc
}

func TestSegmentedCancelPersistsResumeState(t *testing.T) {
	data := testData(8 << 20)
	var mu sync.Mutex
	var starts []int64
	srv := slowRangeServer(data, true, "", &starts, &mu)
	defer srv.Close()

	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Download(ctx, Request{URLs: []string{srv.URL + "/big.bin"}, Dir: dir, Out: "big.bin"}, Options{
			Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 1, RetryWait: time.Millisecond, ResumeBlockSize: 256 << 10,
		})
		done <- err
	}()
	time.Sleep(60 * time.Millisecond)
	cancel()
	if err := <-done; err == nil {
		t.Fatal("expected cancellation error")
	}

	final := filepath.Join(dir, "big.bin")
	meta, err := loadMeta(sidecarPath(final))
	if err != nil {
		t.Fatalf("expected durable sidecar after cancel: %v", err)
	}
	if got := meta.completedBytes(); got == 0 {
		t.Fatal("expected sidecar to contain partial downloaded bytes after cancel")
	}

	mu.Lock()
	starts = nil
	mu.Unlock()
	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/big.bin"}, Dir: dir, Out: "big.bin"}, Options{
		Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 1, RetryWait: time.Millisecond, ResumeBlockSize: 256 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed {
		t.Fatal("expected resumed result after cancellation")
	}
	got, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("resumed file mismatch")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, start := range starts {
		if start == 0 {
			t.Fatalf("resume restarted piece 0 from byte 0; starts=%v", starts)
		}
	}
}

func TestProbeDetectsRangeWhenHeadDoesNotAdvertiseIt(t *testing.T) {
	data := testData(2 << 20)
	var ranges atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		// Intentionally no Accept-Ranges on HEAD. Some CDNs behave this way.
		if r.Method == http.MethodHead {
			return
		}
		if r.Header.Get("Range") != "" {
			ranges.Add(1)
			start, end, ok := parseTestRange(r.Header.Get("Range"), int64(len(data)))
			if !ok {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[start : end+1])
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/range.bin"}, Dir: t.TempDir()}, Options{
		Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Connections != 4 {
		t.Fatalf("expected segmented download after ranged GET probe, connections=%d", res.Connections)
	}
	if ranges.Load() < 3 { // one probe + bitfield block range requests
		t.Fatalf("expected ranged requests, got %d", ranges.Load())
	}
}

func TestResumeToleratesUnstableValidatorsByDefault(t *testing.T) {
	data := testData(2 << 20)
	var mu sync.Mutex
	var starts []int64
	var etag atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", fmt.Sprintf("etag-%d", etag.Add(1)))
		if r.Method == http.MethodHead {
			return
		}
		if r.Header.Get("Range") != "" {
			start, end, ok := parseTestRange(r.Header.Get("Range"), int64(len(data)))
			if !ok {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			mu.Lock()
			starts = append(starts, start)
			mu.Unlock()
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[start : end+1])
			return
		}
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	dir := t.TempDir()
	final := filepath.Join(dir, "unstable.bin")
	info := probeInfo{URL: srv.URL + "/unstable.bin", Size: int64(len(data)), AcceptRanges: true, ETag: "old-etag"}
	meta := newMeta([]string{srv.URL + "/unstable.bin"}, final, info, 512<<10)
	first := meta.pendingPieces(4, 1)[0]
	f, err := os.OpenFile(final, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt(data[first.Start:first.End+1], first.Start); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := meta.markDone(first.Index, sidecarPath(final)); err != nil {
		t.Fatal(err)
	}

	res, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/unstable.bin"}, Dir: dir, Out: "unstable.bin"}, Options{
		Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 1, ResumeBlockSize: 512 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Resumed {
		t.Fatal("expected resume despite unstable ETag by default")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, start := range starts {
		if start == 0 {
			t.Fatalf("unstable validator caused restart from zero; starts=%v", starts)
		}
	}
}

func slowRangeServer(data []byte, advertiseRanges bool, etag string, starts *[]int64, mu *sync.Mutex) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if advertiseRanges {
			w.Header().Set("Accept-Ranges", "bytes")
		}
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		if r.Method == http.MethodHead {
			return
		}
		start, end := int64(0), int64(len(data)-1)
		if r.Header.Get("Range") != "" {
			var ok bool
			start, end, ok = parseTestRange(r.Header.Get("Range"), int64(len(data)))
			if !ok {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if starts != nil && mu != nil {
				mu.Lock()
				*starts = append(*starts, start)
				mu.Unlock()
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
			w.WriteHeader(http.StatusPartialContent)
		}
		fl, _ := w.(http.Flusher)
		for off := start; off <= end; {
			n := int64(64 << 10)
			if off+n-1 > end {
				n = end - off + 1
			}
			_, _ = w.Write(data[off : off+n])
			if fl != nil {
				fl.Flush()
			}
			off += n
			time.Sleep(2 * time.Millisecond)
		}
	}))
}

func TestRangePlanningUsesSplitNotResumeBlockSize(t *testing.T) {
	const size = int64(100 << 20)
	const block = int64(1 << 20)
	dir := t.TempDir()
	final := filepath.Join(dir, "plan.bin")
	info := probeInfo{URL: "http://example.test/plan.bin", Size: size, AcceptRanges: true}
	meta := newMeta([]string{info.URL}, final, info, block)

	pieces := meta.pendingPieces(4, 1<<20)
	if len(pieces) != 4 {
		t.Fatalf("expected 4 HTTP range jobs from split=4, got %d", len(pieces))
	}
	for i, p := range pieces {
		if got := p.size(); got != 25<<20 {
			t.Fatalf("piece %d size=%d, want 25MiB; resume block must not become range size", i, got)
		}
	}
}

func TestBitfieldResumeSkipsBlocksButKeepsLargeRangeJobs(t *testing.T) {
	const size = int64(64 << 20)
	const block = int64(1 << 20)
	dir := t.TempDir()
	final := filepath.Join(dir, "resume-plan.bin")
	info := probeInfo{URL: "http://example.test/resume-plan.bin", Size: size, AcceptRanges: true}
	meta := newMeta([]string{info.URL}, final, info, block)
	if _, err := meta.markRangeComplete(0, 16<<20-1, sidecarPath(final)); err != nil {
		t.Fatal(err)
	}

	pieces := meta.pendingPieces(4, 1<<20)
	if len(pieces) != 4 {
		t.Fatalf("expected remaining 48MiB to be split into 4 range jobs, got %d", len(pieces))
	}
	if pieces[0].Start != 16<<20 {
		t.Fatalf("first range starts at %d, want 16MiB", pieces[0].Start)
	}
	if pieces[0].size() != 12<<20 {
		t.Fatalf("first range size=%d, want 12MiB", pieces[0].size())
	}
}

func TestProgressSpeedExcludesResumedBytes(t *testing.T) {
	tracker := newProgressTracker("job", 200<<20, 4, 200, "file.bin", time.Hour, nil)
	tracker.setCompleted(150 << 20)

	p := tracker.emit(StateDownloading, "", true)
	if p.Completed != 150<<20 {
		t.Fatalf("completed=%d, want resumed bytes", p.Completed)
	}
	if p.Speed != 0 {
		t.Fatalf("initial resumed speed=%f, want 0", p.Speed)
	}
	if p.AvgSpeed != 0 {
		t.Fatalf("initial resumed avg speed=%f, want 0", p.AvgSpeed)
	}

	time.Sleep(time.Millisecond)
	tracker.add(1 << 20)
	p = tracker.emit(StateDownloading, "", true)
	if p.AvgSpeed <= 0 {
		t.Fatalf("avg speed after new bytes=%f, want >0", p.AvgSpeed)
	}
	// The old bug counted all 151MiB as downloaded during this process. The
	// fixed tracker only counts the new 1MiB session delta.
	if p.AvgSpeed > float64(64<<20)/time.Millisecond.Seconds() {
		t.Fatalf("avg speed looks like it counted resumed bytes: %f", p.AvgSpeed)
	}
}

func TestBitfieldProgressUpdatesDuringRangeWrite(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "blocks.bin")
	sidecar := sidecarPath(final)
	info := probeInfo{URL: "http://example.test/blocks.bin", Size: 3 << 20, AcceptRanges: true}
	meta := newMeta([]string{info.URL}, final, info, 1<<20)
	if err := meta.save(sidecar); err != nil {
		t.Fatal(err)
	}

	var samples []Progress
	tracker := newProgressTracker("job", info.Size, 1, int64(meta.piecesTotal()), final, time.Hour, func(p Progress) {
		samples = append(samples, p)
	})
	f, err := os.OpenFile(final, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	data := bytes.NewReader(testData(int(info.Size)))
	_, err = copyRangeToFile(f, data, 0, info.Size, make([]byte, 256<<10), tracker, func(written int64) error {
		changed, err := meta.markRangeComplete(0, written-1, sidecar)
		if changed > 0 {
			tracker.addPiecesDone(int64(changed))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := meta.donePieces(); got != 3 {
		t.Fatalf("donePieces=%d, want 3", got)
	}
	seenOne, seenTwo, seenThree := false, false, false
	for _, s := range samples {
		switch s.PiecesDone {
		case 1:
			seenOne = true
		case 2:
			seenTwo = true
		case 3:
			seenThree = true
		}
	}
	if !seenOne || !seenTwo || !seenThree {
		t.Fatalf("progress did not report block-level piece updates; samples=%v", samples)
	}
}
