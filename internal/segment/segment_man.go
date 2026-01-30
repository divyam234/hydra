package segment

import (
	"sync"
)

// SegmentMan manages the segments of a download
type SegmentMan struct {
	pieceStorage        PieceStorage
	segments            map[int]*Segment // Active segments by segment index
	mu                  sync.Mutex
	nextSegIndex        int
	maxPiecesPerSegment int
	selector            PieceSelector
}

// NewSegmentMan creates a new SegmentMan
func NewSegmentMan(ps PieceStorage, maxPieces int) *SegmentMan {
	if maxPieces <= 0 {
		maxPieces = 20
	}
	return &SegmentMan{
		pieceStorage:        ps,
		segments:            make(map[int]*Segment),
		maxPiecesPerSegment: maxPieces,
		selector:            &InOrderSelector{}, // Default
	}
}

// SetSelector sets the piece selection strategy
func (sm *SegmentMan) SetSelector(s PieceSelector) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if s != nil {
		sm.selector = s
	}
}

// GetSegment returns a new segment to download
func (sm *SegmentMan) GetSegment() *Segment {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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

	// 1. Try to select a new piece using the strategy
	startPiece := sm.selector.Select(sm.pieceStorage, activePieces)

	if startPiece != -1 {
		// Greedily grab contiguous missing pieces
		endPiece := startPiece
		totalPieces := sm.pieceStorage.GetNumPieces()
		maxPieces := sm.maxPiecesPerSegment

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
		return seg
	}

	// 2. Endgame Mode: No new pieces available.
	// Try to steal from the slowest/largest active segment.
	if len(sm.segments) > 0 {
		// Find segment with most remaining bytes
		var bestSeg *Segment
		var maxRem int64 = -1

		// Iteration order is random in Go maps, which acts as a random tie-breaker
		for _, seg := range sm.segments {
			rem := seg.GetRemaining()
			if rem > maxRem {
				maxRem = rem
				bestSeg = seg
			}
		}

		// Minimum size to split (e.g. 256KB or 2 pieces worth?)
		// Let's ensure we don't split too small
		minSplit := int64(256 * 1024)
		if maxRem > minSplit*2 && bestSeg != nil {
			newSeg := bestSeg.Split(minSplit)
			if newSeg != nil {
				newSeg.Index = sm.nextSegIndex
				sm.segments[sm.nextSegIndex] = newSeg
				sm.nextSegIndex++
				// fmt.Printf("Endgame: Split segment %d -> new segment %d (%d bytes)\n", bestSeg.Index, newSeg.Index, newSeg.Length)
				return newSeg
			}
		}
	}

	return nil // No more work possible
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
