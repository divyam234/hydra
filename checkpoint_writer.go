package hydra

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
)

// checkpointWriter belongs to one block-aligned range job and survives that
// job's HTTP retries. Hash only bytes accepted by the output, in offset order.
// Resume still checks the actual file bytes against these expected digests;
// saveIfDueAfterSync still syncs output before publishing the checkpoint.
type checkpointWriter struct {
	output    io.WriterAt
	meta      *metaFile
	hash      hash.Hash
	next      int64
	completed int
}

func newCheckpointWriter(output io.WriterAt, meta *metaFile, start int64) *checkpointWriter {
	return &checkpointWriter{output: output, meta: meta, hash: sha256.New(), next: start}
}

func (w *checkpointWriter) WriteAt(p []byte, off int64) (int, error) {
	if off != w.next || off < 0 || off > w.meta.Size || int64(len(p)) > w.meta.Size-off {
		return 0, fmt.Errorf("nonsequential checkpoint write at %d (expected %d)", off, w.next)
	}
	n, err := w.output.WriteAt(p, off)
	accepted := p[:n]
	for len(accepted) > 0 {
		index := int(w.next / w.meta.BlockSize)
		remaining := min(w.meta.BlockSize-w.next%w.meta.BlockSize, w.meta.Size-w.next)
		count := min(int64(len(accepted)), remaining)
		_, _ = w.hash.Write(accepted[:count])
		w.next += count
		accepted = accepted[count:]
		if count == remaining {
			var digest [sha256.Size]byte
			w.hash.Sum(digest[:0])
			if w.meta.markBlockComplete(index, digest) {
				w.completed++
			}
			w.hash.Reset()
		}
	}
	return n, err
}
