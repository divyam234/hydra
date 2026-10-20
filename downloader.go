package hydra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
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
	mirrors *mirrorScorer
}

type Result struct {
	ID             string
	Path           string
	URL            string
	Size           int64
	Resumed        bool
	ResumedBytes   int64
	Downloaded     int64 // Bytes contributing to the output transferred in this invocation.
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
	d := &Downloader{opts: opts, client: client, mirrors: newMirrorScorer()}
	d.bufPool.New = func() any { return make([]byte, opts.BufferSize) }
	return d, nil
}

func Download(ctx context.Context, req Request, options Options) (Result, error) {
	d, err := New(options)
	if err != nil {
		return Result{}, err
	}
	if d.opts.Transport == nil {
		defer d.CloseIdleConnections()
	}
	return d.Download(ctx, req)
}

// CloseIdleConnections releases pooled connections without interrupting active
// downloads. Reusable Downloaders should call it when no longer needed.
func (d *Downloader) CloseIdleConnections() { d.client.CloseIdleConnections() }

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
	if req.Out != "" && (policy == ExistingFileSkip || policy == ExistingFileError) {
		path := filepath.Join(req.Dir, req.Out)
		res, handled, err := d.handleKnownExisting(ctx, req, path, policy, started)
		if err != nil {
			d.failEvent(req.ID, req.URLs[0], path, started, err)
			return Result{}, err
		}
		if handled {
			return res, nil
		}
	}
	if d.opts.OnProgress != nil {
		d.opts.OnProgress(Progress{ID: req.ID, State: StateProbing, StartedAt: started, UpdatedAt: time.Now()})
	}
	d.emitEvent(Event{Kind: EventProbing, ID: req.ID, URL: req.URLs[0], Time: time.Now()})

	info, err := d.probe(ctx, req.URLs, headers)
	if err != nil {
		d.failEvent(req.ID, req.URLs[0], "", started, err)
		return Result{}, err
	}
	if req.ExpectedSize > 0 && info.Size >= 0 && req.ExpectedSize != info.Size {
		err := fmt.Errorf("unexpected size: expected %d got %d", req.ExpectedSize, info.Size)
		d.failEvent(req.ID, info.URL, "", started, err)
		return Result{}, err
	}
	if req.ExpectedSize > 0 && info.Size < 0 {
		info.Size = req.ExpectedSize
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

	// Multiple responses need a byte-exact identity or an expected final digest.
	// Last-Modified alone is too coarse to exclude same-second replacements.
	useRanges := info.Size > 0 && info.AcceptRanges && (strongETag(info.ETag) || !req.Checksum.Empty())
	connections := 1
	if useRanges && d.opts.Split > 1 && info.Size/d.opts.MinSplitSize >= 2 {
		connections = min(d.opts.Split, d.opts.MaxConnectionsPerServer)
		if int64(connections) > info.Size {
			connections = int(info.Size)
		}
	}

	var res Result
	headers = representationHeaders(headers, info)
	if connections > 1 {
		res, err = d.downloadSegmented(ctx, req, headers, info, finalPath, connections, started)
		if errors.Is(err, ErrServerNoRange) {
			// Some origins advertise byte ranges but ignore them. Do not fail an
			// otherwise downloadable object, or publish the preallocated holes.
			res, err = d.downloadSingle(ctx, req, headers, info, finalPath, false, started)
		}
	} else {
		// A partial file can try a resume even without Accept-Ranges. A server
		// returning 200 instead of 206 triggers the safe restart path below.
		res, err = d.downloadSingle(ctx, req, headers, info, finalPath, info.Size > 0, started)
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
	res.Downloaded = res.Size - res.ResumedBytes
	if res.Duration > 0 && res.Downloaded > 0 {
		res.AvgSpeed = float64(res.Downloaded) / res.Duration.Seconds()
	}
	prog := Progress{ID: req.ID, State: StateComplete, Path: res.Path, Total: res.Size, Completed: res.Size, Resumed: res.ResumedBytes, Downloaded: res.Downloaded, Connections: res.Connections, StartedAt: started, UpdatedAt: res.FinishedAt, AvgSpeed: res.AvgSpeed}
	if d.opts.OnProgress != nil {
		d.opts.OnProgress(prog)
	}
	d.emitEvent(Event{Kind: EventCompleted, ID: req.ID, URL: res.URL, Path: res.Path, Total: res.Size, Completed: res.Size, Time: res.FinishedAt, Progress: prog})
	return res, nil
}

func (d *Downloader) handleKnownExisting(ctx context.Context, req Request, path string, policy ExistingFilePolicy, started time.Time) (Result, bool, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, true, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Result{}, false, nil
		}
		return Result{}, true, err
	}
	if !d.opts.DisableFileLock {
		lock, err := acquireFileLock(path)
		if err != nil {
			return Result{}, true, err
		}
		defer lock.Release()
	}
	return d.handleExisting(ctx, req, probeInfo{URL: req.URLs[0], Size: -1}, path, policy, started)
}

