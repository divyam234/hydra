package engine

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"

	internalhttp "github.com/divyam234/hydra/internal/http"
	"github.com/divyam234/hydra/internal/ui"
	"github.com/divyam234/hydra/pkg/option"
)

// EventType represents the type of download event
type EventType int

const (
	EventComplete EventType = iota
	EventError
	EventPause
	EventResume
	EventCancel
	EventStart
)

// Event represents a download event
type Event struct {
	Type       EventType
	GID        GID
	Error      error
	Downloaded int64 // Bytes downloaded so far
	Total      int64 // Total bytes
	Speed      int   // Current speed in bytes/sec
}

// EventCallback is a function called when events occur
type EventCallback func(Event)

// DownloadEngine manages the lifecycle of downloads
type DownloadEngine struct {
	mu            sync.RWMutex
	options       *option.Option
	requestGroups map[GID]*RequestGroup
	gidGen        *GidGenerator
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	ui            ui.UserInterface

	// Shared resources
	sharedTransport *http.Transport

	// Queue management
	maxConcurrent int             // 0 = unlimited
	activeCount   int             // Current active downloads
	pendingQueue  []*RequestGroup // Pending downloads sorted by priority
	queueMu       sync.Mutex
	queueCond     *sync.Cond

	// Session management
	sessionManager *SessionManager

	// Event hooks
	eventCallback EventCallback
}

// EngineOption configures the engine
type EngineOption func(*DownloadEngine)

// WithMaxConcurrent sets the maximum number of concurrent downloads
func WithMaxConcurrent(n int) EngineOption {
	return func(e *DownloadEngine) {
		e.maxConcurrent = n
	}
}

// WithSessionFile sets the session file path for persistence
func WithSessionFile(path string) EngineOption {
	return func(e *DownloadEngine) {
		e.sessionManager = NewSessionManager(path)
	}
}

// WithEventCallback sets the event callback
func WithEventCallback(cb EventCallback) EngineOption {
	return func(e *DownloadEngine) {
		e.eventCallback = cb
	}
}

// NewDownloadEngine creates a new DownloadEngine
func NewDownloadEngine(opt *option.Option, opts ...EngineOption) *DownloadEngine {
	ctx, cancel := context.WithCancel(context.Background())
	e := &DownloadEngine{
		options:         opt,
		requestGroups:   make(map[GID]*RequestGroup),
		gidGen:          NewGidGenerator(),
		ctx:             ctx,
		cancel:          cancel,
		pendingQueue:    make([]*RequestGroup, 0),
		sharedTransport: internalhttp.NewTransport(opt),
	}
	e.queueCond = sync.NewCond(&e.queueMu)

	for _, o := range opts {
		o(e)
	}

	return e
}

// AddURI adds a new download from a URI
func (e *DownloadEngine) AddURI(uris []string, opt *option.Option) (GID, error) {
	return e.AddURIWithContext(context.Background(), uris, opt, nil)
}

// AddURIWithPriority adds a download with a specific priority (higher = runs first)
func (e *DownloadEngine) AddURIWithPriority(ctx context.Context, uris []string, opt *option.Option, customUI ui.UserInterface, priority int) (GID, error) {
	e.mu.Lock()

	gid, err := e.gidGen.Generate()
	if err != nil {
		e.mu.Unlock()
		return "", err
	}

	rg := NewRequestGroup(gid, uris, opt)
	rg.priority = priority

	// Use shared transport
	rg.SetHTTPTransport(e.sharedTransport)

	// Prioritize custom UI, fall back to engine UI
	if customUI != nil {
		rg.SetUI(customUI)
	} else if e.ui != nil {
		rg.SetUI(e.ui)
	}

	e.requestGroups[gid] = rg
	e.mu.Unlock()

	// Check if we can start immediately or need to queue
	e.queueMu.Lock()
	if e.maxConcurrent > 0 && e.activeCount >= e.maxConcurrent {
		// Add to pending queue
		e.pendingQueue = append(e.pendingQueue, rg)
		// Sort by priority (higher first)
		sort.Slice(e.pendingQueue, func(i, j int) bool {
			return e.pendingQueue[i].priority > e.pendingQueue[j].priority
		})
		e.queueMu.Unlock()
		return gid, nil
	}

	e.activeCount++
	e.queueMu.Unlock()

	// Start the download
	e.startDownload(ctx, rg)

	return gid, nil
}

