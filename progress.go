package hydra

import (
	"sync"
	"sync/atomic"
	"time"
)

type State string

const (
	StateQueued      State = "queued"
	StateProbing     State = "probing"
	StateDownloading State = "downloading"
	StateVerifying   State = "verifying"
	StateMerging     State = "merging"
	StateComplete    State = "complete"
	StateSkipped     State = "skipped"
	StateFailed      State = "failed"
	StateCancelled   State = "cancelled"
)

// Progress is a point-in-time transfer snapshot suitable for terminal UIs,
// dashboards, JSON APIs, or metrics exporters.
type Progress struct {
	ID        string
	State     State
	Path      string
	Total     int64
	Completed int64
	// Resumed is the number of bytes that were already complete before this
	// process started. It is excluded from Speed and AvgSpeed.
	Resumed int64
	// Downloaded is the number of bytes downloaded by this process.
	Downloaded  int64
	Active      int64
	Connections int
	PiecesDone  int64
	PiecesTotal int64
	Retries     int64
	StartedAt   time.Time
	UpdatedAt   time.Time
	Speed       float64 // bytes/sec since previous emitted snapshot, excluding resumed bytes
	AvgSpeed    float64 // bytes/sec for bytes downloaded in this process, excluding resumed bytes
	ETA         time.Duration
	Error       string
}

type ProgressFunc func(Progress)

type progressTracker struct {
	id        string
	total     int64
	completed atomic.Int64
	// baseCompleted is the amount that was already present when this process
	// started/resumed. Speeds must not count those bytes, otherwise a resumed
	// download shows impossible initial speeds that slowly decay.
	baseCompleted atomic.Int64
	active        atomic.Int64
	connections   int
	path          string
	started       time.Time
	callback      ProgressFunc
	interval      time.Duration
	piecesDone    atomic.Int64
	piecesTotal   int64
	retries       atomic.Int64
	lastEmitNS    atomic.Int64
	lastBytes     atomic.Int64
	speedMu       sync.Mutex
	speedSamples  []speedSample
}

type speedSample struct {
	at    time.Time
	bytes int64
}

func newProgressTracker(id string, total int64, connections int, piecesTotal int64, path string, interval time.Duration, cb ProgressFunc) *progressTracker {
	if interval <= 0 {
		interval = DefaultProgressInterval
	}
	now := time.Now()
	p := &progressTracker{id: id, total: total, connections: connections, piecesTotal: piecesTotal, path: path, started: now, interval: interval, callback: cb}
	p.lastEmitNS.Store(now.UnixNano())
	return p
}

func (p *progressTracker) add(n int64) {
	if n <= 0 {
		return
	}
	p.completed.Add(n)
	p.emit(StateDownloading, "", false)
}

func (p *progressTracker) setCompleted(n int64) {
	if n < 0 {
		n = 0
	}
	now := time.Now()
	p.completed.Store(n)
	p.baseCompleted.Store(n)
	p.lastBytes.Store(n)
	p.lastEmitNS.Store(now.UnixNano())
	p.speedMu.Lock()
	p.speedSamples = []speedSample{{at: now, bytes: 0}}
	p.speedMu.Unlock()
}
func (p *progressTracker) incActive()    { p.active.Add(1); p.emit(StateDownloading, "", false) }
func (p *progressTracker) decActive()    { p.active.Add(-1); p.emit(StateDownloading, "", false) }
func (p *progressTracker) incPieceDone() { p.addPiecesDone(1) }
func (p *progressTracker) addPiecesDone(n int64) {
	if n <= 0 {
		return
	}
	p.piecesDone.Add(n)
	p.emit(StateDownloading, "", true)
}
func (p *progressTracker) incRetry() { p.retries.Add(1); p.emit(StateDownloading, "", true) }

func (p *progressTracker) snapshot(state State, errText string, now time.Time) Progress {
	completed := p.completed.Load()
	return p.snapshotWithSpeed(state, errText, now, p.rollingSpeed(now, completed))
}

