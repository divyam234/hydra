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
	"sync/atomic"
	"testing"
	"time"
)

type partialCheckpointOutput struct {
	io.WriterAt
	limit int
	err   error
}

func (w *partialCheckpointOutput) WriteAt(p []byte, off int64) (int, error) {
	if w.err == nil {
		return w.WriterAt.WriteAt(p, off)
	}
	err := w.err
	w.err = nil
	n, writeErr := w.WriterAt.WriteAt(p[:min(len(p), w.limit)], off)
	if writeErr != nil {
		return n, writeErr
	}
	return n, err
}

func TestCheckpointWriterHashesOnlyAcceptedBytesAcrossRetries(t *testing.T) {
	const blockSize = 32 << 10
	data := make([]byte, 3*blockSize+123)
	for i := range data {
		data[i] = byte(i*31 + i/257)
	}
	broken := errors.New("write interrupted")
	for _, accepted := range []int{0, 17, blockSize, blockSize + 17} {
		t.Run(fmt.Sprint(accepted), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			m := newMeta(nil, path, probeInfo{Size: int64(len(data))}, blockSize)
			out := &partialCheckpointOutput{WriterAt: f, limit: accepted, err: broken}
			w := newCheckpointWriter(out, m, 0)
			n, err := w.WriteAt(data, 0)
			if n != accepted || !errors.Is(err, broken) {
				t.Fatalf("partial write: n=%d err=%v", n, err)
			}
			if w.next != int64(accepted) || m.donePieces() != accepted/blockSize {
				t.Fatalf("unwritten bytes checkpointed: next=%d blocks=%d", w.next, m.donePieces())
			}
			// The same job resumes within the block, retaining its partial hash.
			// Split remaining writes across non-block-aligned buffers as well.
			for off := accepted; off < len(data); {
				end := min(off+19001, len(data))
				n, err = w.WriteAt(data[off:end], int64(off))
				if err != nil || n != end-off {
					t.Fatalf("retry: n=%d err=%v", n, err)
				}
				off += n
			}
			if w.completed != 4 || m.donePieces() != 4 {
				t.Fatalf("completed=%d blocks=%d", w.completed, m.donePieces())
			}
			for i := 0; i < m.BlockCount; i++ {
				want := sha256.Sum256(data[i*blockSize : min((i+1)*blockSize, len(data))])
				if !bytes.Equal(m.hashes[i*sha256.Size:(i+1)*sha256.Size], want[:]) {
					t.Fatalf("block %d hash mismatch", i)
				}
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, data) {
				t.Fatal("written bytes mismatch")
			}
		})
	}
}

