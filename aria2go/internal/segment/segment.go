package segment

import (
	"fmt"
)

// Segment represents a contiguous range of bytes to download
type Segment struct {
	Index      int   // Unique index for the segment
	Position   int64 // Start offset in the file
	Length     int64 // Total length of the segment
	Written    int64 // Bytes successfully written/downloaded
	IsComplete bool  // Whether the segment is fully downloaded
	IsCanceled bool  // Whether the segment was canceled
}

// NewSegment creates a new segment
func NewSegment(index int, position int64, length int64) *Segment {
	return &Segment{
		Index:    index,
		Position: position,
		Length:   length,
	}
}

// UpdateWritten updates the written bytes count
func (s *Segment) UpdateWritten(bytes int64) {
	s.Written += bytes
	if s.Written >= s.Length {
		s.Written = s.Length
		s.IsComplete = true
	}
}

// GetRemaining returns the number of bytes left to download
func (s *Segment) GetRemaining() int64 {
	return s.Length - s.Written
}

// String returns a summary of the segment
func (s *Segment) String() string {
	return fmt.Sprintf("Seg#%d[%d-%d](%d/%d)",
		s.Index, s.Position, s.Position+s.Length, s.Written, s.Length)
}
