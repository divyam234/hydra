package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/bhunter/aria2go/pkg/option"
)

// DownloadEngine manages the lifecycle of downloads
type DownloadEngine struct {
	mu            sync.RWMutex
	options       *option.Option
	requestGroups map[GID]*RequestGroup
	gidGen        *GidGenerator
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewDownloadEngine creates a new DownloadEngine
func NewDownloadEngine(opt *option.Option) *DownloadEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &DownloadEngine{
		options:       opt,
		requestGroups: make(map[GID]*RequestGroup),
		gidGen:        NewGidGenerator(),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// AddURI adds a new download from a URI
func (e *DownloadEngine) AddURI(uris []string, opt *option.Option) (GID, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	gid, err := e.gidGen.Generate()
	if err != nil {
		return "", err
	}

	rg := NewRequestGroup(gid, uris, opt)
	e.requestGroups[gid] = rg

	// Start the download in a goroutine
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		if err := rg.Execute(e.ctx); err != nil {
			fmt.Printf("Download %s failed: %v\n", gid, err)
		} else {
			fmt.Printf("Download %s completed\n", gid)
		}
	}()

	return gid, nil
}

// Shutdown gracefully shuts down the engine
func (e *DownloadEngine) Shutdown() {
	e.cancel()
	e.wg.Wait()
}

// Run waits for all downloads to complete (for CLI usage)
func (e *DownloadEngine) Run() {
	// Simple run implementation for now - wait for completion
	// In real implementation, this would handle signals and event loop
	e.wg.Wait()
}
