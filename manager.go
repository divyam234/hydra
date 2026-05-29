package hydra

import (
	"context"
	"errors"
	"sync"
	"time"
)

// JobID identifies a queued download in Manager.
type JobID string

// JobResult is returned by Manager.Wait/WaitAll.
type JobResult struct {
	ID     JobID
	Result Result
	Err    error
}

type job struct {
	id     JobID
	req    Request
	result chan JobResult
}

// Manager is a small production-oriented queue around Downloader. It is meant
// for applications that need an importable safe engine rather than only a
// one-shot function.
type Manager struct {
	downloader *Downloader
	jobs       chan job
	closed     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
	results    map[JobID]chan JobResult
	cancel     context.CancelFunc
}

func NewManager(ctx context.Context, options Options) (*Manager, error) {
	d, err := New(options)
	if err != nil {
		return nil, err
	}
	opts := d.opts
	if opts.MaxConcurrentDownloads <= 0 {
		opts.MaxConcurrentDownloads = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	m := &Manager{
		downloader: d,
		jobs:       make(chan job),
		closed:     make(chan struct{}),
		results:    make(map[JobID]chan JobResult),
		cancel:     cancel,
	}
	for i := 0; i < opts.MaxConcurrentDownloads; i++ {
		m.wg.Add(1)
		go m.worker(ctx)
	}
	return m, nil
}

func (m *Manager) Enqueue(req Request) (JobID, error) {
	if req.ID == "" {
		req.ID = newJobID()
	}
	id := JobID(req.ID)
	ch := make(chan JobResult, 1)
	m.mu.Lock()
	if _, exists := m.results[id]; exists {
		m.mu.Unlock()
		return "", errors.New("duplicate job id: " + string(id))
	}
	m.results[id] = ch
	m.mu.Unlock()

	j := job{id: id, req: req, result: ch}
	select {
	case <-m.closed:
		return "", errors.New("manager is closed")
	case m.jobs <- j:
		m.downloader.emitEvent(Event{Kind: EventQueued, ID: string(id), Time: time.Now()})
		return id, nil
	}
}

func (m *Manager) Wait(ctx context.Context, id JobID) (Result, error) {
	m.mu.Lock()
	ch := m.results[id]
	m.mu.Unlock()
	if ch == nil {
		return Result{}, errors.New("unknown job id: " + string(id))
	}
	select {
	case rr := <-ch:
		return rr.Result, rr.Err
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

func (m *Manager) WaitAll(ctx context.Context, ids ...JobID) []JobResult {
	out := make([]JobResult, 0, len(ids))
	for _, id := range ids {
		res, err := m.Wait(ctx, id)
		out = append(out, JobResult{ID: id, Result: res, Err: err})
	}
	return out
}

func (m *Manager) Close() error {
	select {
	case <-m.closed:
		return nil
	default:
		close(m.closed)
		m.cancel()
		close(m.jobs)
		m.wg.Wait()
		return nil
	}
}

func (m *Manager) worker(ctx context.Context) {
	defer m.wg.Done()
	for j := range m.jobs {
		res, err := m.downloader.Download(ctx, j.req)
		jr := JobResult{ID: j.id, Result: res, Err: err}
		j.result <- jr
		close(j.result)
	}
}

func newJobID() string {
	return time.Now().UTC().Format("20060102T150405.000000000")
}
