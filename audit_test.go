package hydra

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestResumeDoesNotTrustMissingOrTruncatedOutput(t *testing.T) {
	data := integrityData(7)
	for _, length := range []int{-1, 1} {
		t.Run(fmt.Sprint(length), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"test-object"`)
				http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
			}))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "file.bin")
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			meta := newMeta([]string{srv.URL}, path, probeInfo{Size: int64(len(data)), ETag: `"test-object"`}, 32<<10)
			_, markErr := meta.markRangeCompleteFromFile(f, 0, int64(len(data))-1, "")
			_ = f.Close()
			if markErr != nil {
				t.Fatal(markErr)
			}
			if err := meta.save(sidecarPath(path)); err != nil {
				t.Fatal(err)
			}
			if length >= 0 {
				if err := os.WriteFile(path, data[:length], 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			_, err = Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 2, MinSplitSize: 1, ResumeBlockSize: 32 << 10})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, data) {
				t.Fatal("trusted completed metadata without corresponding output bytes")
			}
		})
	}
}

func TestExpectedSizeEnforcedForUnknownLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			return
		}
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("short"))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path, ExpectedSize: 12}, Options{Split: 1})
	if err == nil {
		t.Fatal("accepted incorrect length from chunked response")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("published invalid output")
	}
}

func TestManagerWaitIsRepeatable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	m, err := NewManager(context.Background(), Options{Split: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	id, err := m.Enqueue(Request{ID: "repeat", URLs: []string{srv.URL}, Out: filepath.Join(t.TempDir(), "file")})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		r, err := m.Wait(context.Background(), id)
		if err != nil || r.ID != string(id) {
			t.Fatalf("wait %d: result=%+v err=%v", i, r, err)
		}
	}
}

func TestManagerQueuedEventPrecedesStarted(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	var mu sync.Mutex
	var kinds []EventKind
	m, err := NewManager(context.Background(), Options{Split: 1, OnEvent: func(e Event) { mu.Lock(); kinds = append(kinds, e.Kind); mu.Unlock() }})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	id, err := m.Enqueue(Request{URLs: []string{srv.URL}, Out: filepath.Join(t.TempDir(), "file")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Wait(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(kinds) == 0 || kinds[0] != EventQueued {
		t.Fatalf("queue lifecycle starts out of order: %v", kinds)
	}
}

func TestGeneratedJobIDsAreUniqueConcurrently(t *testing.T) {
	const workers, perWorker = 32, 2048
	ids := make([][]string, workers)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids[worker] = make([]string, perWorker)
			for i := range perWorker {
				ids[worker][i] = newJobID()
			}
		}()
	}
	wg.Wait()
	seen := make(map[string]bool, workers*perWorker)
	for _, group := range ids {
		for _, id := range group {
			if seen[id] {
				t.Fatalf("duplicate generated job id: %s", id)
			}
			seen[id] = true
		}
	}
}

func TestManagerConcurrentCloseAndEnqueue(t *testing.T) {
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		m, err := NewManager(ctx, Options{})
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		var wg sync.WaitGroup
		wg.Add(3)
		go func() { defer wg.Done(); _ = m.Close() }()
		go func() { defer wg.Done(); _ = m.Close() }()
		go func() { defer wg.Done(); _, _ = m.Enqueue(Request{URLs: []string{"http://127.0.0.1:1"}}) }()
		wg.Wait()
	}
}

func TestHTTPSHTTP2SegmentedDownload(t *testing.T) {
	data := testData(512 << 10)
	var sawHTTP2 atomic.Bool
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		if r.ProtoMajor == 2 {
			sawHTTP2.Store(true)
		}
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: filepath.Join(t.TempDir(), "file.bin")}, Options{
		Split: 4, MaxConnectionsPerServer: 4, MinSplitSize: 32768, ResumeBlockSize: 32768, Transport: srv.Client().Transport,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(r.Path)
	if err != nil || !bytes.Equal(got, data) || !sawHTTP2.Load() || r.Connections != 4 {
		t.Fatalf("HTTPS/HTTP2 download failed: http2=%v err=%v", sawHTTP2.Load(), err)
	}
}

func TestMirrorFailoverAfterPermanentHTTPError(t *testing.T) {
	data := testData(128 << 10)
	bad := httptest.NewServer(http.NotFoundHandler())
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer good.Close()
	for _, split := range []int{1, 2} {
		t.Run(fmt.Sprint(split), func(t *testing.T) {
			r, err := Download(context.Background(), Request{URLs: []string{bad.URL, good.URL}, Out: filepath.Join(t.TempDir(), "file")}, Options{
				Split: split, MinSplitSize: 32768, ResumeBlockSize: 32768,
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(r.Path)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("mirror failover produced incorrect output: %v", err)
			}
		})
	}
}

func TestInvalidRangeDoesNotRetrySameBrokenMirror(t *testing.T) {
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "16")
		if r.Method == http.MethodHead {
			return
		}
		gets.Add(1)
		w.Header().Set("Content-Range", "bytes 8-15/16")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("0123456789abcdef"))
	}))
	defer srv.Close()
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 2, MinSplitSize: 1, RetryWait: time.Millisecond, MaxRetryWait: time.Millisecond})
	if !errors.Is(err, ErrInvalidContentRange) {
		t.Fatalf("malformed range error=%v", err)
	}
	if n := gets.Load(); n != 1 {
		t.Fatalf("retried broken protocol response %d times", n)
	}
}

func TestMalformedRangeFailsOverToHealthyMirror(t *testing.T) {
	data := testData(128 << 10)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Header().Set("Accept-Ranges", "bytes")
		if r.Method == http.MethodHead {
			return
		}
		w.Header().Set("Content-Range", "bytes 0-0/1")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(data)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer good.Close()
	for _, split := range []int{1, 2} {
		t.Run(fmt.Sprint(split), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			if split == 1 {
				if err := os.WriteFile(partPath(path), data[:32768], 0644); err != nil {
					t.Fatal(err)
				}
			}
			r, err := Download(context.Background(), Request{URLs: []string{bad.URL, good.URL}, Out: path}, Options{Split: split, MinSplitSize: 32768, ResumeBlockSize: 32768})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(r.Path)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("malformed-mirror failover corrupted output: %v", err)
			}
		})
	}
}

func TestLocalWriteErrorsAreNotRetried(t *testing.T) {
	for _, err := range []error{&os.PathError{Op: "write", Path: "file", Err: os.ErrPermission}, io.ErrShortWrite} {
		if retryable(err) {
			t.Errorf("retryable local write error: %v", err)
		}
	}
}

func TestSegmentedDownloadFallsBackWhenServerIgnoresRange(t *testing.T) {
	data := testData(128 << 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	}))
	defer srv.Close()
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 4, MinSplitSize: 32768, ResumeBlockSize: 32768})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(r.Path)
	if err != nil || !bytes.Equal(got, data) || r.Connections != 1 {
		t.Fatalf("single-stream fallback failed: connections=%d err=%v", r.Connections, err)
	}
}

func TestSingleStreamRemovesStaleSegmentedMetadata(t *testing.T) {
	data := []byte("0123456789abcdef")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file.bin")
	meta := newMeta(nil, path, probeInfo{Size: int64(len(data))}, 32768)
	if err := meta.save(sidecarPath(path)); err != nil {
		t.Fatal(err)
	}
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecarPath(path)); !os.IsNotExist(err) {
		t.Fatal("stale segmented sidecar retained after single-stream completion")
	}
}

func TestMetadataFlushSyncsDataBeforePublishingBits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.hydra")
	m := newMeta(nil, "file", probeInfo{Size: 65536}, 32768)
	if err := m.save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := m.markRangeComplete(0, 32767, ""); err != nil {
		t.Fatal(err)
	}
	called := false
	err := m.saveIfDueAfterSync(path, 0, func() error {
		called = true
		loaded, err := loadMeta(path)
		if err != nil {
			return err
		}
		if loaded.completedBytes() != 0 {
			t.Error("bits published before data sync")
		}
		return fmt.Errorf("simulated sync failure")
	})
	if !called || err == nil {
		t.Fatal("data sync failure not propagated")
	}
	loaded, err := loadMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.completedBytes() != 0 {
		t.Fatal("saved unsynced bits")
	}
	if err := m.saveIfDueAfterSync(path, 0, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadMeta(path)
	if err != nil || loaded.completedBytes() != 32768 {
		t.Fatalf("checkpoint not saved: %v", err)
	}
}

func TestMetadataFlusherFlushesWithoutAnotherWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.hydra")
	m := newMeta(nil, "file", probeInfo{Size: 65536}, 32768)
	if err := m.save(path); err != nil {
		t.Fatal(err)
	}
	f := startMetadataFlusher(m, path, 10*time.Millisecond)
	defer f.close()
	if _, err := m.markRangeComplete(0, 32767, ""); err != nil {
		t.Fatal(err)
	}
	f.request()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("dirty checkpoint never flushed while worker idle")
		case <-ticker.C:
			loaded, err := loadMeta(path)
			if err == nil && loaded.completedBytes() == 32768 {
				return
			}
		}
	}
}

func TestPreallocateShrinksExistingFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "file"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(65536); err != nil {
		t.Fatal(err)
	}
	if err := preallocateFile(f, 32768); err != nil {
		t.Fatal(err)
	}
	st, err := f.Stat()
	if err != nil || st.Size() != 32768 {
		t.Fatalf("incorrect output size: %v %v", st, err)
	}
}

func TestSingleStreamDiscoversUnadvertisedRangeSupport(t *testing.T) {
	data := []byte("0123456789abcdef")
	var resumed bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprint(len(data)))
			return
		}
		if r.Header.Get("Range") == "bytes=8-" {
			resumed = true
		}
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(partPath(path), data[:8], 0644); err != nil {
		t.Fatal(err)
	}
	seedSingleCheckpoint(t, path, srv.URL, len(data))
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 1})
	if err != nil || !r.Resumed || !resumed {
		t.Fatalf("resume=%v request=%v err=%v", r.Resumed, resumed, err)
	}
}

func TestRejectMalformedFallbackProbe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Range", "bytes 1-1/100")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{})
	if err == nil {
		t.Fatal("accepted wrong probe range")
	}
}

func TestSmallFreshDownloadDoesNotNeedRangeProbe(t *testing.T) {
	var requests, ranges atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Length", "2")
		if r.Header.Get("Range") != "" {
			ranges.Add(1)
		}
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	for _, opts := range []Options{{}, {Split: 8, MaxConnectionsPerServer: 1, MinSplitSize: 1}} {
		requests.Store(0)
		ranges.Store(0)
		_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, opts)
		if err != nil {
			t.Fatal(err)
		}
		if requests.Load() != 2 || ranges.Load() != 0 {
			t.Fatalf("unnecessary range probe: requests=%d ranged=%d", requests.Load(), ranges.Load())
		}
	}
}

func TestHEADNotImplementedFallsBackToGET(t *testing.T) {
	data := []byte("HEAD unsupported")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(r.Path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("HEAD fallback failed: %v", err)
	}
}

func TestSafeRemoteFilenames(t *testing.T) {
	for _, name := range []string{"..", ".", "/", `C:\tmp\..`} {
		if got := filenameFromResponse("https://example.test/file", http.Header{"Content-Disposition": {fmt.Sprintf("attachment; filename=%q", name)}}); got != "download.bin" {
			t.Errorf("%q: got %q", name, got)
		}
	}
}

func TestInvalidRequestPolicy(t *testing.T) {
	_, err := (Request{URLs: []string{"http://example.test"}, ExistingFile: "typo"}).normalized()
	if err == nil {
		t.Fatal("invalid policy accepted")
	}
}

func TestInvalidChecksumRejectedBeforeNetwork(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); _, _ = w.Write([]byte("x")) }))
	defer srv.Close()
	for _, value := range []string{"00", "zz", stringsRepeat("z", 64), ""} {
		_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir(), Checksum: Checksum{Algorithm: "sha256", Value: value}}, Options{})
		if err == nil {
			t.Errorf("accepted malformed checksum %q", value)
		}
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("malformed checksums caused %d network requests", got)
	}
	if _, err := ParseChecksum("sha256:00"); err == nil {
		t.Fatal("accepted short SHA-256 digest")
	}
}

func TestChecksumValidationAcceptsSupportedDigests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checksum")
	data := []byte("hydra checksum validation")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	for _, algorithm := range []string{"sha256", "sha-256", "sha512", "sha-512", "sha1", "sha-1", "md5"} {
		h, err := (Checksum{Algorithm: algorithm}).newHash()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = h.Write(data)
		digest := fmt.Sprintf("%x", h.Sum(nil))
		c, err := ParseChecksum(algorithm + ":" + digest)
		if err != nil {
			t.Fatalf("%s: %v", algorithm, err)
		}
		if _, err := c.VerifyFile(path); err != nil {
			t.Fatalf("%s: %v", algorithm, err)
		}
	}
}

func TestDirectorySyncSupportsMetadataCheckpoints(t *testing.T) {
	dir := t.TempDir()
	if err := syncDir(dir); err != nil {
		t.Fatal(err)
	}
	m := newMeta(nil, "file", probeInfo{Size: 65536}, 32768)
	path := filepath.Join(dir, "file.hydra")
	for i := 0; i < 2; i++ {
		if err := m.save(path); err != nil {
			t.Fatalf("checkpoint %d: %v", i, err)
		}
	}
}

func TestSuccessfulSegmentedTransferDoesNotRewriteDiscardedCheckpoint(t *testing.T) {
	data := testData(128 << 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	var once sync.Once
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{
		Split: 4, MinSplitSize: 32768, ResumeBlockSize: 32768, MetadataFlushInterval: time.Hour,
		OnProgress: func(p Progress) {
			if p.State == StateDownloading && p.Completed == p.Total && p.PiecesDone > 0 {
				once.Do(func() {
					if err := os.Mkdir(sidecarPath(path)+".tmp", 0755); err != nil {
						t.Error(err)
					}
				})
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecarPath(path)); !os.IsNotExist(err) {
		t.Fatalf("completed sidecar retained: %v", err)
	}
	if st, err := os.Stat(sidecarPath(path) + ".tmp"); err != nil || !st.IsDir() {
		t.Fatalf("checkpoint rewrite guard not reached: %v", err)
	}
}

func TestKeptSegmentedCheckpointIsComplete(t *testing.T) {
	data := testData(128 << 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 4, MinSplitSize: 32768, ResumeBlockSize: 32768, KeepPartFile: true, MetadataFlushInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	m, err := loadMeta(sidecarPath(r.Path))
	if err != nil || m.completedBytes() != int64(len(data)) {
		t.Fatalf("incomplete kept checkpoint: %v", err)
	}
}

func TestConcurrentProgressBlockCountsStayBounded(t *testing.T) {
	data := testData(4 << 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{
		Split: 16, MaxConnectionsPerServer: 16, MinSplitSize: 32768, ResumeBlockSize: 32768,
		OnProgress: func(p Progress) {
			if p.PiecesDone > p.PiecesTotal {
				t.Errorf("completed blocks exceed total: %+v", p)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestOwnedTransportConnectionsCloseAfterUse(t *testing.T) {
	for _, manager := range []bool{false, true} {
		t.Run(fmt.Sprint(manager), func(t *testing.T) {
			closed := make(chan struct{})
			var once sync.Once
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader([]byte("ok")))
			}))
			srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					once.Do(func() { close(closed) })
				}
			}
			srv.Start()
			defer srv.Close()
			req := Request{URLs: []string{srv.URL}, Out: filepath.Join(t.TempDir(), "file")}
			if manager {
				m, err := NewManager(context.Background(), Options{Split: 1})
				if err != nil {
					t.Fatal(err)
				}
				defer m.Close()
				id, err := m.Enqueue(req)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := m.Wait(context.Background(), id); err != nil {
					t.Fatal(err)
				}
				if err := m.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := Download(context.Background(), req, Options{Split: 1}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("owned idle connection was not closed")
			}
		})
	}
}

type closeTrackingTransport struct{ closes atomic.Int64 }

func (tr *closeTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := io.NopCloser(bytes.NewReader([]byte("ok")))
	if req.Method == http.MethodHead {
		body = http.NoBody
	}
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header), Body: body, ContentLength: 2, Request: req}, nil
}

func (tr *closeTrackingTransport) CloseIdleConnections() { tr.closes.Add(1) }

func TestSharedCustomTransportIsNotAutomaticallyClosed(t *testing.T) {
	tr := &closeTrackingTransport{}
	opts := Options{Split: 1, Transport: tr}
	req := Request{URLs: []string{"http://example.test/file"}, Out: filepath.Join(t.TempDir(), "file")}
	if _, err := Download(context.Background(), req, opts); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if tr.closes.Load() != 0 {
		t.Fatal("automatically closed caller-owned transport")
	}
	d, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	d.CloseIdleConnections()
	if tr.closes.Load() != 1 {
		t.Fatal("explicit connection cleanup did not reach custom transport")
	}
}

func TestRetryDelayNeverOverflowsOrExceedsCap(t *testing.T) {
	for _, wait := range []time.Duration{time.Second, time.Duration(math.MaxInt64/2 + 1)} {
		d := &Downloader{opts: Options{RetryWait: wait, MaxRetryWait: 4 * time.Second}}
		for attempt := 0; attempt < 100; attempt++ {
			if got := d.retryDelay(attempt); got < 0 || got > 4*time.Second {
				t.Fatalf("wait=%v attempt=%d delay=%v", wait, attempt, got)
			}
		}
	}
	if got := parseRetryAfter("9223372036854775807", time.Now()); got != time.Duration(math.MaxInt64) {
		t.Fatalf("Retry-After overflow: %v", got)
	}
}

func TestDisableRetriesAndMaximumRetryCount(t *testing.T) {
	d, err := New(Options{DisableRetries: true})
	if err != nil || d.opts.MaxRetries != 0 {
		t.Fatalf("disable retries: %v %v", d, err)
	}
	if _, err := New(Options{MaxRetries: int(^uint(0) >> 1)}); err == nil {
		t.Fatal("retry-count overflow accepted")
	}
	d = &Downloader{opts: Options{RetryWait: time.Duration(math.MaxInt64/2 + 1)}}
	for attempt := 0; attempt < 100; attempt++ {
		if got := d.retryDelay(attempt); got < 0 {
			t.Fatalf("uncapped backoff overflow: %v", got)
		}
	}
}

func TestExtremeSizeArithmetic(t *testing.T) {
	if strconv.IntSize == 64 {
		if got := int64(blockCount(math.MaxInt64, 1<<20)); got != 1<<43 {
			t.Fatalf("block count overflow: %d", got)
		}
	}
	m := newMeta(nil, "file", probeInfo{Size: math.MaxInt64}, math.MaxInt64-1)
	if got := m.blockSize(1); got != 1 {
		t.Fatalf("last block size=%d, want 1", got)
	}
	pieces := m.pendingPieces(2, math.MaxInt64)
	if len(pieces) != 1 || pieces[0].Start != 0 || pieces[0].End != math.MaxInt64-1 {
		t.Fatalf("overflowed range plan: %+v", pieces)
	}
}

func TestLargeSparseRangeWrite(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sparse-file allocation behavior is only verified on Linux in this audit")
	}
	f, err := os.Create(filepath.Join(t.TempDir(), "large-sparse.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	const offset = int64(1<<32) + 12345
	const size = offset + 65536
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	data := testData(32768)
	tracker := newProgressTracker("large", size, 1, 1, f.Name(), time.Hour, nil)
	written, err := copyRangeToFile(f, bytes.NewReader(data), offset, int64(len(data)), make([]byte, 4096), tracker, nil)
	if err != nil || written != int64(len(data)) {
		t.Fatalf("large-offset write=%d err=%v", written, err)
	}
	got := make([]byte, len(data))
	if _, err := f.ReadAt(got, offset); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("range above 4 GiB was misplaced or corrupted")
	}
	st, err := f.Stat()
	if err != nil || st.Size() != size {
		t.Fatalf("large output size changed: %v %v", st, err)
	}
}

func TestOversizedMetadataRejectedBeforeAllocation(t *testing.T) {
	if err := validateMetadataSize(math.MaxInt64, 32768); err == nil {
		t.Fatal("unbounded metadata allocation allowed")
	}
	path := filepath.Join(t.TempDir(), "bad.hydra")
	if err := os.WriteFile(path, []byte(`{"version":2,"size":9223372036854775807,"block_size":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadMeta(path); err == nil {
		t.Fatal("malformed sidecar allowed unbounded allocation")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(int64(math.MaxInt64)))
		w.Header().Set("Accept-Ranges", "bytes")
	}))
	defer srv.Close()
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 2})
	if err == nil {
		t.Fatal("absurd advertised size was accepted")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(2*maxResumeMetadataBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadMeta(path); err == nil {
		t.Fatal("oversized sidecar was read")
	}
}

func TestExtremeMinimumSplitSizeDoesNotOverflow(t *testing.T) {
	data := testData(65536)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 2, MinSplitSize: math.MaxInt64})
	if err != nil || r.Connections != 1 {
		t.Fatalf("connections=%d err=%v", r.Connections, err)
	}
}

