package hydra

import (
	"fmt"
	"io"
)

// checkpointWriter tracks fully written resume blocks. The payload is written
// directly to the destination; no per-block digest is computed. A block is
// marked complete only after the output writer accepts its final byte.
// Metadata is published only after the file data has been synced.
type checkpointWriter struct {
	output    io.WriterAt
	meta      *metaFile
	next      int64
	completed int
}

func newCheckpointWriter(output io.WriterAt, meta *metaFile, start int64) *checkpointWriter {
	return &checkpointWriter{output: output, meta: meta, next: start}
}

func (w *checkpointWriter) WriteAt(p []byte, off int64) (int, error) {
	if off != w.next || off < 0 || off > w.meta.Size || int64(len(p)) > w.meta.Size-off {
		return 0, fmt.Errorf("nonsequential checkpoint write at %d (expected %d)", off, w.next)
	}
	n, err := w.output.WriteAt(p, off)
	accepted := int64(n)
	for accepted > 0 {
		index := int(w.next / w.meta.BlockSize)
		remaining := min(w.meta.BlockSize-w.next%w.meta.BlockSize, w.meta.Size-w.next)
		count := min(accepted, remaining)
		w.next += count
		accepted -= count
		if count == remaining && w.meta.markBlockComplete(index) {
			w.completed++
		}
	}
	return n, err
}
