package hydra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Downloader is safe to use for multiple sequential downloads. Use Manager for
// concurrent queues or create separate Downloaders when you want fully
// independent transports/proxy settings.
type Downloader struct {
	opts    Options
	client  *http.Client
	bufPool sync.Pool
}

type Result struct {
	ID             string
	Path           string
	URL            string
	Size           int64
	Resumed        bool
	Skipped        bool
	Verified       bool
	Checksum       Checksum
	ChecksumActual string
	Connections    int
	StartedAt      time.Time
	FinishedAt     time.Time
	Duration       time.Duration
	AvgSpeed       float64
}

func New(options Options) (*Downloader, error) {
	opts, err := options.normalized()
	if err != nil {
		return nil, err
	}
	client, err := newHTTPClient(opts)
	if err != nil {
		return nil, err
	}
	d := &Downloader{opts: opts, client: client}
	d.bufPool.New = func() any { return make([]byte, opts.BufferSize) }
	return d, nil
}

func Download(ctx context.Context, req Request, options Options) (Result, error) {
	d, err := New(options)
	if err != nil {
		return Result{}, err
	}
	return d.Download(ctx, req)
}

func (d *Downloader) Download(ctx context.Context, req Request) (Result, error) {
	started := time.Now()
	req, err := req.normalized()
	if err != nil {
		return Result{}, err
	}
	if req.ID == "" {
		req.ID = newJobID()
	}
	policy := d.opts.ExistingFile
	if req.ExistingFile != "" {
		policy = req.ExistingFile
	}
	headers := cloneHeader(d.opts.Headers)
	mergeHeader(headers, req.Headers)

	d.emitEvent(Event{Kind: EventStarted, ID: req.ID, URL: req.URLs[0], Time: started})
	if d.opts.OnProgress != nil {
		d.opts.OnProgress(Progress{ID: req.ID, State: StateProbing, StartedAt: started, UpdatedAt: time.Now()})
	}
	d.emitEvent(Event{Kind: EventProbing, ID: req.ID, URL: req.URLs[0], Time: time.Now()})

	info, err := d.probe(ctx, req.URLs, headers)
	if err != nil {
		d.failEvent(req.ID, req.URLs[0], "", started, err)
		return Result{}, err
	}
	if req.ExpectedSize > 0 && info.Size > 0 && req.ExpectedSize != info.Size {
		err := fmt.Errorf("unexpected size: expected %d got %d", req.ExpectedSize, info.Size)
		d.failEvent(req.ID, info.URL, "", started, err)
		return Result{}, err
	}

	outName := req.Out
	if outName == "" {
		outName = info.Filename
	}
	finalPath := filepath.Join(req.Dir, outName)
	if err := ensureParent(finalPath); err != nil {
		d.failEvent(req.ID, info.URL, finalPath, started, err)
		return Result{}, err
	}

	var lock *fileLock
	if !d.opts.DisableFileLock {
		lock, err = acquireFileLock(finalPath)
		if err != nil {
			d.failEvent(req.ID, info.URL, finalPath, started, err)
			return Result{}, err
		}
		defer lock.Release()
	}

	if res, handled, err := d.handleExisting(ctx, req, info, finalPath, policy, started); handled || err != nil {
		if err != nil {
			d.failEvent(req.ID, info.URL, finalPath, started, err)
			return Result{}, err
		}
		return res, nil
	}

	useRanges := info.Size > 0 && info.AcceptRanges
	connections := 1
	if useRanges && d.opts.Split > 1 && info.Size >= 2*d.opts.MinSplitSize {
		connections = min(d.opts.Split, d.opts.MaxConnectionsPerServer)
		if int64(connections) > info.Size {
			connections = int(info.Size)
		}
	}

	var res Result
	if connections > 1 {
		res, err = d.downloadSegmented(ctx, req, headers, info, finalPath, connections, started)
	} else {
		res, err = d.downloadSingle(ctx, req, headers, info, finalPath, useRanges, started)
	}
	if err != nil {
		state := StateFailed
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			state = StateCancelled
			d.emitEvent(Event{Kind: EventCancelled, ID: req.ID, URL: info.URL, Path: finalPath, Error: err.Error(), Time: time.Now()})
		} else {
			d.failEvent(req.ID, info.URL, finalPath, started, err)
		}
		if d.opts.OnProgress != nil {
			d.opts.OnProgress(Progress{ID: req.ID, State: state, Path: finalPath, Total: info.Size, StartedAt: started, UpdatedAt: time.Now(), Error: err.Error()})
		}
		return Result{}, err
	}
	if !req.Checksum.Empty() {
		d.emitEvent(Event{Kind: EventVerifying, ID: req.ID, URL: info.URL, Path: res.Path, Time: time.Now()})
		if d.opts.OnProgress != nil {
			d.opts.OnProgress(Progress{ID: req.ID, State: StateVerifying, Path: res.Path, Total: res.Size, Completed: res.Size, StartedAt: started, UpdatedAt: time.Now()})
		}
		actual, err := req.Checksum.VerifyFile(res.Path)
		res.Checksum = req.Checksum.normalized()
		res.ChecksumActual = actual
		if err != nil {
			d.failEvent(req.ID, info.URL, res.Path, started, err)
			return Result{}, err
		}
		res.Verified = true
	}
	res.FinishedAt = time.Now()
	res.Duration = res.FinishedAt.Sub(started)
	if res.Duration > 0 && res.Size > 0 {
		res.AvgSpeed = float64(res.Size) / res.Duration.Seconds()
	}
	prog := Progress{ID: req.ID, State: StateComplete, Path: res.Path, Total: res.Size, Completed: res.Size, Connections: res.Connections, StartedAt: started, UpdatedAt: res.FinishedAt, AvgSpeed: res.AvgSpeed}
	if d.opts.OnProgress != nil {
		d.opts.OnProgress(prog)
	}
	d.emitEvent(Event{Kind: EventCompleted, ID: req.ID, URL: res.URL, Path: res.Path, Total: res.Size, Completed: res.Size, Time: res.FinishedAt, Progress: prog})
	return res, nil
}