func TestResultSpeedDoesNotCountResumedBytes(t *testing.T) {
	data := testData(128 << 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"test-object"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	for _, split := range []int{1, 2} {
		t.Run(fmt.Sprint(split), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			if split == 1 {
				if err := os.WriteFile(partPath(path), data[:32768], 0644); err != nil {
					t.Fatal(err)
				}
				seedSingleCheckpoint(t, path, srv.URL, len(data))
			} else {
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
				m := newMeta(nil, path, probeInfo{Size: int64(len(data)), ETag: `"test-object"`}, 32768)
				if err := m.markDone(0, sidecarPath(path)); err != nil {
					t.Fatal(err)
				}
				if err := m.save(sidecarPath(path)); err != nil {
					t.Fatal(err)
				}
			}
			var mu sync.Mutex
			var complete Progress
			r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{
				Split: split, MinSplitSize: 32768, ResumeBlockSize: 32768,
				OnProgress: func(p Progress) {
					mu.Lock()
					defer mu.Unlock()
					if p.State == StateComplete {
						complete = p
					}
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !r.Resumed || r.ResumedBytes != 32768 || r.Downloaded != int64(len(data))-32768 {
				t.Fatalf("incorrect result counters: %+v", r)
			}
			if want := float64(r.Downloaded) / r.Duration.Seconds(); r.AvgSpeed != want {
				t.Fatalf("average speed=%v want=%v", r.AvgSpeed, want)
			}
			mu.Lock()
			defer mu.Unlock()
			if complete.Resumed != r.ResumedBytes || complete.Downloaded != r.Downloaded || complete.AvgSpeed != r.AvgSpeed {
				t.Fatalf("incorrect completion progress: %+v", complete)
			}
		})
	}
}

func TestFreshSingleStreamRestartsAfterInterruptedNonRangeResponse(t *testing.T) {
	data := []byte("0123456789abcdef")
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method == http.MethodHead {
			return
		}
		if gets.Add(1) == 1 {
			_, _ = w.Write(data[:8])
			return
		}
		// Deliberately ignore Range, as a non-resumable origin would.
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 1, RetryWait: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(r.Path)
	if err != nil || !bytes.Equal(got, data) || r.Resumed || r.ResumedBytes != 0 {
		t.Fatalf("restart failed: %+v err=%v", r, err)
	}
}

func FuzzPendingPieces(f *testing.F) {
	f.Add(uint32(1048576), uint16(1), uint32(32768), uint8(4), uint64(0))
	f.Add(uint32(4194303), uint16(0), uint32(4294967295), uint8(255), uint64(0xaaaaaaaaaaaaaaaa))
	f.Fuzz(func(t *testing.T, rawSize uint32, rawBlock uint16, rawMin uint32, rawSplit uint8, done uint64) {
		size := int64(rawSize%(4<<20)) + 1
		block := int64(rawBlock%32+1) * 32768
		m := newMeta(nil, "file", probeInfo{Size: size}, block)
		for i := 0; i < m.BlockCount; i++ {
			if done&(uint64(1)<<uint(i%64)) != 0 {
				setBit(m.bits, i)
			}
		}
		pieces := m.pendingPieces(int(rawSplit)+1, int64(rawMin)+1)
		covered := make([]int, m.BlockCount)
		for _, p := range pieces {
			if p.Start < 0 || p.End < p.Start || p.End >= size || p.Start%block != 0 || (p.End+1 != size && (p.End+1)%block != 0) {
				t.Fatalf("invalid range %+v (size=%d block=%d)", p, size, block)
			}
			for i := int(p.Start / block); i <= int(p.End/block); i++ {
				covered[i]++
			}
		}
		for i, count := range covered {
			want := 1
			if testBit(m.bits, i) {
				want = 0
			}
			if count != want {
				t.Fatalf("block %d covered %d times, want %d", i, count, want)
			}
		}
	})
}

func TestKnownEmptyRemoteDoesNotSkipNonemptyOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "0")
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("not empty"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 1})
	if err == nil {
		t.Fatal("skipped a mismatched existing file")
	}
}

