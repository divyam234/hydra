package hydra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Hold reads of the first block so tests can observe metadata while hashing
// is in flight, without relying on hashing speed or scheduler timing.
type gatedBlockReader struct {
	*bytes.Reader
	blockSize int64
	started   chan struct{}
	release   chan struct{}
}

func (r *gatedBlockReader) ReadAt(p []byte, off int64) (int, error) {
	if off < r.blockSize {
		r.started <- struct{}{}
		<-r.release
	}
	return r.Reader.ReadAt(p, off)
}

func awaitMetadataTest[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("metadata operation blocked while another block was hashing")
		var zero T
		return zero
	}
}

func TestBlockHashDoesNotHoldMetadataLock(t *testing.T) {
	const blockSize = 32 << 10
	data := append(bytes.Repeat([]byte("a"), blockSize), bytes.Repeat([]byte("b"), blockSize)...)
	m := newMeta(nil, filepath.Join(t.TempDir(), "file"), probeInfo{Size: int64(len(data))}, blockSize)
	r := &gatedBlockReader{bytes.NewReader(data), blockSize, make(chan struct{}, 1), make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(r.release) }) }
	defer unblock()
	type completion struct {
		n   int
		err error
	}
	first := make(chan completion, 1)
	go func() {
		n, err := m.markRangeCompleteFromFile(r, 0, blockSize-1, "")
		first <- completion{n, err}
	}()
	awaitMetadataTest(t, r.started)
	second := make(chan completion, 1)
	go func() {
		n, err := m.markRangeCompleteFromFile(r, blockSize, int64(len(data))-1, "")
		second <- completion{n, err}
	}()
	if got := awaitMetadataTest(t, second); got.n != 1 || got.err != nil {
		t.Fatalf("second block: %+v", got)
	}
	// A checkpoint may include the finished block, but not the in-flight hash.
	// Publication after this save must cause a fresh data sync and checkpoint.
	sidecar := sidecarPath(m.Path)
	syncs := 0
	save := func() error {
		return m.saveIfDueAfterSync(sidecar, 0, func() error { syncs++; return nil })
	}
	saved := make(chan error, 1)
	go func() { saved <- save() }()
	if err := awaitMetadataTest(t, saved); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := loadMeta(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if testBit(checkpoint.bits, 0) || !testBit(checkpoint.bits, 1) {
		t.Fatalf("checkpoint published in-flight block: %08b", checkpoint.bits)
	}
	want := sha256.Sum256(data[blockSize:])
	if !bytes.Equal(checkpoint.hashes[sha256.Size:], want[:]) {
		t.Fatal("checkpoint did not publish the completed block's hash")
	}
	unblock()
	if got := awaitMetadataTest(t, first); got.n != 1 || got.err != nil {
		t.Fatalf("first block: %+v", got)
	}
	if err := save(); err != nil {
		t.Fatal(err)
	}
	checkpoint, err = loadMeta(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if syncs != 2 || checkpoint.donePieces() != 2 {
		t.Fatalf("late completion not synced and saved: syncs=%d blocks=%d", syncs, checkpoint.donePieces())
	}
}

func TestConcurrentBlockCompletionCountedOnce(t *testing.T) {
	const blockSize = 32 << 10
	data := bytes.Repeat([]byte("x"), blockSize)
	m := newMeta(nil, "file", probeInfo{Size: blockSize}, blockSize)
	r := &gatedBlockReader{bytes.NewReader(data), blockSize, make(chan struct{}, 2), make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(r.release) }) }
	defer unblock()
	counts, errs := make(chan int, 2), make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			n, err := m.markRangeCompleteFromFile(r, 0, blockSize-1, "")
			counts <- n
			errs <- err
		}()
	}
	awaitMetadataTest(t, r.started)
	awaitMetadataTest(t, r.started)
	unblock()
	n := awaitMetadataTest(t, counts) + awaitMetadataTest(t, counts)
	for i := 0; i < 2; i++ {
		if err := awaitMetadataTest(t, errs); err != nil {
			t.Fatal(err)
		}
	}
	if n != 1 || m.donePieces() != 1 {
		t.Fatalf("duplicate completion: changed=%d", n)
	}
	want := sha256.Sum256(data)
	if !bytes.Equal(m.hashes, want[:]) {
		t.Fatal("wrong completed block hash")
	}
}

func TestBlockHashFailureKeepsEarlierCompletionDirty(t *testing.T) {
	const blockSize = 32 << 10
	m := newMeta(nil, "file", probeInfo{Size: 2 * blockSize}, blockSize)
	r := bytes.NewReader(make([]byte, blockSize))
	n, err := m.markRangeCompleteFromFile(r, 0, 2*blockSize-1, "")
	if n != 1 || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("completion: n=%d err=%v", n, err)
	}
	if !m.dirty || !testBit(m.bits, 0) || testBit(m.bits, 1) {
		t.Fatalf("invalid state after partial hash failure: dirty=%v bits=%08b", m.dirty, m.bits)
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
		t.Fatal("expected a segmented download")
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
	if m.donePieces() != 3 || m.completedBytes() != int64(len(data)) {
		t.Fatalf("checkpoint omitted completed data: blocks=%d bytes=%d", m.donePieces(), m.completedBytes())
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := m.verifyCompletedBlocks(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if m.donePieces() != 3 {
		t.Fatal("checkpoint block hashes do not match saved bytes")
	}
}