func (d *Downloader) handleExisting(ctx context.Context, req Request, info probeInfo, finalPath string, policy ExistingFilePolicy, started time.Time) (Result, bool, error) {
	_ = ctx
	st, statErr := os.Stat(finalPath)
	exists := statErr == nil && !st.IsDir()
	if statErr != nil && !os.IsNotExist(statErr) {
		return Result{}, false, statErr
	}
	if policy == ExistingFileOverwrite {
		_ = removeIfExists(finalPath)
		_ = removeIfExists(partPath(finalPath))
		_ = removeIfExists(sidecarPath(finalPath))
		return Result{}, false, nil
	}
	if !exists {
		return Result{}, false, nil
	}
	if policy == ExistingFileError {
		return Result{}, true, fmt.Errorf("target exists: %s", finalPath)
	}
	if policy == ExistingFileSkip || (policy == ExistingFileResume && !hasResumeState(finalPath)) {
		res := Result{ID: req.ID, Path: finalPath, URL: info.URL, Size: st.Size(), Skipped: true, Connections: 0, StartedAt: started, FinishedAt: time.Now()}
		if !req.Checksum.Empty() {
			actual, err := req.Checksum.VerifyFile(finalPath)
			res.Checksum = req.Checksum.normalized()
			res.ChecksumActual = actual
			if err != nil {
				return Result{}, true, err
			}
			res.Verified = true
		}
		if req.ExpectedSize > 0 && st.Size() != req.ExpectedSize {
			return Result{}, true, fmt.Errorf("existing file size mismatch: expected %d got %d", req.ExpectedSize, st.Size())
		}
		if info.Size > 0 && st.Size() == info.Size || info.Size <= 0 {
			res.Duration = res.FinishedAt.Sub(started)
			prog := Progress{ID: req.ID, State: StateSkipped, Path: finalPath, Total: res.Size, Completed: res.Size, StartedAt: started, UpdatedAt: res.FinishedAt}
			if d.opts.OnProgress != nil {
				d.opts.OnProgress(prog)
			}
			d.emitEvent(Event{Kind: EventSkipped, ID: req.ID, URL: info.URL, Path: finalPath, Total: res.Size, Completed: res.Size, Time: res.FinishedAt, Progress: prog})
			return res, true, nil
		}
		return Result{}, true, fmt.Errorf("target exists with different size: %s", finalPath)
	}
	return Result{}, false, nil
}

func hasResumeState(finalPath string) bool {
	if _, err := os.Stat(sidecarPath(finalPath)); err == nil {
		return true
	}
	if _, err := os.Stat(partPath(finalPath)); err == nil {
		return true
	}
	return false
}