func TestKnownSkipAndErrorPoliciesDoNotContactOrigin(t *testing.T) {
	for _, policy := range []ExistingFilePolicy{ExistingFileSkip, ExistingFileError} {
		t.Run(string(policy), func(t *testing.T) {
			var requests atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.NotFound(w, r) }))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "existing.bin")
			if err := os.WriteFile(path, []byte("already downloaded"), 0644); err != nil {
				t.Fatal(err)
			}
			r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{ExistingFile: policy})
			if policy == ExistingFileSkip && (err != nil || !r.Skipped) {
				t.Fatalf("local skip failed: %+v err=%v", r, err)
			}
			if policy == ExistingFileError && err == nil {
				t.Fatal("existing-file error policy returned success")
			}
			if requests.Load() != 0 {
				t.Fatalf("local %s policy contacted the origin", policy)
			}
		})
	}
}

func TestExplicitSkipAllowsDifferentRemoteSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		if r.Method != http.MethodHead {
			_, _ = w.Write(make([]byte, 100))
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.bin"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL + "/file.bin"}, Dir: dir}, Options{Split: 1, ExistingFile: ExistingFileSkip})
	if err != nil || !r.Skipped || r.Size != 3 {
		t.Fatalf("explicit skip failed: %+v err=%v", r, err)
	}
}