func TestCheckpointWriterDetectsDamageOnResume(t *testing.T) {
	const blockSize = 32 << 10
	path := filepath.Join(t.TempDir(), "file")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data := bytes.Repeat([]byte("x"), 2*blockSize+123)
	m := newMeta(nil, path, probeInfo{Size: int64(len(data))}, blockSize)
	w := newCheckpointWriter(f, m, blockSize)
	if _, err := w.WriteAt(data[blockSize:], blockSize); err != nil {
		t.Fatal(err)
	}
	if testBit(m.bits, 0) || !testBit(m.bits, 1) || !testBit(m.bits, 2) {
		t.Fatalf("wrong range completed: %08b", m.bits)
	}
	if _, err := f.WriteAt([]byte("corrupt"), blockSize+100); err != nil {
		t.Fatal(err)
	}
	if err := m.saveIfDueAfterSync(sidecarPath(path), 0, f.Sync); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadMeta(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.verifyCompletedBlocks(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if testBit(loaded.bits, 1) || !testBit(loaded.bits, 2) {
		t.Fatalf("damaged block trusted on resume: %08b", loaded.bits)
	}
	if _, err := w.WriteAt([]byte("out of order"), blockSize); err == nil {
		t.Fatal("accepted an out-of-order hash input")
	}
}

type countedRangeOutput struct {
	io.WriterAt
	writes int
}

type notifyingCheckpointOutput struct {
	io.WriterAt
	accepted chan struct{}
}

func (w *notifyingCheckpointOutput) WriteAt(p []byte, off int64) (int, error) {
	n, err := w.WriterAt.WriteAt(p, off)
	close(w.accepted)
	return n, err
}

func TestCheckpointWriteDuringSyncNeedsAnotherCheckpoint(t *testing.T) {
	const blockSize = 32 << 10
	path := filepath.Join(t.TempDir(), "file")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data := bytes.Repeat([]byte("x"), blockSize)
	m := newMeta(nil, path, probeInfo{Size: 2 * blockSize}, blockSize)
	if _, err := newCheckpointWriter(f, m, 0).WriteAt(data, 0); err != nil {
		t.Fatal(err)
	}
	accepted, syncing := make(chan struct{}), make(chan struct{})
	// Unblock the sync even if the test fails before the second write.
	abort := make(chan struct{})
	defer close(abort)
	saved := make(chan error, 1)
	go func() {
		saved <- m.saveIfDueAfterSync(sidecarPath(path), 0, func() error {
			close(syncing)
			select {
			case <-accepted:
			case <-abort:
			}
			return f.Sync()
		})
	}()
	awaitMetadataTest(t, syncing)
	written := make(chan error, 1)
	go func() {
		w := newCheckpointWriter(&notifyingCheckpointOutput{f, accepted}, m, blockSize)
		_, err := w.WriteAt(data, blockSize)
		written <- err
	}()
	if err := awaitMetadataTest(t, saved); err != nil {
		t.Fatal(err)
	}
	if err := awaitMetadataTest(t, written); err != nil {
		t.Fatal(err)
	}
	first, err := loadMeta(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if first.donePieces() != 1 || testBit(first.bits, 1) {
		t.Fatal("a completion during sync was included in the earlier checkpoint")
	}
	synced := false
	if err := m.saveIfDueAfterSync(sidecarPath(path), 0, func() error { synced = true; return f.Sync() }); err != nil {
		t.Fatal(err)
	}
	second, err := loadMeta(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if !synced || second.donePieces() != 2 {
		t.Fatal("late completion was not synced and checkpointed")
	}
}

func (w *countedRangeOutput) WriteAt(p []byte, off int64) (int, error) {
	w.writes++
	return w.WriterAt.WriteAt(p, off)
}

func TestCopyRangeCoalescesShortReadsAndPreservesFullBufferError(t *testing.T) {
	broken := errors.New("reader interrupted")
	for _, terminal := range []error{io.EOF, broken} {
		t.Run(terminal.Error(), func(t *testing.T) {
			f, err := os.Create(filepath.Join(t.TempDir(), "file"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			out := &countedRangeOutput{WriterAt: f}
			data := []byte("abcdefghijklmnop")
			r := &oneShotTerminalReader{terminalChunkReader{data: data, err: terminal}}
			tracker := newProgressTracker("copy", 32, 1, 1, "", time.Hour, nil)
			n, err := copyRangeToFile(out, r, 0, 32, make([]byte, 8), tracker, nil)
			wantErr := terminal
			if terminal == io.EOF {
				wantErr = io.ErrUnexpectedEOF
			}
			if n != int64(len(data)) || !errors.Is(err, wantErr) {
				t.Fatalf("copy: n=%d err=%v", n, err)
			}
			if out.writes != 2 {
				t.Fatalf("short reads caused %d writes, want 2", out.writes)
			}
			got, err := os.ReadFile(f.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, data) {
				t.Fatal("partial read bytes were lost")
			}
		})
	}
}

// A later read returns EOF rather than repeating the original error, so a
// coalescing reader cannot discard a data-bearing error and recover it later.
type oneShotTerminalReader struct {
	reader terminalChunkReader
}

func (r *oneShotTerminalReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil {
		r.reader.err = io.EOF
	}
	return n, err
}

func TestSegmentedHTTPRetryKeepsPartialBlockHash(t *testing.T) {
	const blockSize, interruptedAt = 32 << 10, 19001
	data := make([]byte, 4*blockSize+123)
	for i := range data {
		data[i] = byte(i*31 + i/257)
	}
	var interrupted, retried atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"stable"`)
		var start, end int
		if r.Method == http.MethodGet {
			if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err == nil {
				if start == interruptedAt {
					retried.Store(true)
				}
				if start == 0 && end >= blockSize-1 && interrupted.CompareAndSwap(false, true) {
					w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
					w.WriteHeader(http.StatusPartialContent)
					_, _ = w.Write(data[:interruptedAt])
					return // A short body forces an HTTP retry within the first block.
				}
			}
		}
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{
		Split: 2, MaxConnectionsPerServer: 2, MinSplitSize: 1, BufferSize: blockSize,
		ResumeBlockSize: blockSize, KeepPartFile: true, MaxRetries: 1,
		RetryWait: time.Nanosecond, MaxRetryWait: time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !interrupted.Load() || !retried.Load() {
		t.Fatal("did not exercise an HTTP retry within a block")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("retried download content mismatch")
	}
	m, err := loadMeta(sidecarPath(path))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := m.verifyCompletedBlocks(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	if m.donePieces() != 5 {
		t.Fatal("retry lost hash state or published the wrong block digest")
	}
}
