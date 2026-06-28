package hydra

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMirrorScorerPrefersSuccessfulFastMirror(t *testing.T) {
	s := newMirrorScorer()
	urls := []string{"slow", "fast", "unknown"}
	s.observe("slow", 1024, time.Second, nil)
	s.observe("fast", 1024*1024, time.Second, nil)
	ordered := s.ordered(urls, 0)
	if ordered[0] != "fast" {
		t.Fatalf("ordered=%v, want fast first", ordered)
	}
	s.observe("fast", 0, time.Millisecond, errors.New("failure"))
	s.observe("fast", 0, time.Millisecond, errors.New("failure"))
	ordered = s.ordered(urls, 0)
	if ordered[0] == "unknown" && ordered[1] == "unknown" {
		t.Fatalf("invalid ordering: %v", ordered)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("3", now); got != 3*time.Second {
		t.Fatalf("seconds retry-after=%v", got)
	}
	if got := parseRetryAfter(now.Add(5*time.Second).Format(http.TimeFormat), now); got != 5*time.Second {
		t.Fatalf("date retry-after=%v", got)
	}
	if got := parseRetryAfter("invalid", now); got != 0 {
		t.Fatalf("invalid retry-after=%v", got)
	}
}

func TestRetryDelayForHonorsRetryAfterAndCap(t *testing.T) {
	d, err := New(Options{RetryWait: time.Second, MaxRetryWait: 4 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	statusErr := &HTTPStatusError{StatusCode: 429, RetryAfter: 3 * time.Second}
	if got := d.retryDelayFor(0, statusErr); got != 3*time.Second {
		t.Fatalf("delay=%v want 3s", got)
	}
	statusErr.RetryAfter = 10 * time.Second
	if got := d.retryDelayFor(0, statusErr); got != 10*time.Second {
		t.Fatalf("server delay=%v want 10s", got)
	}
}

func TestPendingPiecesSupportsOverdecomposedWorkQueue(t *testing.T) {
	m := newMeta([]string{"http://example.test/file"}, "file", probeInfo{Size: 64 << 20}, 1<<20)
	pieces := m.pendingPieces(16, 1<<20)
	if len(pieces) != 16 {
		t.Fatalf("pieces=%d want 16", len(pieces))
	}
	for i := 1; i < len(pieces); i++ {
		if pieces[i-1].End+1 != pieces[i].Start {
			t.Fatalf("gap or overlap between %+v and %+v", pieces[i-1], pieces[i])
		}
	}
}

func TestMetadataFlusherPersistsDirtyMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.bin")
	meta := newMeta([]string{"http://example.test/file"}, path, probeInfo{Size: 2 << 20}, 1<<20)
	sidecar := sidecarPath(path)
	flusher := startMetadataFlusher(meta, sidecar, 0)
	if _, err := meta.markRangeComplete(0, (1<<20)-1, sidecar); err != nil {
		t.Fatal(err)
	}
	flusher.request()
	deadline := time.Now().Add(time.Second)
	for {
		loaded, err := loadMeta(sidecar)
		if err == nil && loaded.completedBytes() == 1<<20 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("metadata was not flushed: %v", err)
		}
		time.Sleep(time.Millisecond)
	}
	if err := flusher.close(); err != nil {
		t.Fatal(err)
	}
}

func TestPreallocateFileSetsSize(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "preallocated.bin"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	const size = int64(2 << 20)
	if err := preallocateFile(f, size); err != nil {
		t.Fatal(err)
	}
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != size {
		t.Fatalf("size=%d want %d", st.Size(), size)
	}
}

func BenchmarkParseContentRange(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if err := validateContentRange("bytes 1048576-2097151/1073741824", 1048576, 2097151, 1073741824); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPendingPieces(b *testing.B) {
	m := newMeta([]string{"http://example.test/file"}, "file", probeInfo{Size: 4 << 30}, 1<<20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pieces := m.pendingPieces(64, 20<<20)
		if len(pieces) == 0 {
			b.Fatal("no pieces")
		}
	}
}

func BenchmarkCopyWithProgress(b *testing.B) {
	data := bytes.Repeat([]byte("x"), 8<<20)
	buf := make([]byte, 256<<10)
	tracker := newProgressTracker("bench", int64(len(data)), 1, 1, "", time.Hour, nil)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tracker.setCompleted(0)
		if _, err := copyWithProgress(io.Discard, bytes.NewReader(data), buf, tracker); err != nil {
			b.Fatal(err)
		}
	}
}
