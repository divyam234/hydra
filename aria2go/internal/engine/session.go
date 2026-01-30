package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SessionEntry represents a single download in the session
type SessionEntry struct {
	GID      GID               `json:"gid"`
	URIs     []string          `json:"uris"`
	Options  map[string]string `json:"options"`
	State    int32             `json:"state"`
	Priority int               `json:"priority"`
}

// Session represents the engine's saved state
type Session struct {
	Downloads []SessionEntry `json:"downloads"`
}

// SessionManager handles saving and loading engine state
type SessionManager struct {
	filePath string
	mu       sync.Mutex
}

// NewSessionManager creates a new SessionManager
func NewSessionManager(sessionFile string) *SessionManager {
	return &SessionManager{
		filePath: sessionFile,
	}
}

// Save saves the current engine state to disk
func (sm *SessionManager) Save(engine *DownloadEngine) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	engine.mu.RLock()
	defer engine.mu.RUnlock()

	session := Session{
		Downloads: make([]SessionEntry, 0, len(engine.requestGroups)),
	}

	for gid, rg := range engine.requestGroups {
		state := rg.state.Load()
		// Only save incomplete downloads (pending, active, paused)
		if state == RGStateComplete || state == RGStateCancelled {
			continue
		}

		entry := SessionEntry{
			GID:      gid,
			URIs:     rg.uris,
			Options:  rg.options.ToMap(),
			State:    state,
			Priority: rg.priority,
		}
		session.Downloads = append(session.Downloads, entry)
	}

	// Create directory if needed
	dir := filepath.Dir(sm.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create session directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	return os.WriteFile(sm.filePath, data, 0644)
}

// Load loads the session from disk
func (sm *SessionManager) Load() (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Session{}, nil
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// Remove deletes the session file
func (sm *SessionManager) Remove() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	err := os.Remove(sm.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Exists checks if a session file exists
func (sm *SessionManager) Exists() bool {
	_, err := os.Stat(sm.filePath)
	return !os.IsNotExist(err)
}
