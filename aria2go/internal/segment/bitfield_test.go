package segment

import (
	"testing"
)

func TestBitfieldMan_HexRoundTrip(t *testing.T) {
	numPieces := 100
	bf := NewBitfieldMan(numPieces)

	// Set some bits
	bf.SetBit(0)
	bf.SetBit(7)
	bf.SetBit(8)
	bf.SetBit(99)

	// Get Hex String
	hexStr := bf.String()

	// Create new Bitfield
	bf2 := NewBitfieldMan(numPieces)
	err := bf2.FromHexString(hexStr)
	if err != nil {
		t.Fatalf("FromHexString failed: %v", err)
	}

	// Verify bits
	if !bf2.HasBit(0) {
		t.Error("Bit 0 not set")
	}
	if !bf2.HasBit(7) {
		t.Error("Bit 7 not set")
	}
	if !bf2.HasBit(8) {
		t.Error("Bit 8 not set")
	}
	if !bf2.HasBit(99) {
		t.Error("Bit 99 not set")
	}
	if bf2.HasBit(1) {
		t.Error("Bit 1 set incorrectly")
	}

	// Verify count
	if bf2.CountSetBit() != 4 {
		t.Errorf("Expected 4 set bits, got %d", bf2.CountSetBit())
	}
}

func TestBitfieldMan_LengthValidation(t *testing.T) {
	bf := NewBitfieldMan(16) // 2 bytes -> 4 hex chars

	// Too short
	err := bf.FromHexString("FF")
	if err == nil {
		t.Error("Expected error for short hex string")
	}

	// Too long
	err = bf.FromHexString("FFFFFF")
	if err == nil {
		t.Error("Expected error for long hex string")
	}

	// Correct length
	err = bf.FromHexString("FFFF")
	if err != nil {
		t.Errorf("Unexpected error for correct length: %v", err)
	}
}
