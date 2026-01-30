package segment

import (
	"sync"
)

// SegmentMan manages the segments of a download
type SegmentMan struct {
	pieceStorage PieceStorage
	segments     map[int]*Segment // Active segments by segment index
	mu           sync.Mutex
	nextSegIndex int
}

// NewSegmentMan creates a new SegmentMan
func NewSegmentMan(ps PieceStorage) *SegmentMan {
	return &SegmentMan{
		pieceStorage: ps,
		segments:     make(map[int]*Segment),
	}
}

// GetSegment returns a new segment to download
// This is a simplified version: it finds the first missing piece sequence
// and returns a segment for it. In a real adaptive downloader, this would
// be smarter about splitting large remaining chunks.
func (sm *SegmentMan) GetSegment() *Segment {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Find first missing piece not currently being downloaded
	// This is a naive O(N) scan, can be optimized
	totalPieces := sm.pieceStorage.GetNumPieces()

	// Track which pieces are covered by active segments
	activePieces := make(map[int]bool)
	for _, seg := range sm.segments {
		// Calculate pieces covered by this segment
		startPiece := int(seg.Position / sm.pieceStorage.GetPieceLength())
		endPiece := int((seg.Position + seg.Length - 1) / sm.pieceStorage.GetPieceLength())
		for i := startPiece; i <= endPiece; i++ {
			activePieces[i] = true
		}
	}

	startPiece := -1

	for i := 0; i < totalPieces; i++ {
		if !sm.pieceStorage.HasPiece(i) && !activePieces[i] {
			startPiece = i
			break
		}
	}

	if startPiece == -1 {
		return nil // No more segments to download
	}

	// Greedily grab as many contiguous missing pieces as possible
	// up to a limit (e.g., 5MB or reasonable chunk)
	endPiece := startPiece

	// TODO: Make this configurable or smarter based on connection speed/file size
	maxPieces := 20 // Increased from 5 to allow larger segments (e.g. 20MB with 1MB pieces)

	for i := startPiece + 1; i < totalPieces && i < startPiece+maxPieces; i++ {
		if sm.pieceStorage.HasPiece(i) || activePieces[i] {
			break
		}
		endPiece = i
	}

	// Create segment
	pieceLen := sm.pieceStorage.GetPieceLength()
	offset := int64(startPiece) * pieceLen

	// Calculate length
	length := int64(0)
	for i := startPiece; i <= endPiece; i++ {
		p := sm.pieceStorage.GetPiece(i)
		length += p.Length
	}

	seg := NewSegment(sm.nextSegIndex, offset, length)
	sm.segments[sm.nextSegIndex] = seg
	sm.nextSegIndex++

	// fmt.Printf("Created segment %s for pieces %d-%d\n", seg, startPiece, endPiece)

	return seg
}

// UpdateSegment updates progress of a segment
func (sm *SegmentMan) UpdateSegment(segIndex int, bytesWritten int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	seg, ok := sm.segments[segIndex]
	if !ok {
		return
	}

	seg.UpdateWritten(bytesWritten)
}

// CompleteSegment marks a segment as complete and updates piece storage
func (sm *SegmentMan) CompleteSegment(segIndex int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	seg, ok := sm.segments[segIndex]
	if !ok {
		return
	}

	// Mark pieces as complete
	startPiece := int(seg.Position / sm.pieceStorage.GetPieceLength())
	endPiece := int((seg.Position + seg.Length - 1) / sm.pieceStorage.GetPieceLength())

	for i := startPiece; i <= endPiece; i++ {
		// Verify piece is actually fully within this segment's written range
		// For now assume perfect alignment
		sm.pieceStorage.CompletePiece(i)
	}

	delete(sm.segments, segIndex)
}

// CancelSegment returns a segment to the pool
func (sm *SegmentMan) CancelSegment(segIndex int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.segments, segIndex)
}

// IsAllComplete checks if download is finished
func (sm *SegmentMan) IsAllComplete() bool {
	return sm.pieceStorage.IsAllPieceSet()
}
