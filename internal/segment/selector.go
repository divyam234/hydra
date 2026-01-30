package segment

import (
	"math/rand"
	"time"
)

// PieceSelector selects which piece to download next
type PieceSelector interface {
	Select(ps PieceStorage, activePieces map[int]bool) int
}

// InOrderSelector selects the first available piece
type InOrderSelector struct{}

func (s *InOrderSelector) Select(ps PieceStorage, activePieces map[int]bool) int {
	totalPieces := ps.GetNumPieces()
	for i := 0; i < totalPieces; i++ {
		if !ps.HasPiece(i) && !activePieces[i] {
			return i
		}
	}
	return -1
}

// RandomSelector selects a random available piece
type RandomSelector struct {
	rng *rand.Rand
}

func NewRandomSelector() *RandomSelector {
	return &RandomSelector{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *RandomSelector) Select(ps PieceStorage, activePieces map[int]bool) int {
	totalPieces := ps.GetNumPieces()

	// Collect candidates
	var candidates []int
	for i := 0; i < totalPieces; i++ {
		if !ps.HasPiece(i) && !activePieces[i] {
			candidates = append(candidates, i)
		}
	}

	if len(candidates) == 0 {
		return -1
	}

	// Pick random
	return candidates[s.rng.Intn(len(candidates))]
}
