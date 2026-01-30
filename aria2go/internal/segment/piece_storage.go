package segment

import (
	"fmt"
	"sync"
)

// PieceStorage manages the pieces of a download
type PieceStorage interface {
	GetTotalLength() int64
	GetNumPieces() int
	GetPieceLength() int64
	GetPiece(index int) *Piece
	GetBitfield() *BitfieldMan
	HasPiece(index int) bool
	CompletePiece(index int)
	IsAllPieceSet() bool
}

// DefaultPieceStorage implements PieceStorage for known-length files
type DefaultPieceStorage struct {
	totalLength int64
	pieceLength int64
	numPieces   int
	pieces      []*Piece
	bitfield    *BitfieldMan
	mu          sync.RWMutex
}

// NewDefaultPieceStorage creates a new DefaultPieceStorage
func NewDefaultPieceStorage(totalLength int64, pieceLength int64) *DefaultPieceStorage {
	if pieceLength <= 0 {
		pieceLength = 1024 * 1024 // Default 1MB
	}

	numPieces := int((totalLength + pieceLength - 1) / pieceLength)

	pieces := make([]*Piece, numPieces)
	for i := 0; i < numPieces; i++ {
		length := pieceLength
		if i == numPieces-1 {
			length = totalLength - int64(i)*pieceLength
		}
		pieces[i] = NewPiece(i, length, int64(i)*pieceLength)
	}

	return &DefaultPieceStorage{
		totalLength: totalLength,
		pieceLength: pieceLength,
		numPieces:   numPieces,
		pieces:      pieces,
		bitfield:    NewBitfieldMan(numPieces),
	}
}

func (ps *DefaultPieceStorage) GetTotalLength() int64 {
	return ps.totalLength
}

func (ps *DefaultPieceStorage) GetNumPieces() int {
	return ps.numPieces
}

func (ps *DefaultPieceStorage) GetPieceLength() int64 {
	return ps.pieceLength
}

func (ps *DefaultPieceStorage) GetPiece(index int) *Piece {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if index < 0 || index >= ps.numPieces {
		return nil
	}
	return ps.pieces[index]
}

func (ps *DefaultPieceStorage) GetBitfield() *BitfieldMan {
	return ps.bitfield
}

func (ps *DefaultPieceStorage) HasPiece(index int) bool {
	return ps.bitfield.HasBit(index)
}

func (ps *DefaultPieceStorage) CompletePiece(index int) {
	ps.bitfield.SetBit(index)
}

func (ps *DefaultPieceStorage) IsAllPieceSet() bool {
	return ps.bitfield.IsAllBitSet()
}

// GetMissingPieceIndex returns the index of the first missing piece
// starting from the given index. Returns -1 if no missing pieces found.
func (ps *DefaultPieceStorage) GetMissingPieceIndex(startIndex int) int {
	return ps.bitfield.GetFirstMissingBit(startIndex)
}

// CalculateOptimalPieceLength calculates a reasonable piece size
// based on the total file size.
//
// Strategy similar to aria2:
// < 50MiB: 1MiB
// < 250MiB: 2MiB
// < 500MiB: 4MiB
// ...
func CalculateOptimalPieceLength(totalLength int64) int64 {
	if totalLength <= 50*1024*1024 {
		return 1 * 1024 * 1024
	}

	pieceLength := int64(1 * 1024 * 1024)
	for totalLength > 50*1024*1024 {
		pieceLength *= 2
		totalLength /= 2
		if pieceLength > 16*1024*1024 { // Cap at 16MB
			return 16 * 1024 * 1024
		}
	}
	return pieceLength
}

// String provides summary
func (ps *DefaultPieceStorage) String() string {
	return fmt.Sprintf("Storage: %d pieces, %d bytes total, %d bytes/piece. Done: %d/%d",
		ps.numPieces, ps.totalLength, ps.pieceLength, ps.bitfield.CountSetBit(), ps.numPieces)
}