func (d *Downloader) downloadSegmented(ctx context.Context, req Request, headers http.Header, info probeInfo, finalPath string, connections int, started time.Time) (Result, error) {
	sidecar := sidecarPath(finalPath)
	resumed := false
	var meta *metaFile
	if !d.opts.DisableResume {
		if loaded, err := loadMeta(sidecar); err == nil && loaded.compatible(finalPath, info, d.opts.StrictResumeValidation) {
			meta = loaded
			resumed = loaded.completedBytes() > 0
		}
	}
	if meta == nil {
		meta = newMeta(req.URLs, finalPath, info, d.opts.ResumeBlockSize)
		if err := meta.save(sidecar); err != nil {
			return Result{}, err
		}
	}

	f, err := os.OpenFile(finalPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	if err := f.Truncate(info.Size); err != nil {
		return Result{}, err
	}

	tracker := newProgressTracker(req.ID, info.Size, connections, int64(meta.piecesTotal()), finalPath, d.opts.ProgressInterval, d.opts.OnProgress)
	tracker.setCompleted(meta.completedBytes())
	tracker.piecesDone.Store(int64(meta.donePieces()))
	tracker.emit(StateDownloading, "", true)
	d.emitEvent(Event{Kind: EventDownloading, ID: req.ID, URL: info.URL, Path: finalPath, Time: time.Now()})

	jobs := make(chan piece)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for p := range jobs {
				tracker.incActive()
				err := d.downloadPieceWithRetry(ctx, req.ID, req.URLs, headers, f, p, meta, sidecar, tracker)
				tracker.decActive()
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
				// Piece counters are driven by resume-block bit flips during writes.
				// Sync to metadata here, but never increment blindly on HTTP range-job completion.
				tracker.piecesDone.Store(int64(meta.donePieces()))
				d.emitEvent(Event{Kind: EventPieceDone, ID: req.ID, URL: info.URL, Path: finalPath, PieceIndex: p.Index, Total: info.Size, Completed: tracker.completed.Load(), Time: time.Now(), Progress: tracker.snapshot(StateDownloading, "", time.Now())})
			}
		}(i)
	}

enqueue:
	for _, p := range meta.pendingPieces(d.opts.Split, d.opts.MinSplitSize) {
		select {
		case <-ctx.Done():
			break enqueue
		case jobs <- p:
		}
	}
	close(jobs)
	wg.Wait()

	if err := meta.save(sidecar); err != nil {
		return Result{}, err
	}
	if err := f.Sync(); err != nil {
		return Result{}, err
	}

	select {
	case err := <-errCh:
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			tracker.emit(StateCancelled, err.Error(), true)
		}
		return Result{}, err
	default:
	}
	if err := ctx.Err(); err != nil {
		tracker.emit(StateCancelled, err.Error(), true)
		return Result{}, err
	}
	if !d.opts.KeepPartFile {
		_ = removeIfExists(sidecar)
	}
	return Result{ID: req.ID, Path: finalPath, URL: info.URL, Size: info.Size, Resumed: resumed, Connections: connections, StartedAt: started}, nil
}

