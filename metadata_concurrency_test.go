package hydra

import (
	"bytes"
	"context"
	"errors"
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

func awaitMetadataTest[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for metadata operation")
		var zero T
		return zero
	}
}

func TestConcurrentBlockCompletionCountedOnce(t *testing.T) {
	const blockSize = 32 << 10
	m := newMeta(nil, "file", probeInfo{Size: blockSize}, blockSize)
	var wg sync.WaitGroup
	var completed atomic.Int64
	for i := 0; i < 48; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.markBlockComplete(0) {
				completed.Add(1)
			}
		}()
	}
	wg.Wait()
	if completed.Load() != 1 || m.donePieces() != 1 || !m.dirty {
		t.Fatalf("duplicate completion: count=%d bits=%d dirty=%v", completed.Load(), m.donePieces(), m.dirty)
	}
}

type forbiddenCheckpointReader struct {
	reads atomic.Int32
}

func (r *forbiddenCheckpointReader) ReadAt(_ []byte, _ int64) (int, error) {
	r.reads.Add(1)
	return 0, errors.New("bitmap-only metadata must not read or hash output")
}

func TestBitmapCheckpointDoesNotReadFile(t *testing.T) {
	const blockSize = 32 << 10
	m := newMeta(nil, "file", probeInfo{Size: 2 * blockSize}, blockSize)
	reader := &forbiddenCheckpointReader{}
	n, err := m.markRangeCompleteFromFile(reader, 0, 2*blockSize-1, "")
	if err != nil || n != 2 || reader.reads.Load() != 0 {
		t.Fatalf("unexpected hashing: completions=%d reads=%d err=%v", n, reader.reads.Load(), err)
	}
	if m.donePieces() != 2 || !m.dirty {
		t.Fatalf("bitmap not updated: bits=%d dirty=%v", m.donePieces(), m.dirty)
	}
}

func TestBitmapMetadataIsSmallAndContainsNoHashes(t *testing.T) {
	const blockSize = 32 << 10
	path := filepath.Join(t.TempDir(), "file")
	m := newMeta(nil, path, probeInfo{Size: 10 * blockSize, ETag: `"stable"`}, blockSize)
	for i := 0; i < 10; i++ {
		m.markBlockComplete(i)
	}
	if err := m.save(sidecarPath(path)); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("block_hashes")) || bytes.Contains(b, []byte("prefix_sha256")) {
		t.Fatalf("bitmap-only checkpoint contains hashes: %s", b)
	}
	loaded, err := loadMeta(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != metaVersionBitfield || loaded.donePieces() != 10 || loaded.BlockHashes != "" {
		t.Fatalf("invalid bitmap-only sidecar: version=%d blocks=%d", loaded.Version, loaded.donePieces())
	}
}

func TestBitmapLegacyV3CheckpointRestartsInsteadOfTrustingOldBits(t *testing.T) {
	const blockSize = 32 << 10
	path := filepath.Join(t.TempDir(), "file")
	m := newMeta(nil, path, probeInfo{Size: 3 * blockSize, ETag: `"stable"`}, blockSize)
	m.Version = 3
	m.markBlockComplete(0)
	m.markBlockComplete(1)
	// A real previous-version sidecar also stored hashes. The v4 decoder
	// discards those and restarts rather than accepting unverified bytes.
	m.BlockHashes = "legacy-hashes"
	if err := m.save(sidecarPath(path)); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadMeta(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != metaVersionBitfield || loaded.donePieces() != 0 ||
		loaded.BlockHashes != "" || loaded.PrefixSize != 0 {
		t.Fatalf("legacy sidecar reused unexpectedly: version=%d complete=%d", loaded.Version, loaded.donePieces())
	}
}

func TestSegmentedCheckpointIncludesShortFinalBlock(t *testing.T) {
	data := make([]byte, 2*DefaultResumeBlockSize+123)
	for i := range data {
		data[i] = byte(i*31 + i/257)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"stable"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	result, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{
		Split: 8, MinSplitSize: 1, BufferSize: 32 << 10, KeepPartFile: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Connections <= 1 {
		t.Fatal("expected segmented download")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("download content mismatch")
	}
	m, err := loadMeta(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if m.donePieces() != 3 || m.completedBytes() != int64(len(data)) || m.BlockHashes != "" {
		t.Fatalf("checkpoint omitted completed data or added hashes: blocks=%d bytes=%d", m.donePieces(), m.completedBytes())
	}
}

// The explicit bitmap-only tradeoff is that a previously completed block
// is not rehashed. File checksums, if supplied, are verified separately.
func TestBitmapResumeDoesNotVerifyCorruptedCompletedData(t *testing.T) {
	const blockSize = 32 << 10
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 2*blockSize), 0600); err != nil {
		t.Fatal(err)
	}
	m := newMeta(nil, path, probeInfo{Size: 2 * blockSize, ETag: `"stable"`}, blockSize)
	m.markBlockComplete(0)
	if err := m.save(sidecarPath(path)); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteAt([]byte("damage"), 0); err != nil {
		t.Fatal(err)
	}
	if m.donePieces() != 1 {
		t.Fatal("unexpected completion state")
	}
}

// Compile-time check that a nil ReaderAt is allowed on the bitmap-only path.
var _ io.ReaderAt = (*forbiddenCheckpointReader)(nil)
