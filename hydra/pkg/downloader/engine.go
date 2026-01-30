package downloader

import (
	"context"
	"fmt"
	"time"

	"github.com/bhunter/hydra/internal/engine"
	"github.com/bhunter/hydra/internal/ui"
	"github.com/bhunter/hydra/pkg/option"
)

// Engine manages concurrent downloads
type Engine struct {
	internal *engine.DownloadEngine
	options  *option.Option
}

// NewEngine creates a new download engine
func NewEngine(opts ...Option) *Engine {
	cfg := &config{
		opt: option.GetDefaultOptions(),
	}
	for _, o := range opts {
		o(cfg)
	}

	// Build internal engine options
	var engineOpts []engine.EngineOption
	if cfg.maxConcurrent > 0 {
		engineOpts = append(engineOpts, engine.WithMaxConcurrent(cfg.maxConcurrent))
	}
	if cfg.sessionFile != "" {
		engineOpts = append(engineOpts, engine.WithSessionFile(cfg.sessionFile))
	}
	if cfg.eventCb != nil {
		engineOpts = append(engineOpts, engine.WithEventCallback(func(e engine.Event) {
			cfg.eventCb(Event{
				Type:       EventType(e.Type),
				ID:         DownloadID(e.GID),
				Error:      e.Error,
				Downloaded: e.Downloaded,
				Total:      e.Total,
				Speed:      int64(e.Speed),
			})
		}))
	}

	eng := engine.NewDownloadEngine(cfg.opt, engineOpts...)

	// If callbacks are set
	if cfg.progressCb != nil || cfg.messageCb != nil {
		eng.SetUI(&callbackUI{
			progressCb: cfg.progressCb,
			messageCb:  cfg.messageCb,
		})
	}

	return &Engine{
		internal: eng,
		options:  cfg.opt,
	}
}

// AddDownload starts a new download
func (e *Engine) AddDownload(ctx context.Context, urls []string, opts ...Option) (DownloadID, error) {
	// Clone engine options and apply specific ones
	cfg := &config{
		opt: e.options.Clone(),
	}

	for _, o := range opts {
		o(cfg)
	}

	var customUI ui.UserInterface
	if cfg.progressCb != nil || cfg.messageCb != nil {
		customUI = &callbackUI{
			progressCb: cfg.progressCb,
			messageCb:  cfg.messageCb,
		}
	}

	gid, err := e.internal.AddURIWithPriority(ctx, urls, cfg.opt, customUI, cfg.priority)
	if err != nil {
		return "", err
	}

	return DownloadID(gid), nil
}

// SetProgressCallback sets a global progress callback for the engine
func (e *Engine) SetProgressCallback(cb func(Progress)) {
	// Preserve existing message callback if present
	var msgCb func(string)
	if ui := e.internal.GetUI(); ui != nil {
		if existing, ok := ui.(*callbackUI); ok {
			msgCb = existing.messageCb
		}
	}

	e.internal.SetUI(&callbackUI{

		progressCb: cb,
		messageCb:  msgCb,
	})
}

// SetMessageCallback sets a global message callback for the engine
func (e *Engine) SetMessageCallback(cb func(string)) {
	// Preserve existing progress callback if present
	var progressCb func(Progress)
	if ui := e.internal.GetUI(); ui != nil {
		if existing, ok := ui.(*callbackUI); ok {
			progressCb = existing.progressCb
		}
	}

	e.internal.SetUI(&callbackUI{
		progressCb: progressCb,
		messageCb:  cb,
	})
}

// Wait waits for all downloads to complete
func (e *Engine) Wait() error {
	return e.internal.Run()
}

// Shutdown stops the engine
func (e *Engine) Shutdown() {
	e.internal.Shutdown()
}

// Pause pauses a download
func (e *Engine) Pause(id DownloadID) bool {
	return e.internal.Pause(engine.GID(id))
}