func (d *Downloader) downloadSingle(ctx context.Context, req Request, headers http.Header, info probeInfo, finalPath string, canResume bool, started time.Time) (Result, error) {
	tmp := partPath(finalPath)
	offset := int64(0)
	resumed := false
	flag := os.O_CREATE | os.O_WRONLY
	if !d.opts.DisableResume && canResume {
		if st, err := os.Stat(tmp); err == nil && st.Size() > 0 && (info.Size <= 0 || st.Size() < info.Size) {
			offset = st.Size()
			resumed = true
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(tmp, flag, 0o644)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()

	tracker := newProgressTracker(req.ID, info.Size, 1, 1, finalPath, d.opts.ProgressInterval, d.opts.OnProgress)
	tracker.setCompleted(offset)
	tracker.incActive()
	d.emitEvent(Event{Kind: EventDownloading, ID: req.ID, URL: info.URL, Path: finalPath, Time: time.Now()})
	err = d.getToWriterWithRetry(ctx, req.ID, req.URLs, headers, f, offset, info.Size, tracker)
	tracker.decActive()
	if err != nil {
		_ = f.Sync()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			tracker.emit(StateCancelled, err.Error(), true)
		}
		return Result{}, err
	}
	if info.Size > 0 {
		if st, err := f.Stat(); err == nil && st.Size() != info.Size {
			return Result{}, fmt.Errorf("downloaded size mismatch: expected %d got %d", info.Size, st.Size())
		}
	}
	if err := f.Sync(); err != nil {
		return Result{}, err
	}
	if err := f.Close(); err != nil {
		return Result{}, err
	}
	if err := os.Rename(tmp, finalPath); err != nil {
		return Result{}, err
	}
	if !d.opts.KeepPartFile {
		_ = removeIfExists(tmp)
	}
	size := info.Size
	if size <= 0 {
		if st, err := os.Stat(finalPath); err == nil {
			size = st.Size()
		}
	}
	return Result{ID: req.ID, Path: finalPath, URL: info.URL, Size: size, Resumed: resumed, Connections: 1, StartedAt: started}, nil
}

func (d *Downloader) downloadPieceWithRetry(ctx context.Context, id string, urls []string, headers http.Header, f *os.File, p piece, meta *metaFile, sidecar string, tracker *progressTracker) error {
	var last error
	tries := max(1, d.opts.MaxRetries+1)
	done := int64(0)
	if p.size() <= 0 {
		return nil
	}

	for attempt := 0; attempt < tries && done < p.size(); attempt++ {
		for _, raw := range rotated(urls, p.Index+attempt) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			reqStart := p.Start + done
			markStart := reqStart
			if meta.BlockSize > 0 {
				markStart = (reqStart / meta.BlockSize) * meta.BlockSize
				if markStart < p.Start {
					markStart = p.Start
				}
			}
			n, err := d.downloadPiece(ctx, raw, headers, f, reqStart, p.End, tracker, func(writtenThisRequest int64) error {
				if writtenThisRequest <= 0 {
					return nil
				}
				changed, markErr := meta.markRangeComplete(markStart, reqStart+writtenThisRequest-1, sidecar)
				if markErr != nil {
					return markErr
				}
				if changed > 0 {
					tracker.addPiecesDone(int64(changed))
					return meta.saveIfDue(sidecar, d.opts.MetadataFlushInterval)
				}
				return nil
			})
			if n > 0 {
				done += n
			}
			if done == p.size() && err == nil {
				return nil
			}
			if err == nil {
				continue
			}
			last = err
			if !retryable(err) {
				return err
			}
			d.emitRetry(id, raw, p.Index, attempt+1, err, tracker)
		}
		if attempt+1 < tries {
			if err := sleepContext(ctx, d.retryDelay(attempt)); err != nil {
				return err
			}
		}
	}
	if last != nil {
		return last
	}
	return io.ErrUnexpectedEOF
}

func (d *Downloader) downloadPiece(ctx context.Context, raw string, headers http.Header, f *os.File, start, end int64, tracker *progressTracker, onWrite func(int64) error) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, err
	}
	d.applyHeaders(req, headers)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return 0, &HTTPStatusError{URL: raw, StatusCode: resp.StatusCode, Status: resp.Status}
	}
	buf := d.getBuffer()
	defer d.putBuffer(buf)
	return copyRangeToFile(f, resp.Body, start, end-start+1, buf, tracker, onWrite)
}

func (d *Downloader) getToWriterWithRetry(ctx context.Context, id string, urls []string, headers http.Header, w io.Writer, offset int64, total int64, tracker *progressTracker) error {
	var last error
	tries := max(1, d.opts.MaxRetries+1)
	current := offset
	for attempt := 0; attempt < tries; attempt++ {
		for _, raw := range rotated(urls, attempt) {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			n, err := d.getToWriter(ctx, raw, headers, w, current, total, tracker)
			current += n
			if err == nil {
				return nil
			}
			last = err
			if !retryable(err) {
				return err
			}
			d.emitRetry(id, raw, -1, attempt+1, err, tracker)
		}
		if attempt+1 < tries {
			if err := sleepContext(ctx, d.retryDelay(attempt)); err != nil {
				return err
			}
		}
	}
	return last
}

func (d *Downloader) getToWriter(ctx context.Context, raw string, headers http.Header, w io.Writer, offset int64, total int64, tracker *progressTracker) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, err
	}
	d.applyHeaders(req, headers)
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if offset > 0 && resp.StatusCode != http.StatusPartialContent {
		return 0, ErrServerNoRange
	}
	if offset == 0 && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return 0, &HTTPStatusError{URL: raw, StatusCode: resp.StatusCode, Status: resp.Status}
	}
	if offset > 0 && total > 0 && resp.StatusCode == http.StatusPartialContent {
		if cr := resp.Header.Get("Content-Range"); cr != "" && !strings.HasPrefix(cr, "bytes "+strconv.FormatInt(offset, 10)+"-") {
			return 0, fmt.Errorf("unexpected content-range: %s", cr)
		}
	}
	buf := d.getBuffer()
	defer d.putBuffer(buf)
	return copyWithProgress(w, resp.Body, buf, tracker)
}

