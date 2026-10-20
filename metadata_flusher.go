package hydra

import (
	"sync"
	"time"
)

type metadataFlusher struct {
	meta     *metaFile
	path     string
	interval time.Duration
	wake     chan struct{}
	stop     chan struct{}
	done     chan struct{}

	mu       sync.Mutex
	err      error
	syncData func() error
}

func startMetadataFlusher(meta *metaFile, path string, interval time.Duration, syncData ...func() error) *metadataFlusher {
	f := &metadataFlusher{
		meta: meta, path: path, interval: interval,
		wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
	}
	if len(syncData) > 0 {
		f.syncData = syncData[0]
	}
	go f.run()
	return f
}

func (f *metadataFlusher) request() {
	select {
	case f.wake <- struct{}{}:
	default:
	}
}

func (f *metadataFlusher) run() {
	defer close(f.done)
	var tick <-chan time.Time
	if f.interval > 0 {
		ticker := time.NewTicker(f.interval)
		defer ticker.Stop()
		tick = ticker.C
	}
	for {
		select {
		case <-f.wake:
			f.flushIfDue()
		case <-tick:
			f.flushIfDue()
		case <-f.stop:
			return
		}
	}
}

func (f *metadataFlusher) flushIfDue() {
	if err := f.meta.saveIfDueAfterSync(f.path, f.interval, f.syncData); err != nil {
		f.mu.Lock()
		if f.err == nil {
			f.err = err
		}
		f.mu.Unlock()
	}
}

func (f *metadataFlusher) close() error {
	close(f.stop)
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}
