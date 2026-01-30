package segment

// Piece represents a chunk of the file
type Piece struct {
	Index  int   // Index of the piece
	Length int64 // Length of the piece in bytes
	Offset int64 // Offset in the file
}

// NewPiece creates a new Piece
func NewPiece(index int, length int64, offset int64) *Piece {
	return &Piece{
		Index:  index,
		Length: length,
		Offset: offset,
	}
}

// BlockSize is the standard block size for piece download (16KB)
const BlockSize = 16 * 1024