func TestOverwriteNeverRemovesAnExistingDirectory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "keep-directory")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{ExistingFile: ExistingFileOverwrite})
	if err == nil {
		t.Fatal("overwrote a directory with a file")
	}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		t.Fatalf("directory was removed: %v %v", st, err)
	}
}

func TestCleanupNeverRemovesDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := removeIfExists(path); err == nil {
		t.Fatal("directory cleanup was allowed")
	}
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		t.Fatalf("directory removed: %v %v", st, err)
	}
}

func TestEarlySkipPreservesConstraintsAndLocks(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.NotFound(w, r) }))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, req := range []Request{
		{URLs: []string{srv.URL}, Out: path, ExpectedSize: 3},
		{URLs: []string{srv.URL}, Out: path, Checksum: Checksum{Algorithm: "sha256", Value: stringsRepeat("0", 64)}},
	} {
		if _, err := Download(context.Background(), req, Options{ExistingFile: ExistingFileSkip}); err == nil {
			t.Fatal("skip ignored local integrity constraint")
		}
	}
	lock, err := acquireFileLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	if _, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{ExistingFile: ExistingFileSkip}); err == nil {
		t.Fatal("skip ignored an active file lock")
	}
	if requests.Load() != 0 {
		t.Fatal("local skip validation contacted origin")
	}
}

func FuzzParseContentRange(f *testing.F) {
	for _, value := range []string{"bytes 0-0/1", "bytes 8-15/16", "bytes */0", "bytes -1-4/10", "", "bytes 0-9223372036854775807/9223372036854775807"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		cr, err := parseContentRange(value)
		if err == nil && (cr.Start < 0 || cr.End < cr.Start || cr.Total <= cr.End) {
			t.Fatalf("invalid accepted range: %+v", cr)
		}
	})
}

func TestEnvironmentProxyConfiguration(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		client, err := newHTTPClient(Options{NoProxyFromEnvironment: disabled})
		if err != nil {
			t.Fatal(err)
		}
		tr := client.Transport.(*http.Transport)
		if (tr.Proxy == nil) != disabled {
			t.Errorf("disabled=%v: Proxy configured incorrectly", disabled)
		}
	}
}

func BenchmarkMetadataCheckpoint(b *testing.B) {
	m := newMeta([]string{"http://example.test"}, "file", probeInfo{Size: 4 << 30}, 1<<20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.markRangeComplete(0, int64((i%4096)+1)*(1<<20)-1, ""); err != nil {
			b.Fatal(err)
		}
	}
}
