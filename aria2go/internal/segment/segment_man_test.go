package segment

import (
	"testing"
)

type mockPieceStorage struct {
	numPieces   int
	pieceLength int64
	totalLength int64
	pieces      []*Piece
	bitfield    *BitfieldMan
}

func (m *mockPieceStorage) GetTotalLength() int64     { return m.totalLength }
func (m *mockPieceStorage) GetNumPieces() int         { return m.numPieces }
func (m *mockPieceStorage) GetPieceLength() int64     { return m.pieceLength }
func (m *mockPieceStorage) GetPiece(index int) *Piece { return m.pieces[index] }
func (m *mockPieceStorage) GetBitfield() *BitfieldMan { return m.bitfield }
func (m *mockPieceStorage) HasPiece(index int) bool   { return m.bitfield.HasBit(index) }
func (m *mockPieceStorage) CompletePiece(index int)   { m.bitfield.SetBit(index) }
func (m *mockPieceStorage) IsAllPieceSet() bool       { return m.bitfield.IsAllBitSet() }

func newMockPieceStorage(num int, length int64) *mockPieceStorage {
	ps := &mockPieceStorage{
		numPieces:   num,
		pieceLength: length,
		totalLength: int64(num) * length,
		pieces:      make([]*Piece, num),
		bitfield:    NewBitfieldMan(num),
	}
	for i := 0; i < num; i++ {
		ps.pieces[i] = NewPiece(i, length, int64(i)*length)
	}
	return ps
}

func TestSegmentMan_GetSegment(t *testing.T) {
	ps := newMockPieceStorage(100, 1024)
	sm := NewSegmentMan(ps)

	// Get first segment
	seg1 := sm.GetSegment()
	if seg1 == nil {
		t.Fatal("Expected segment, got nil")
	}
	if seg1.Position != 0 {
		t.Errorf("Expected position 0, got %d", seg1.Position)
	}
	// Default maxPieces is 20
	expectedLen := int64(20 * 1024)
	if seg1.Length != expectedLen {
		t.Errorf("Expected length %d, got %d", expectedLen, seg1.Length)
	}

	// Get second segment
	seg2 := sm.GetSegment()
	if seg2 == nil {
		t.Fatal("Expected second segment")
	}
	if seg2.Position != expectedLen {
		t.Errorf("Expected position %d, got %d", expectedLen, seg2.Position)
	}

	// Mark first segment complete
	sm.CompleteSegment(seg1.Index)
	if !ps.HasPiece(0) {
		t.Error("Piece 0 should be complete")
	}
	if !ps.HasPiece(19) {
		t.Error("Piece 19 should be complete")
	}
	if ps.HasPiece(20) {
		t.Error("Piece 20 should not be complete")
	}

	// Cancel second segment
	sm.CancelSegment(seg2.Index)

	// Get segment again, should be the one we just canceled
	seg3 := sm.GetSegment()
	if seg3.Position != seg2.Position {
		t.Errorf("Expected to re-get canceled segment at %d, got %d", seg2.Position, seg3.Position)
	}
}

func TestSegmentMan_IsAllComplete(t *testing.T) {
	ps := newMockPieceStorage(5, 1024)
	sm := NewSegmentMan(ps)

	if sm.IsAllComplete() {
		t.Error("Should not be complete initially")
	}

	// Get and complete one big segment
	seg := sm.GetSegment()
	if seg.Length != 5*1024 {
		t.Errorf("Expected length 5120, got %d", seg.Length)
	}
	sm.CompleteSegment(seg.Index)

	if !sm.IsAllComplete() {
		t.Error("Should be complete now")
	}
}
