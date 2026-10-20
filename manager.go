package hydra

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
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
	result *jobResult
}

// Closing done publishes value to every waiter, including future Wait calls.
type jobResult struct {
	done  chan struct{}
	value JobResult
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
	results    map[JobID]*jobResult
	cancel     context.CancelFunc
	closeOnce  sync.Once
	ctx        context.Context
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
		results:    make(map[JobID]*jobResult),
		cancel:     cancel,
		ctx:        ctx,
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
	ch := &jobResult{done: make(chan struct{})}
	m.mu.Lock()
	if m.ctx.Err() != nil {
		m.mu.Unlock()
		return "", errors.New("manager is closed")
	}
	if _, exists := m.results[id]; exists {
		m.mu.Unlock()
		return "", errors.New("duplicate job id: " + string(id))
	}
	m.results[id] = ch
	m.mu.Unlock()

	j := job{id: id, req: req, result: ch}
	select {
	case <-m.ctx.Done():
		m.mu.Lock()
		delete(m.results, id)
		m.mu.Unlock()
		return "", errors.New("manager is closed")
	case m.jobs <- j:
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
	case <-ch.done:
		return ch.value.Result, ch.value.Err
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
	m.closeOnce.Do(func() {
		close(m.closed)
		m.cancel()
		m.wg.Wait()
		if m.downloader.opts.Transport == nil {
			m.downloader.CloseIdleConnections()
		}
	})
	return nil
}

func (m *Manager) worker(ctx context.Context) {
	defer m.wg.Done()
	for {
		var j job
		select {
		case <-ctx.Done():
			return
		case j = <-m.jobs:
		}
		m.downloader.emitEvent(Event{Kind: EventQueued, ID: string(j.id), Time: time.Now()})
		res, err := m.downloader.Download(ctx, j.req)
		jr := JobResult{ID: j.id, Result: res, Err: err}
		j.result.value = jr
		close(j.result.done)
	}
}

var jobSequence atomic.Uint64

func newJobID() string {
	// Clock timestamps alone collide under concurrent enqueue/download calls.
	var buf [64]byte
	id := time.Now().UTC().AppendFormat(buf[:0], "20060102T150405.000000000")
	id = append(id, '-')
	id = strconv.AppendUint(id, jobSequence.Add(1), 16)
	return string(id)
}
