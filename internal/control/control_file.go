package control

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/divyam234/hydra/internal/segment"
)

// ControlFile represents the state of a download
type ControlFile struct {
	GID         string   `json:"gid"`
	TotalLength int64    `json:"total_length"`
	PieceLength int64    `json:"piece_length"`
	NumPieces   int      `json:"num_pieces"`
	Bitfield    string   `json:"bitfield"` // Hex string
	URIs        []string `json:"uris"`
	Path        string   `json:"path"` // Output file path
}

// Controller manages the control file
type Controller struct {
	filePath string
	mu       sync.Mutex
}

// NewController creates a new Controller
func NewController(downloadPath string) *Controller {
	return &Controller{
		filePath: downloadPath + ".hydra",
	}
}

// Exists checks if the control file exists
func (c *Controller) Exists() bool {
	_, err := os.Stat(c.filePath)
	return !os.IsNotExist(err)
}

// Save saves the download state
func (c *Controller) Save(gid string, ps segment.PieceStorage, uris []string, outPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cf := ControlFile{
		GID:         gid,
		TotalLength: ps.GetTotalLength(),
		PieceLength: ps.GetPieceLength(),
		NumPieces:   ps.GetNumPieces(),
		Bitfield:    ps.GetBitfield().String(),
		URIs:        uris,
		Path:        outPath,
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal control file: %w", err)
	}

	return os.WriteFile(c.filePath, data, 0666)
}

// Load loads the download state
func (c *Controller) Load() (*ControlFile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read control file: %w", err)
	}

	var cf ControlFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal control file: %w", err)
	}

	return &cf, nil
}

// Remove deletes the control file
func (c *Controller) Remove() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return os.Remove(c.filePath)
}