// AddURIWithContext adds a new download with a custom context and optional UI
func (e *DownloadEngine) AddURIWithContext(ctx context.Context, uris []string, opt *option.Option, customUI ui.UserInterface) (GID, error) {
	return e.AddURIWithPriority(ctx, uris, opt, customUI, 0)
}

// startDownload starts a download in a goroutine
func (e *DownloadEngine) startDownload(ctx context.Context, rg *RequestGroup) {
	e.wg.Go(func() {
		defer e.onDownloadFinished(rg)

		childCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		// Monitor engine shutdown
		done := make(chan struct{})
		go func() {
			select {
			case <-e.ctx.Done():
				cancel()
			case <-childCtx.Done():
			case <-done:
			}
		}()

		// Fire start event
		e.fireEventWithProgress(EventStart, rg, nil)

		if err := rg.Execute(childCtx); err != nil {
			fmt.Printf("Download %s failed: %v\n", rg.gid, err)
			if rg.IsCancelled() {
				e.fireEventWithProgress(EventCancel, rg, nil)
			} else {
				e.fireEventWithProgress(EventError, rg, err)
			}
		} else {
			fmt.Printf("Download %s completed\n", rg.gid)
			e.fireEventWithProgress(EventComplete, rg, nil)
		}
		close(done)
	})
}

// onDownloadFinished is called when a download finishes to start next queued download
func (e *DownloadEngine) onDownloadFinished(rg *RequestGroup) {
	e.queueMu.Lock()

	e.activeCount--

	// Start next pending download if any
	if len(e.pendingQueue) > 0 && (e.maxConcurrent == 0 || e.activeCount < e.maxConcurrent) {
		next := e.pendingQueue[0]
		e.pendingQueue = e.pendingQueue[1:]
		e.activeCount++
		e.startDownload(context.Background(), next)
	}
	e.queueMu.Unlock()

	// Cleanup resources
	rg.Cleanup()

	// Save session if configured
	if e.sessionManager != nil {
		e.sessionManager.Save(e)
	}
}

// fireEvent calls the event callback if set
func (e *DownloadEngine) fireEvent(event Event) {
	if e.eventCallback != nil {
		e.eventCallback(event)
	}
}

// fireEventWithProgress fires an event with progress info from the request group
func (e *DownloadEngine) fireEventWithProgress(eventType EventType, rg *RequestGroup, err error) {
	if e.eventCallback == nil {
		return
	}

	status := rg.GetFullStatus()
	event := Event{
		Type:       eventType,
		GID:        rg.gid,
		Error:      err,
		Downloaded: status.Completed,
		Total:      status.Total,
		Speed:      status.Speed,
	}
	e.eventCallback(event)
}

// SetUI sets the user interface for all new downloads
func (e *DownloadEngine) SetUI(u ui.UserInterface) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ui = u
}

// GetUI returns the current user interface
func (e *DownloadEngine) GetUI() ui.UserInterface {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ui
}

// GetRequestGroup retrieves a request group by GID
func (e *DownloadEngine) GetRequestGroup(gid GID) *RequestGroup {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.requestGroups[gid]
}

// Shutdown gracefully shuts down the engine
func (e *DownloadEngine) Shutdown() {
	// Save session before shutdown
	if e.sessionManager != nil {
		e.sessionManager.Save(e)
	}

	e.cancel()
	e.wg.Wait()

	if e.sharedTransport != nil {
		e.sharedTransport.CloseIdleConnections()
	}
}

// Run waits for all downloads to complete and returns any errors encountered
func (e *DownloadEngine) Run() error {
	e.wg.Wait()

	// Check for any errors in request groups
	e.mu.RLock()
	defer e.mu.RUnlock()

	var errs []error
	for gid, rg := range e.requestGroups {
		status := rg.GetFullStatus()
		if status.Error != nil {
			errs = append(errs, fmt.Errorf("download %s failed: %w", gid, status.Error))
		}
	}

	if len(errs) > 0 {
		if len(errs) == 1 {
			return errs[0]
		}

		errMsg := fmt.Sprintf("%d downloads failed:", len(errs))
		for i, err := range errs {
			errMsg += fmt.Sprintf("\n%d. %v", i+1, err)
		}
		return fmt.Errorf("%s", errMsg)
	}

	return nil
}