func (d *Downloader) handleExisting(ctx context.Context, req Request, info probeInfo, finalPath string, policy ExistingFilePolicy, started time.Time) (Result, bool, error) {
	_ = ctx
	st, statErr := os.Stat(finalPath)
	exists := statErr == nil && !st.IsDir()
	if statErr != nil && !os.IsNotExist(statErr) {
		return Result{}, false, statErr
	}
	if statErr == nil && !st.Mode().IsRegular() {
		return Result{}, true, fmt.Errorf("target is not a regular file: %s", finalPath)
	}
	if policy == ExistingFileOverwrite {
		for _, path := range []string{finalPath, partPath(finalPath), sidecarPath(finalPath)} {
			if err := removeIfExists(path); err != nil {
				return Result{}, false, err
			}
		}
		return Result{}, false, nil
	}
	if !exists {
		return Result{}, false, nil
	}
	if policy == ExistingFileError {
		return Result{}, true, fmt.Errorf("target exists: %s", finalPath)
	}
	if policy == ExistingFileSkip || (policy == ExistingFileResume && !hasResumeState(finalPath)) {
		res := Result{ID: req.ID, Path: finalPath, URL: info.URL, Size: st.Size(), Skipped: true, Connections: 0, StartedAt: started}
		if req.ExpectedSize > 0 && st.Size() != req.ExpectedSize {
			return Result{}, true, fmt.Errorf("existing file size mismatch: expected %d got %d", req.ExpectedSize, st.Size())
		}
		if !req.Checksum.Empty() {
			actual, err := req.Checksum.VerifyFile(finalPath)
			res.Checksum = req.Checksum.normalized()
			res.ChecksumActual = actual
			if err != nil {
				return Result{}, true, err
			}
			res.Verified = true
		}
		if policy == ExistingFileSkip || info.Size >= 0 && st.Size() == info.Size || info.Size < 0 {
			res.FinishedAt = time.Now()
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
	if st, err := os.Stat(sidecarPath(finalPath)); err == nil && st.Mode().IsRegular() {
		return true
	}
	if st, err := os.Stat(partPath(finalPath)); err == nil && st.Mode().IsRegular() {
		return true
	}
	return false
}

func (d *Downloader) downloadSegmented(ctx context.Context, req Request, headers http.Header, info probeInfo, finalPath string, connections int, started time.Time) (Result, error) {
	if err := validateMetadataSize(info.Size, d.opts.ResumeBlockSize); err != nil {
		return Result{}, err
	}
	sidecar := sidecarPath(finalPath)
	resumed := false
	var meta *metaFile
	if !d.opts.DisableResume {
		if loaded, err := loadMeta(sidecar); err == nil && !loaded.Single && loaded.compatible(finalPath, info) {
			// A sidecar alone cannot prove that its completed bytes still exist.
			if st, err := os.Stat(finalPath); err == nil && st.Mode().IsRegular() && st.Size() == info.Size {
				meta = loaded
			}
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
	if err := meta.verifyCompletedBlocks(ctx, f); err != nil {
		return Result{}, err
	}
	resumed = meta.completedBytes() > 0
	if err := preallocateFile(f, info.Size); err != nil {
		return Result{}, err
	}

	tracker := newProgressTracker(req.ID, info.Size, connections, int64(meta.piecesTotal()), finalPath, d.opts.ProgressInterval, d.opts.OnProgress)
	resumedBytes := meta.completedBytes()
	tracker.setCompleted(resumedBytes)
	tracker.piecesDone.Store(int64(meta.donePieces()))
	tracker.emit(StateDownloading, "", true)
	d.emitEvent(Event{Kind: EventDownloading, ID: req.ID, URL: info.URL, Path: finalPath, Time: time.Now()})

	jobs := make(chan piece)
	errCh := make(chan error, 1)
	flusher := startMetadataFlusher(meta, sidecar, d.opts.MetadataFlushInterval, f.Sync)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for p := range jobs {
				tracker.incActive()
				err := d.downloadPieceWithRetry(ctx, req.ID, req.URLs, headers, f, p, meta, sidecar, tracker, flusher)
				tracker.decActive()
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
				// Only block-bit transitions increment the counter. Replacing it
				// with a metadata snapshot races with other workers' increments.
				d.emitEvent(Event{Kind: EventPieceDone, ID: req.ID, URL: info.URL, Path: finalPath, PieceIndex: p.Index, Total: info.Size, Completed: tracker.completed.Load(), Time: time.Now(), Progress: tracker.snapshot(StateDownloading, "", time.Now())})
			}
		}(i)
	}

enqueue:
	for _, p := range meta.pendingPieces(max(d.opts.Split, connections*4), d.opts.MinSplitSize) {
		select {
		case <-ctx.Done():
			break enqueue
		case jobs <- p:
		}
	}
	close(jobs)
	wg.Wait()
	if err := flusher.close(); err != nil {
		return Result{}, err
	}

	if err := f.Sync(); err != nil {
		return Result{}, err
	}
	var transferErr error
	select {
	case err := <-errCh:
		transferErr = err
	default:
		transferErr = ctx.Err()
	}
	if transferErr != nil || d.opts.KeepPartFile {
		if err := meta.save(sidecar); err != nil {
			return Result{}, err
		}
	}
	if transferErr != nil {
		if errors.Is(transferErr, context.Canceled) || errors.Is(transferErr, context.DeadlineExceeded) {
			tracker.emit(StateCancelled, transferErr.Error(), true)
		}
		return Result{}, transferErr
	}
	if !d.opts.KeepPartFile {
		// The complete output is already synced. Do not rewrite and fsync a
		// completed checkpoint just to delete it; persist the directory change.
		_ = removeIfExists(sidecar)
		if err := syncDir(filepath.Dir(finalPath)); err != nil {
			return Result{}, err
		}
	}
	return Result{ID: req.ID, Path: finalPath, URL: info.URL, Size: info.Size, Resumed: resumed, ResumedBytes: resumedBytes, Connections: connections, StartedAt: started}, nil
}

func (d *Downloader) downloadSingle(ctx context.Context, req Request, headers http.Header, info probeInfo, finalPath string, canResume bool, started time.Time) (Result, error) {
	tmp := partPath(finalPath)
	offset := int64(0)
	resumed := false
	flag := os.O_CREATE | os.O_WRONLY
	if !d.opts.DisableResume && canResume {
		if st, err := os.Stat(tmp); err == nil && st.Size() > 0 && (info.Size <= 0 || st.Size() < info.Size) {
			if meta, err := loadMeta(sidecarPath(finalPath)); err == nil && meta.Single && meta.compatible(finalPath, info) && meta.PrefixSize == st.Size() {
				if prefix, err := os.Open(tmp); err == nil {
					digest, hashErr := hashFileBlock(prefix, 0, st.Size())
					_ = prefix.Close()
					if hashErr == nil && fmt.Sprintf("%x", digest) == meta.PrefixHash {
						offset = st.Size()
						resumed = true
					}
				}
			}
		}
	}
	if resumed {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(tmp, flag, 0o644)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	// A bare .part file has no proven object identity. Never append to it just
	// because its length is plausible, including when switching transfer modes.

	tracker := newProgressTracker(req.ID, info.Size, 1, 1, finalPath, d.opts.ProgressInterval, d.opts.OnProgress)
	tracker.setCompleted(offset)
	tracker.incActive()
	d.emitEvent(Event{Kind: EventDownloading, ID: req.ID, URL: info.URL, Path: finalPath, Time: time.Now()})
	err = d.getToWriterWithRetry(ctx, req.ID, req.URLs, headers, f, offset, info.Size, tracker)
	if errors.Is(err, ErrServerNoRange) && tracker.completed.Load() > 0 {
		if truncateErr := f.Truncate(0); truncateErr != nil {
			return Result{}, truncateErr
		}
		if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
			return Result{}, seekErr
		}
		tracker.setCompleted(0)
		resumed = false
		offset = 0
		err = d.getToWriterWithRetry(ctx, req.ID, req.URLs, headers, f, 0, info.Size, tracker)
	}
	tracker.decActive()
	if err != nil {
		if syncErr := f.Sync(); syncErr != nil {
			return Result{}, syncErr
		}
		if info.Size > 0 {
			if saveErr := saveSingleResume(tmp, finalPath, req.URLs, info); saveErr != nil {
				return Result{}, saveErr
			}
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			tracker.emit(StateCancelled, err.Error(), true)
		}
		return Result{}, err
	}
	if info.Size >= 0 {
		st, err := f.Stat()
		if err != nil {
			return Result{}, err
		}
		if st.Size() != info.Size {
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
		_ = removeIfExists(sidecarPath(finalPath))
	}
	size := info.Size
	if size <= 0 {
		if st, err := os.Stat(finalPath); err == nil {
			size = st.Size()
		}
	}
	return Result{ID: req.ID, Path: finalPath, URL: info.URL, Size: size, Resumed: resumed, ResumedBytes: offset, Connections: 1, StartedAt: started}, nil
}

func (d *Downloader) downloadPieceWithRetry(ctx context.Context, id string, urls []string, headers http.Header, f *os.File, p piece, meta *metaFile, sidecar string, tracker *progressTracker, flusher *metadataFlusher) error {
	var last error
	badURLs := make(map[string]bool)
	tries := max(1, d.opts.MaxRetries+1)
	done := int64(0)
	if p.size() <= 0 {
		return nil
	}

	for attempt := 0; attempt < tries && done < p.size(); attempt++ {
		canRetry := false
		for _, raw := range d.mirrors.ordered(urls, p.Index+attempt) {
			if badURLs[raw] {
				continue
			}
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
			started := time.Now()
			n, err := d.downloadPiece(ctx, raw, headers, f, reqStart, p.End, meta.Size, tracker, func(writtenThisRequest int64) error {
				if writtenThisRequest <= 0 {
					return nil
				}
				changed, markErr := meta.markRangeCompleteFromFile(f, markStart, reqStart+writtenThisRequest-1, sidecar)
				if markErr != nil {
					return markErr
				}
				if changed > 0 {
					tracker.addPiecesDone(int64(changed))
					flusher.request()
					// Already-completed blocks need not be rescanned on each buffer.
					markStart = ((reqStart + writtenThisRequest) / meta.BlockSize) * meta.BlockSize
				}
				return nil
			})
			d.mirrors.observe(raw, n, time.Since(started), err)
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
				var statusErr *HTTPStatusError
				if errors.As(err, &statusErr) || errors.Is(err, ErrInvalidContentRange) || errors.Is(err, ErrServerNoRange) || errors.Is(err, ErrRemoteChanged) {
					badURLs[raw] = true
					continue // A permanent failure of one mirror need not fail the file.
				}
				return err
			}
			canRetry = true
			d.emitRetry(id, raw, p.Index, attempt+1, err, tracker)
		}
		if !canRetry {
			break
		}
		if attempt+1 < tries {
			if err := sleepContext(ctx, d.retryDelayFor(attempt, last)); err != nil {
				return err
			}
		}
	}
	if last != nil {
		return last
	}
	return io.ErrUnexpectedEOF
}

func (d *Downloader) downloadPiece(ctx context.Context, raw string, headers http.Header, f *os.File, start, end, total int64, tracker *progressTracker, onWrite func(int64) error) (int64, error) {
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
	if err := validateRepresentation(headers, resp); err != nil {
		return 0, err
	}
	if resp.StatusCode == http.StatusOK {
		return 0, ErrServerNoRange
	}
	if resp.StatusCode != http.StatusPartialContent {
		return 0, &HTTPStatusError{URL: raw, StatusCode: resp.StatusCode, Status: resp.Status, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
	}
	if err := validateContentRange(resp.Header.Get("Content-Range"), start, end, total); err != nil {
		return 0, err
	}
	buf := d.getBuffer()
	defer d.putBuffer(buf)
	return copyRangeToFile(f, resp.Body, start, end-start+1, buf, tracker, onWrite)
}

func (d *Downloader) getToWriterWithRetry(ctx context.Context, id string, urls []string, headers http.Header, w io.Writer, offset int64, total int64, tracker *progressTracker) error {
	var last error
	badURLs := make(map[string]bool)
	tries := max(1, d.opts.MaxRetries+1)
	current := offset
	for attempt := 0; attempt < tries; attempt++ {
		canRetry := false
		for _, raw := range d.mirrors.ordered(urls, attempt) {
			if badURLs[raw] {
				continue
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			started := time.Now()
			n, err := d.getToWriter(ctx, raw, headers, w, current, total, tracker)
			d.mirrors.observe(raw, n, time.Since(started), err)
			current += n
			// Without a representation validator, appending a retry could combine
			// two versions. Let the caller restart the stream from byte zero.
			if retryable(err) && current > 0 && !strongETag(headers.Get("If-Match")) {
				return ErrServerNoRange
			}
			if err == nil {
				return nil
			}
			last = err
			if !retryable(err) {
				var statusErr *HTTPStatusError
				if errors.As(err, &statusErr) || errors.Is(err, ErrInvalidContentRange) || errors.Is(err, ErrServerNoRange) || errors.Is(err, ErrRemoteChanged) {
					badURLs[raw] = true
					continue
				}
				return err
			}
			canRetry = true
			d.emitRetry(id, raw, -1, attempt+1, err, tracker)
		}
		if !canRetry {
			break
		}
		if attempt+1 < tries {
			if err := sleepContext(ctx, d.retryDelayFor(attempt, last)); err != nil {
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
	if err := validateRepresentation(headers, resp); err != nil {
		return 0, err
	}
	if offset > 0 {
		switch resp.StatusCode {
		case http.StatusPartialContent:
			if err := validateContentRange(resp.Header.Get("Content-Range"), offset, -1, total); err != nil {
				return 0, err
			}
		case http.StatusOK:
			return 0, ErrServerNoRange
		default:
			return 0, &HTTPStatusError{URL: raw, StatusCode: resp.StatusCode, Status: resp.Status, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
		}
	} else if resp.StatusCode != http.StatusOK {
		return 0, &HTTPStatusError{URL: raw, StatusCode: resp.StatusCode, Status: resp.Status, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())}
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
	var pathErr *os.PathError
	if errors.As(err, &pathErr) || errors.Is(err, io.ErrShortWrite) {
		return false
	}
	if errors.Is(err, ErrServerNoRange) || errors.Is(err, ErrChecksumMismatch) || errors.Is(err, ErrInvalidContentRange) || errors.Is(err, ErrRemoteChanged) {
		return false
	}
	return true
}

func (d *Downloader) retryDelay(attempt int) time.Duration {
	base := d.opts.RetryWait
	if base <= 0 {
		return 0
	}
	limit := d.opts.MaxRetryWait
	if limit <= 0 {
		limit = time.Duration(math.MaxInt64)
	}
	base = min(base, limit)
	for i := 0; i < attempt; i++ {
		if base >= limit-base {
			base = limit
			break
		}
		base *= 2
	}
	// Deterministic bounded jitter avoids synchronized retry storms while keeping tests stable.
	percent := (attempt%21*(1103515245%21)+12345%21)%21 - 10
	jitter := base / 100 * time.Duration(percent)
	if jitter > 0 && base > limit-jitter {
		return limit
	}
	return base + jitter
}

func (d *Downloader) retryDelayFor(attempt int, err error) time.Duration {
	delay := d.retryDelay(attempt)
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.RetryAfter > delay {
		return statusErr.RetryAfter
	}
	return delay
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