// Resume resumes a paused download
func (e *Engine) Resume(id DownloadID) bool {
	return e.internal.Resume(engine.GID(id))
}

// Cancel cancels a download
func (e *Engine) Cancel(id DownloadID) bool {
	return e.internal.Cancel(engine.GID(id))
}

// Status retrieves the status of a download
func (e *Engine) Status(id DownloadID) (*Status, error) {
	gid := engine.GID(id)
	rg := e.internal.GetRequestGroup(gid)
	if rg == nil {
		return nil, fmt.Errorf("download not found: %s", id)
	}

	ds := rg.GetFullStatus()

	var state State
	switch ds.State {
	case engine.RGStatePending:
		state = StatePending
	case engine.RGStateActive:
		state = StateActive
	case engine.RGStatePaused:
		state = StatePaused
	case engine.RGStateComplete:
		state = StateComplete
	case engine.RGStateError:
		state = StateError
	case engine.RGStateCancelled:
		state = StateCancelled
	}

	var duration time.Duration
	if !ds.EndTime.IsZero() {
		duration = ds.EndTime.Sub(ds.StartTime)
	} else if !ds.StartTime.IsZero() {
		duration = time.Since(ds.StartTime)
	}

	percent := 0.0
	if ds.Total > 0 {
		percent = float64(ds.Completed) / float64(ds.Total) * 100
	}

	return &Status{
		ID:    id,
		State: state,
		Progress: Progress{
			ID:         id,
			Downloaded: ds.Completed,
			Total:      ds.Total,
			Percent:    percent,
			Speed:      int64(ds.Speed),
		},
		Filename:         ds.OutputPath,
		Duration:         duration,
		Error:            ds.Error,
		ChecksumOK:       ds.ChecksumOK,
		ChecksumVerified: ds.ChecksumVerified,
	}, nil
}

// SaveSession saves the current session to disk
func (e *Engine) SaveSession() error {
	return e.internal.SaveSession()
}

// LoadSession loads a session from disk and restores downloads
func (e *Engine) LoadSession() error {
	return e.internal.LoadSession()
}

// GetActiveCount returns the number of active downloads
func (e *Engine) GetActiveCount() int {
	return e.internal.GetActiveCount()
}

// GetPendingCount returns the number of pending/queued downloads
func (e *Engine) GetPendingCount() int {
	return e.internal.GetPendingCount()
}

// SetMaxConcurrentDownloads changes the max concurrent downloads limit
func (e *Engine) SetMaxConcurrentDownloads(n int) {
	e.internal.SetMaxConcurrent(n)
}

// GetQueuePosition returns the position of a download in the pending queue.
// Returns -1 if the download is not in the queue (either active, completed, or not found).
// Position 0 means it's next to be started.
func (e *Engine) GetQueuePosition(id DownloadID) int {
	return e.internal.GetQueuePosition(engine.GID(id))
}

// GetQueuedDownloads returns the IDs of all pending downloads in queue order
func (e *Engine) GetQueuedDownloads() []DownloadID {
	gids := e.internal.GetQueuedDownloads()
	ids := make([]DownloadID, len(gids))
	for i, gid := range gids {
		ids[i] = DownloadID(gid)
	}
	return ids
}

// callbackUI adapts the UI interface to a callback
type callbackUI struct {
	progressCb func(Progress)
	messageCb  func(string)
}

func (c *callbackUI) PrintProgress(gid string, total, completed int64, speed int, numConns int) {
	if c.progressCb != nil {
		p := Progress{
			ID:          DownloadID(gid),
			Downloaded:  completed,
			Total:       total,
			Speed:       int64(speed),
			Connections: numConns,
		}
		if total > 0 {
			p.Percent = float64(completed) / float64(total) * 100
		}
		c.progressCb(p)
	}
}

func (c *callbackUI) ClearLine() {}

func (c *callbackUI) Printf(format string, a ...interface{}) {
	if c.messageCb != nil {
		c.messageCb(fmt.Sprintf(format, a...))
	}
}