// Pause pauses a download by GID
func (e *DownloadEngine) Pause(gid GID) bool {
	e.mu.RLock()
	rg, exists := e.requestGroups[gid]
	e.mu.RUnlock()

	if !exists {
		return false
	}
	if rg.Pause() {
		e.fireEventWithProgress(EventPause, rg, nil)
		return true
	}
	return false
}

// Resume resumes a paused download by GID
func (e *DownloadEngine) Resume(gid GID) bool {
	e.mu.RLock()
	rg, exists := e.requestGroups[gid]
	e.mu.RUnlock()

	if !exists {
		return false
	}
	if rg.Resume() {
		e.fireEventWithProgress(EventResume, rg, nil)
		return true
	}
	return false
}

// Cancel cancels a download by GID
func (e *DownloadEngine) Cancel(gid GID) bool {
	e.mu.RLock()
	rg, exists := e.requestGroups[gid]
	e.mu.RUnlock()

	if !exists {
		return false
	}
	return rg.Cancel()
}

// SaveSession saves the current session to disk
func (e *DownloadEngine) SaveSession() error {
	if e.sessionManager == nil {
		return fmt.Errorf("no session file configured")
	}
	return e.sessionManager.Save(e)
}

// LoadSession loads a session from disk and restores downloads
func (e *DownloadEngine) LoadSession() error {
	if e.sessionManager == nil {
		return fmt.Errorf("no session file configured")
	}

	session, err := e.sessionManager.Load()
	if err != nil {
		return err
	}

	for _, entry := range session.Downloads {
		opt := option.NewOption()
		opt.FromMap(entry.Options)

		// Restore the download
		e.mu.Lock()
		rg := NewRequestGroup(entry.GID, entry.URIs, opt)
		rg.priority = entry.Priority
		if e.ui != nil {
			rg.SetUI(e.ui)
		}
		e.requestGroups[entry.GID] = rg
		e.mu.Unlock()

		// Start or queue based on previous state
		if entry.State == RGStateActive || entry.State == RGStatePending {
			e.queueMu.Lock()
			if e.maxConcurrent > 0 && e.activeCount >= e.maxConcurrent {
				e.pendingQueue = append(e.pendingQueue, rg)
			} else {
				e.activeCount++
				e.startDownload(context.Background(), rg)
			}
			e.queueMu.Unlock()
		} else if entry.State == RGStatePaused {
			// Add to request groups but don't start
			rg.state.Store(RGStatePaused)
		}
	}

	return nil
}

// GetActiveCount returns the number of active downloads
func (e *DownloadEngine) GetActiveCount() int {
	e.queueMu.Lock()
	defer e.queueMu.Unlock()
	return e.activeCount
}

// GetPendingCount returns the number of pending downloads
func (e *DownloadEngine) GetPendingCount() int {
	e.queueMu.Lock()
	defer e.queueMu.Unlock()
	return len(e.pendingQueue)
}

// SetMaxConcurrent changes the max concurrent downloads limit
func (e *DownloadEngine) SetMaxConcurrent(n int) {
	e.queueMu.Lock()
	defer e.queueMu.Unlock()

	oldMax := e.maxConcurrent
	e.maxConcurrent = n

	// If we increased the limit, start more downloads
	if n > oldMax || n == 0 {
		for len(e.pendingQueue) > 0 && (n == 0 || e.activeCount < n) {
			next := e.pendingQueue[0]
			e.pendingQueue = e.pendingQueue[1:]
			e.activeCount++
			e.startDownload(context.Background(), next)
		}
	}
}

// GetQueuePosition returns the position of a download in the pending queue.
// Returns -1 if the download is not in the queue (either active, completed, or not found).
// Position 0 means it's next to be started.
func (e *DownloadEngine) GetQueuePosition(gid GID) int {
	e.queueMu.Lock()
	defer e.queueMu.Unlock()

	for i, rg := range e.pendingQueue {
		if rg.gid == gid {
			return i
		}
	}
	return -1
}

// GetQueuedDownloads returns the GIDs of all pending downloads in queue order
func (e *DownloadEngine) GetQueuedDownloads() []GID {
	e.queueMu.Lock()
	defer e.queueMu.Unlock()

	gids := make([]GID, len(e.pendingQueue))
	for i, rg := range e.pendingQueue {
		gids[i] = rg.gid
	}
	return gids
}
