package segment

import (
	"testing"
)

func TestInOrderSelector(t *testing.T) {
	// 10MB total, 1MB pieces = 10 pieces (0-9)
	ps := NewDefaultPieceStorage(10*1024*1024, 1*1024*1024)
	selector := &InOrderSelector{}
	active := make(map[int]bool)

	// Initially, 0 should be selected
	idx := selector.Select(ps, active)
	if idx != 0 {
		t.Errorf("Expected 0, got %d", idx)
	}

	// Mark 0 as active
	active[0] = true
	idx = selector.Select(ps, active)
	if idx != 1 {
		t.Errorf("Expected 1, got %d", idx)
	}

	// Mark 1 as done
	ps.CompletePiece(1)
	idx = selector.Select(ps, active)
	if idx != 2 {
		t.Errorf("Expected 2, got %d", idx)
	}

	// Mark all done/active except 9
	for i := 2; i < 9; i++ {
		ps.CompletePiece(i)
	}
	idx = selector.Select(ps, active)
	if idx != 9 {
		t.Errorf("Expected 9, got %d", idx)
	}

	// Complete 9
	ps.CompletePiece(9)
	idx = selector.Select(ps, active)
	if idx != -1 {
		t.Errorf("Expected -1, got %d", idx)
	}
}

func TestRandomSelector(t *testing.T) {
	// 10 pieces
	ps := NewDefaultPieceStorage(10*1024*1024, 1*1024*1024)
	selector := NewRandomSelector()
	active := make(map[int]bool)

	// Select multiple times, ensure within range
	for i := 0; i < 50; i++ {
		idx := selector.Select(ps, active)
		if idx < 0 || idx >= 10 {
			t.Errorf("Invalid index %d", idx)
		}
	}

	// Mark all except 5 as done
	for i := 0; i < 10; i++ {
		if i != 5 {
			ps.CompletePiece(i)
		}
	}

	idx := selector.Select(ps, active)
	if idx != 5 {
		t.Errorf("Expected 5, got %d", idx)
	}

	// Mark 5 active
	active[5] = true
	idx = selector.Select(ps, active)
	if idx != -1 {
		t.Errorf("Expected -1, got %d", idx)
	}
}
