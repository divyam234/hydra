package hydra

import "sync"

// rangeLease owns [piece.Start, end]. A downloader reserves its next read
// window before touching the response body. Stealing may only shorten end
// *after* that reservation, on a resume-block boundary, so neither worker
// can write the other's bytes even if a socket read is in progress.
type rangeLease struct {
	mu          sync.Mutex
	job         piece
	end         int64
	reservedEnd int64
}

func newRangeLease(p piece) *rangeLease {
	return &rangeLease{job: p, end: p.End, reservedEnd: p.Start - 1}
}

// reserve limits the next read to the range still owned by this worker.
// Only the owning worker calls reserve; other workers only split the tail.
func (l *rangeLease) reserve(offset int64, want int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if want <= 0 || offset < l.job.Start || offset > l.end {
		return 0
	}
	n := int(min(int64(want), l.end-offset+1))
	if last := offset + int64(n) - 1; last > l.reservedEnd {
		l.reservedEnd = last
	}
	return n
}

func (l *rangeLease) limit() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.end
}

// splitPointLocked calculates a checkpoint-aligned tail while retaining
// at least minChunk bytes on both sides of the worker's last reserved read.
// The lease lock must be held by the caller.
func (l *rangeLease) splitPointLocked(blockSize, minChunk int64) int64 {
	if blockSize <= 0 {
		return 0
	}
	minChunk = max(minChunk, blockSize)
	firstUnreserved := max(l.reservedEnd+1, l.job.Start)
	if firstUnreserved > l.end || (l.end-firstUnreserved+1)/2 < minChunk {
		return 0
	}

	// The cutoff is aligned to a checkpoint block, so neither owner hashes
	// a partial block. Advance to the next block if midpoint rounding
	// leaves insufficient bytes before the cutoff.
	split := firstUnreserved + (l.end-firstUnreserved+1)/2
	split -= split % blockSize
	if split-firstUnreserved < minChunk {
		split += blockSize
	}
	if split > l.end || split-firstUnreserved < minChunk || l.end-split+1 < minChunk {
		return 0
	}
	return split
}

// rangeScheduler initially assigns roughly one HTTP Range request per worker.
// When a worker becomes idle it steals an as-yet-unread tail from the largest
// running range. Unlike eagerly creating 4x as many requests, fast/uniform
// servers stay on one request/worker; uneven connections gain extra work.
type rangeScheduler struct {
	mu        sync.Mutex
	pending   []piece
	nextJob   int
	active    map[*rangeLease]struct{}
	blockSize int64
	minChunk  int64
}

func newRangeScheduler(pending []piece, blockSize, minChunk int64) *rangeScheduler {
	next := 0
	for _, p := range pending {
		next = max(next, p.Index+1)
	}
	return &rangeScheduler{
		pending: pending, nextJob: next, active: make(map[*rangeLease]struct{}),
		blockSize: blockSize, minChunk: minChunk,
	}
}

func (s *rangeScheduler) claim() *rangeLease {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pending) > 0 {
		p := s.pending[0]
		s.pending[0] = piece{}
		s.pending = s.pending[1:]
		lease := newRangeLease(p)
		s.active[lease] = struct{}{}
		return lease
	}

	// Only steal if no unclaimed work remains. Compare remaining bytes under
	// each lease lock; the scheduler lock serializes competing stealers.
	var best *rangeLease
	var maxRemaining int64
	for lease := range s.active {
		lease.mu.Lock()
		split := lease.splitPointLocked(s.blockSize, s.minChunk)
		remaining := lease.end - lease.reservedEnd
		lease.mu.Unlock()
		if split != 0 && remaining > maxRemaining {
			best, maxRemaining = lease, remaining
		}
	}
	if best == nil {
		return nil
	}

	best.mu.Lock()
	split := best.splitPointLocked(s.blockSize, s.minChunk)
	if split == 0 { // Owner advanced past a safe split while we selected it.
		best.mu.Unlock()
		return nil
	}
	oldEnd := best.end
	best.end = split - 1
	best.mu.Unlock()

	p := piece{Index: s.nextJob, Start: split, End: oldEnd, State: piecePending}
	s.nextJob++
	lease := newRangeLease(p)
	s.active[lease] = struct{}{}
	return lease
}

func (s *rangeScheduler) finish(lease *rangeLease) {
	s.mu.Lock()
	delete(s.active, lease)
	s.mu.Unlock()
}