func (d *Downloader) applyHeaders(req *http.Request, h http.Header) {
	req.Header.Set("User-Agent", d.opts.UserAgent)
	if d.opts.DisableCompression {
		req.Header.Set("Accept-Encoding", "identity")
	}
	for k, vals := range h {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
}

func (d *Downloader) getBuffer() []byte {
	v, ok := d.bufPool.Get().([]byte)
	if !ok || cap(v) < d.opts.BufferSize {
		return make([]byte, d.opts.BufferSize)
	}
	return v[:d.opts.BufferSize]
}

func (d *Downloader) putBuffer(buf []byte) {
	if cap(buf) < d.opts.BufferSize {
		return
	}
	d.bufPool.Put(buf[:d.opts.BufferSize])
}

func copyRangeToFile(f *os.File, r io.Reader, off int64, expected int64, buf []byte, tracker *progressTracker, onWrite func(int64) error) (int64, error) {
	var written int64
	for {
		n, er := r.Read(buf)
		if n > 0 {
			if written+int64(n) > expected {
				n = int(expected - written)
			}
			wn, ew := f.WriteAt(buf[:n], off+written)
			if wn > 0 {
				written += int64(wn)
				tracker.add(int64(wn))
				if onWrite != nil {
					// Pass cumulative bytes written for this range request. Passing only
					// the last buffer size prevents resume-block checkpoints from
					// advancing after the first buffer.
					if err := onWrite(written); err != nil {
						return written, err
					}
				}
			}
			if ew != nil {
				return written, ew
			}
			if wn != n {
				return written, io.ErrShortWrite
			}
			if written == expected {
				return written, nil
			}
		}
		if er != nil {
			if er == io.EOF {
				if written == expected {
					return written, nil
				}
				return written, io.ErrUnexpectedEOF
			}
			return written, er
		}
	}
}

func copyWithProgress(w io.Writer, r io.Reader, buf []byte, tracker *progressTracker) (int64, error) {
	var written int64
	for {
		n, er := r.Read(buf)
		if n > 0 {
			wn, ew := w.Write(buf[:n])
			if wn > 0 {
				written += int64(wn)
				tracker.add(int64(wn))
			}
			if ew != nil {
				return written, ew
			}
			if wn != n {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}

func retryable(err error) bool {
	if err == nil {
		return false
	}
	var se *HTTPStatusError
	if errors.As(err, &se) {
		if se.StatusCode == http.StatusTooManyRequests || se.StatusCode == http.StatusRequestTimeout {
			return true
		}
		if se.StatusCode >= 500 && se.StatusCode != http.StatusNotImplemented {
			return true
		}
		return false
	}
	if errors.Is(err, ErrServerNoRange) || errors.Is(err, ErrChecksumMismatch) {
		return false
	}
	return true
}

func (d *Downloader) retryDelay(attempt int) time.Duration {
	base := d.opts.RetryWait
	if base <= 0 {
		return 0
	}
	// Exponential backoff capped at MaxRetryWait; deterministic to keep tests stable.
	for i := 0; i < attempt; i++ {
		base *= 2
		if d.opts.MaxRetryWait > 0 && base > d.opts.MaxRetryWait {
			return d.opts.MaxRetryWait
		}
	}
	return base
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (d *Downloader) emitEvent(ev Event) {
	if d.opts.OnEvent == nil {
		return
	}
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	d.opts.OnEvent(ev)
}

func (d *Downloader) emitRetry(id, raw string, pieceIndex int, attempt int, err error, tracker *progressTracker) {
	tracker.incRetry()
	d.emitEvent(Event{Kind: EventRetrying, ID: id, URL: raw, Attempt: attempt, PieceIndex: pieceIndex, Error: err.Error(), Time: time.Now(), Progress: tracker.snapshot(StateDownloading, err.Error(), time.Now())})
}

func (d *Downloader) failEvent(id, url, path string, started time.Time, err error) {
	prog := Progress{ID: id, State: StateFailed, Path: path, StartedAt: started, UpdatedAt: time.Now(), Error: err.Error()}
	d.emitEvent(Event{Kind: EventFailed, ID: id, URL: url, Path: path, Error: err.Error(), Time: time.Now(), Progress: prog})
}

func cloneHeader(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vals := range h {
		out[k] = append([]string(nil), vals...)
	}
	return out
}
func mergeHeader(dst, src http.Header) {
	for k, vals := range src {
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}
func rotated[T any](in []T, start int) []T {
	if len(in) < 2 {
		return in
	}
	out := make([]T, 0, len(in))
	s := start % len(in)
	out = append(out, in[s:]...)
	out = append(out, in[:s]...)
	return out
}