func (p *progressTracker) snapshotWithSpeed(state State, errText string, now time.Time, inst float64) Progress {
	completed := p.completed.Load()
	base := p.baseCompleted.Load()
	if base < 0 || base > completed {
		base = 0
	}
	sessionBytes := completed - base
	elapsed := now.Sub(p.started)
	if elapsed < 0 {
		elapsed = 0
	}
	avg := 0.0
	if elapsed > 0 && sessionBytes > 0 {
		avg = float64(sessionBytes) / elapsed.Seconds()
	}
	eta := time.Duration(0)
	// ETA should use the recent rolling speed when available. AvgSpeed is useful
	// for final stats, but after resume it starts from zero and can take too long
	// to converge to the real ISP speed.
	etaSpeed := inst
	if etaSpeed <= 0 {
		etaSpeed = avg
	}
	if p.total > 0 && etaSpeed > 0 && completed < p.total {
		eta = time.Duration(float64(p.total-completed) / etaSpeed * float64(time.Second))
	}
	return Progress{
		ID: p.id, State: state, Path: p.path, Total: p.total, Completed: completed,
		Resumed: base, Downloaded: sessionBytes,
		Active: p.active.Load(), Connections: p.connections, PiecesDone: p.piecesDone.Load(),
		PiecesTotal: p.piecesTotal, Retries: p.retries.Load(), StartedAt: p.started,
		UpdatedAt: now, Speed: inst, AvgSpeed: avg, ETA: eta, Error: errText,
	}
}

func speedSince(lastNS int64, lastBytes int64, completed int64, now time.Time) float64 {
	if lastNS == 0 {
		return 0
	}
	delta := completed - lastBytes
	if delta <= 0 {
		return 0
	}
	dt := now.Sub(time.Unix(0, lastNS))
	if dt <= 0 {
		return 0
	}
	return float64(delta) / dt.Seconds()
}

func (p *progressTracker) rollingSpeed(now time.Time, completed int64) float64 {
	base := p.baseCompleted.Load()
	if base < 0 || base > completed {
		base = 0
	}
	sessionBytes := completed - base
	p.speedMu.Lock()
	defer p.speedMu.Unlock()
	if len(p.speedSamples) == 0 {
		p.speedSamples = append(p.speedSamples, speedSample{at: now, bytes: sessionBytes})
		return 0
	}
	last := p.speedSamples[len(p.speedSamples)-1]
	// Avoid thousands of samples per second while still recording forced final
	// updates and meaningful byte movement.
	if sessionBytes != last.bytes || now.Sub(last.at) >= 250*time.Millisecond {
		p.speedSamples = append(p.speedSamples, speedSample{at: now, bytes: sessionBytes})
	}
	cutoff := now.Add(-3 * time.Second)
	for len(p.speedSamples) > 2 && p.speedSamples[1].at.Before(cutoff) {
		p.speedSamples = p.speedSamples[1:]
	}
	first := p.speedSamples[0]
	last = p.speedSamples[len(p.speedSamples)-1]
	dt := last.at.Sub(first.at)
	if dt <= 0 || last.bytes <= first.bytes {
		return 0
	}
	return float64(last.bytes-first.bytes) / dt.Seconds()
}

func (p *progressTracker) emit(state State, errText string, force bool) Progress {
	now := time.Now()
	lastNS := p.lastEmitNS.Load()
	if !force && lastNS != 0 && now.Sub(time.Unix(0, lastNS)) < p.interval {
		// Do not mutate the sampling state when throttling. Mutating here was the
		// source of bogus progress speeds because bytes were consumed by snapshots
		// that were never emitted to the UI.
		return p.snapshot(state, errText, now)
	}
	completed := p.completed.Load()
	inst := p.rollingSpeed(now, completed)
	p.lastEmitNS.Store(now.UnixNano())
	p.lastBytes.Store(completed)
	prog := p.snapshotWithSpeed(state, errText, now, inst)
	if p.callback != nil {
		p.callback(prog)
	}
	return prog
}
