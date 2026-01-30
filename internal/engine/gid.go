package engine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// GID represents a Global ID for downloads
type GID string

// GidGenerator generates unique Global IDs
type GidGenerator struct {
	mu sync.Mutex
}

// NewGidGenerator creates a new GID generator
func NewGidGenerator() *GidGenerator {
	return &GidGenerator{}
}

// Generate creates a new 16-character hex GID
func (g *GidGenerator) Generate() (GID, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	return GID(hex.EncodeToString(bytes)), nil
}

// String returns the string representation of the GID
func (g GID) String() string {
	return string(g)
}
