package segment

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

// 1. Bitfield Concurrency Tests
func TestBitfieldMan_Concurrency(t *testing.T) {
	numPieces := 1000
	bf := NewBitfieldMan(numPieces)
	var wg sync.WaitGroup
	workers := 20

	// Concurrent SetBit
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numPieces; j++ {
				if j%workers == workerID {
					bf.SetBit(j)
				}
			}
		}(i)
	}
	wg.Wait()

	if bf.CountSetBit() != numPieces {
		t.Errorf("Expected %d bits, got %d", numPieces, bf.CountSetBit())
	}

	// Concurrent UnsetBit and Count
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numPieces; j++ {
				if j%workers == workerID {
					bf.UnsetBit(j)
				}
				_ = bf.CountSetBit()
			}
		}(i)
	}
	wg.Wait()

	if bf.CountSetBit() != 0 {
		t.Errorf("Expected 0 bits, got %d", bf.CountSetBit())
	}
}

func TestBitfieldMan_DeadlockPrevention(t *testing.T) {
	// Tests the fix where ToBinaryString was calling HasBit while holding a lock
	bf := NewBitfieldMan(100)
	bf.SetBit(10)

	done := make(chan struct{})
	go func() {
		_ = bf.ToBinaryString()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Deadlock detected in ToBinaryString")
	}
}

// 2. SegmentMan Concurrency and Race Tests
func TestSegmentMan_ConcurrentAllocation(t *testing.T) {
	ps := newMockPieceStorage(1000, 1024)
	sm := NewSegmentMan(ps, 20)
	var wg sync.WaitGroup
	workers := 10
	allocated := make(chan *Segment, 1000)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				seg := sm.GetSegment()
				if seg == nil {
					return
				}
				allocated <- seg
				// Simulate some work
				time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
				sm.CompleteSegment(seg.Index)
			}
		}()
	}

	wg.Wait()
	close(allocated)

	if !sm.IsAllComplete() {
		t.Error("Download should be complete")
	}
}

// 3. PieceStorage Logic Tests
func TestDefaultPieceStorage_PieceAlignment(t *testing.T) {
	totalLen := int64(1024*10 + 500) // 10 pieces of 1024 + 1 piece of 500
	pieceLen := int64(1024)
	ps := NewDefaultPieceStorage(totalLen, pieceLen)

	if ps.GetNumPieces() != 11 {
		t.Errorf("Expected 11 pieces, got %d", ps.GetNumPieces())
	}

	lastPiece := ps.GetPiece(10)
	if lastPiece.Length != 500 {
		t.Errorf("Expected last piece length 500, got %d", lastPiece.Length)
	}

	if lastPiece.Offset != 1024*10 {
		t.Errorf("Expected last piece offset %d, got %d", 1024*10, lastPiece.Offset)
	}
}

func TestCalculateOptimalPieceLength(t *testing.T) {
	tests := []struct {
		size     int64
		expected int64
	}{
		{10 * 1024 * 1024, 1 * 1024 * 1024},
		{100 * 1024 * 1024, 2 * 1024 * 1024},
		{300 * 1024 * 1024, 8 * 1024 * 1024},
		{1024 * 1024 * 1024, 16 * 1024 * 1024}, // Capped
	}

	for _, tt := range tests {
		got := CalculateOptimalPieceLength(tt.size)
		if got != tt.expected {
			t.Errorf("Size %d: Expected %d, got %d", tt.size, tt.expected, got)
		}
	}
}

// 4. Segment Recovery Scenario
func TestSegmentMan_RetryAndRecovery(t *testing.T) {
	ps := newMockPieceStorage(10, 1024)
	sm := NewSegmentMan(ps, 20)

	// Worker 1 gets segment
	seg := sm.GetSegment()
	if seg == nil {
		t.Fatal("No segment")
	}

	// Simulated crash/failure - Worker 1 fails without completing
	sm.CancelSegment(seg.Index)

	// Worker 2 should get the exact same segment
	seg2 := sm.GetSegment()
	if seg2.Position != seg.Position || seg2.Length != seg.Length {
		t.Error("Worker 2 did not recover failed segment correctly")
	}
}

// 5. PieceStorage Roundtrip via Bitfield
func TestPieceStorage_BitfieldSync(t *testing.T) {
	ps := NewDefaultPieceStorage(1024*10, 1024)

	ps.CompletePiece(0)
	ps.CompletePiece(5)

	bf := ps.GetBitfield()
	if !bf.HasBit(0) || !bf.HasBit(5) || bf.HasBit(1) {
		t.Error("Bitfield not in sync with PieceStorage")
	}
}

func TestBitfieldMan_OutOfBounds(t *testing.T) {
	bf := NewBitfieldMan(10)
	if bf.SetBit(10) {
		t.Error("SetBit should fail out of bounds")
	}
	if bf.SetBit(-1) {
		t.Error("SetBit should fail for negative")
	}
	if bf.HasBit(10) {
		t.Error("HasBit should fail out of bounds")
	}
}

func TestBitfieldMan_BulkOperations(t *testing.T) {
	bf := NewBitfieldMan(100)
	bf.SetAll()
	if bf.CountSetBit() != 100 {
		t.Errorf("Expected 100, got %d", bf.CountSetBit())
	}
	if !bf.IsAllBitSet() {
		t.Error("IsAllBitSet should be true")
	}

	bf.Clear()
	if bf.CountSetBit() != 0 {
		t.Error("Expected 0 after clear")
	}
}

func TestBitfieldMan_BinaryString(t *testing.T) {
	bf := NewBitfieldMan(4)
	bf.SetBit(0)
	bf.SetBit(2)
	s := bf.ToBinaryString()
	if s != "1010" {
		t.Errorf("Expected 1010, got %s", s)
	}
}

func TestBitfieldMan_HexEdgeCases(t *testing.T) {
	bf := NewBitfieldMan(12) // 2 bytes
	bf.SetBit(0)
	bf.SetBit(11)

	h := bf.String()
	bf2 := NewBitfieldMan(12)
	bf2.FromHexString(h)

	if !bf2.HasBit(0) || !bf2.HasBit(11) {
		t.Error("Hex roundtrip failed for non-byte-aligned size")
	}

	// Error case: odd length hex
	if bf2.FromHexString("abc") == nil {
		t.Error("Should fail for odd length hex")
	}
}
