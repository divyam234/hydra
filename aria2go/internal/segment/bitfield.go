package segment

import (
	"fmt"
	"strings"
	"sync"
)

// BitfieldMan manages a bitfield of pieces
type BitfieldMan struct {
	bitfield   []byte
	numPieces  int
	cachedOnes int
	mu         sync.RWMutex
}

// NewBitfieldMan creates a new BitfieldMan with numPieces
func NewBitfieldMan(numPieces int) *BitfieldMan {
	numBytes := (numPieces + 7) / 8
	return &BitfieldMan{
		bitfield:   make([]byte, numBytes),
		numPieces:  numPieces,
		cachedOnes: 0,
	}
}

// SetBit sets the bit at index
func (b *BitfieldMan) SetBit(index int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if index < 0 || index >= b.numPieces {
		return false
	}

	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	mask := byte(1 << bitIndex)

	if (b.bitfield[byteIndex] & mask) == 0 {
		b.bitfield[byteIndex] |= mask
		b.cachedOnes++
		return true
	}
	return false
}

// UnsetBit unsets the bit at index
func (b *BitfieldMan) UnsetBit(index int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if index < 0 || index >= b.numPieces {
		return false
	}

	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	mask := byte(1 << bitIndex)

	if (b.bitfield[byteIndex] & mask) != 0 {
		b.bitfield[byteIndex] &= ^mask
		b.cachedOnes--
		return true
	}
	return false
}

// HasBit checks if the bit at index is set
func (b *BitfieldMan) HasBit(index int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if index < 0 || index >= b.numPieces {
		return false
	}

	byteIndex := index / 8
	bitIndex := 7 - (index % 8)
	return (b.bitfield[byteIndex] & (1 << bitIndex)) != 0
}

// GetFirstMissingBit returns the index of the first unset bit
func (b *BitfieldMan) GetFirstMissingBit(start int) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for i := start; i < b.numPieces; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		if (b.bitfield[byteIndex] & (1 << bitIndex)) == 0 {
			return i
		}
	}
	return -1
}

// GetFirstSetBit returns the index of the first set bit
func (b *BitfieldMan) GetFirstSetBit(start int) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for i := start; i < b.numPieces; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		if (b.bitfield[byteIndex] & (1 << bitIndex)) != 0 {
			return i
		}
	}
	return -1
}

// CountSetBit returns the number of set bits
func (b *BitfieldMan) CountSetBit() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cachedOnes
}

// CountMissingBit returns the number of unset bits
func (b *BitfieldMan) CountMissingBit() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.numPieces - b.cachedOnes
}

// IsAllBitSet returns true if all bits are set
func (b *BitfieldMan) IsAllBitSet() bool {
	return b.CountSetBit() == b.numPieces
}

// Clear clears all bits
func (b *BitfieldMan) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.bitfield {
		b.bitfield[i] = 0
	}
	b.cachedOnes = 0
}

// SetAll sets all bits
func (b *BitfieldMan) SetAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := 0; i < b.numPieces; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		b.bitfield[byteIndex] |= (1 << bitIndex)
	}
	b.cachedOnes = b.numPieces
}

// String returns a hex string representation of the bitfield
func (b *BitfieldMan) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var sb strings.Builder
	for i := 0; i < len(b.bitfield); i++ {
		fmt.Fprintf(&sb, "%02x", b.bitfield[i])
	}
	// Note: this may print more bits than numPieces due to byte alignment
	return sb.String()
}

// FromHexString restores the bitfield from a hex string
func (b *BitfieldMan) FromHexString(hexStr string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(hexStr)%2 != 0 {
		return fmt.Errorf("invalid hex string length")
	}

	byteLen := len(hexStr) / 2
	if byteLen != len(b.bitfield) {
		// Strict check: length must match exactly
		return fmt.Errorf("hex string length mismatch: got %d bytes, expected %d", byteLen, len(b.bitfield))
	}

	b.cachedOnes = 0
	for i := 0; i < byteLen; i++ {
		var val byte
		_, err := fmt.Sscanf(hexStr[i*2:i*2+2], "%02x", &val)
		if err != nil {
			return fmt.Errorf("invalid hex at position %d: %v", i*2, err)
		}
		b.bitfield[i] = val

		// Recalculate cachedOnes
		// We can use a loop or lookup table. Simple loop for now.
		for j := 0; j < 8; j++ {
			if (val & (1 << j)) != 0 {
				// Only count if within numPieces range
				bitIdx := i*8 + (7 - j)
				if bitIdx < b.numPieces {
					b.cachedOnes++
				}
			}
		}
	}
	return nil
}

// ToBinaryString returns a binary string representation
func (b *BitfieldMan) ToBinaryString() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var sb strings.Builder
	for i := 0; i < b.numPieces; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		if (b.bitfield[byteIndex] & (1 << bitIndex)) != 0 {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}
