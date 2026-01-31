package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/divyam234/hydra/internal/control"
	"github.com/divyam234/hydra/internal/disk"
	internalhttp "github.com/divyam234/hydra/internal/http"
	"github.com/divyam234/hydra/internal/limit"
	"github.com/divyam234/hydra/internal/segment"
	"github.com/divyam234/hydra/internal/stats"
	"github.com/divyam234/hydra/internal/ui"
	"github.com/divyam234/hydra/internal/util"
	"github.com/divyam234/hydra/pkg/option"
)

// RequestGroup represents a single download task
type RequestGroup struct {
	gid                GID
	uris               []string
	options            *option.Option
	diskAdaptor        disk.DiskAdaptor
	segmentMan         *segment.SegmentMan
	pieceStorage       segment.PieceStorage
	controller         *control.Controller
	httpClient         *http.Client
	limiter            *limit.BandwidthLimiter
	speedCalc          *stats.SpeedCalc
	console            ui.UserInterface
	totalLength        int64
	completedBytes     atomic.Int64
	outputPath         string
	workers            int
	speedCheckInterval time.Duration // For testing

	// State tracking
	startTime        time.Time
	endTime          time.Time
	state            atomic.Int32 // RGState* constants
	lastError        error
	checksumOK       bool
	checksumVerified bool
	stateMu          sync.RWMutex // protects lastError, checksumOK, checksumVerified

	// Pause/Resume/Cancel control
	pauseCh    chan struct{}
	resumeCh   chan struct{}
	cancelCh   chan struct{}
	cancelOnce sync.Once

	// Queue management
	priority int // Higher priority downloads run first
}

// NewRequestGroup creates a new RequestGroup
func NewRequestGroup(gid GID, uris []string, opt *option.Option) *RequestGroup {
	rg := &RequestGroup{
		gid:                gid,
		uris:               uris,
		options:            opt,
		diskAdaptor:        disk.NewBufferedDiskAdaptor(opt.Get(option.FileAllocation)),
		speedCheckInterval: 30 * time.Second,
		pauseCh:            make(chan struct{}),
		resumeCh:           make(chan struct{}),
		cancelCh:           make(chan struct{}),
	}
	rg.state.Store(RGStatePending)
	return rg
}

// Pause pauses the download
func (rg *RequestGroup) Pause() bool {
	currentState := rg.state.Load()
	if currentState != RGStateActive {
		return false
	}
	if rg.state.CompareAndSwap(RGStateActive, RGStatePaused) {
		select {
		case rg.pauseCh <- struct{}{}:
		default:
		}
		return true
	}
	return false
}

// Resume resumes a paused download
func (rg *RequestGroup) Resume() bool {
	currentState := rg.state.Load()
	if currentState != RGStatePaused {
		return false
	}
	if rg.state.CompareAndSwap(RGStatePaused, RGStateActive) {
		select {
		case rg.resumeCh <- struct{}{}:
		default:
		}
		return true
	}
	return false
}

// Cancel cancels the download
func (rg *RequestGroup) Cancel() bool {
	currentState := rg.state.Load()
	if currentState == RGStateComplete || currentState == RGStateCancelled || currentState == RGStateError {
		return false
	}

	rg.cancelOnce.Do(func() {
		rg.state.Store(RGStateCancelled)
		close(rg.cancelCh)
	})
	return true
}

// IsPaused returns true if the download is paused
func (rg *RequestGroup) IsPaused() bool {
	return rg.state.Load() == RGStatePaused
}

// IsCancelled returns true if the download is cancelled
func (rg *RequestGroup) IsCancelled() bool {
	return rg.state.Load() == RGStateCancelled
}

// SetUI sets the user interface
func (rg *RequestGroup) SetUI(u ui.UserInterface) {
	rg.console = u
}

// Execute starts the download
func (rg *RequestGroup) Execute(ctx context.Context) (err error) {
	rg.stateMu.Lock()
	rg.startTime = time.Now()
	rg.state.Store(RGStateActive)
	rg.stateMu.Unlock()

	defer func() {
		rg.stateMu.Lock()
		rg.endTime = time.Now()
		currentState := rg.state.Load()
		// Don't override cancelled state
		if currentState == RGStateCancelled {
			rg.lastError = fmt.Errorf("download cancelled")
		} else if err != nil {
			rg.state.Store(RGStateError)
			rg.lastError = err
		} else {
			rg.state.Store(RGStateComplete)
		}
		rg.stateMu.Unlock()
	}()

	if len(rg.uris) == 0 {
		return fmt.Errorf("no URIs provided")
	}

	uriStr := rg.uris[0]
	u, err := util.ParseURI(uriStr)
	if err != nil {
		return err
	}

	// 1. Resolve Output Path
	out := rg.options.Get(option.Out)
	if out == "" {
		out = filepath.Base(u.Path)
		if out == "" || out == "/" || out == "." {
			out = "index.html"
		}
	}
	dir := rg.options.Get(option.Dir)
	if dir != "" {
		out = filepath.Join(dir, out)
	}
	rg.outputPath = out

	// Initialize Rate Limiter
	maxSpeed := 0
	if optStr := rg.options.Get(option.MaxDownloadLimit); optStr != "" {
		if val, err := option.ParseUnitNumber(optStr); err == nil {
			maxSpeed = int(val)
		}
	}
	rg.limiter = limit.NewBandwidthLimiter(maxSpeed)

	// Initialize Stats
	rg.speedCalc = stats.NewSpeedCalc()
	if rg.console == nil {
		quiet, _ := rg.options.GetAsBool(option.Quiet)
		var logWriter io.Writer
		if logPath := rg.options.Get(option.Log); logPath != "" {
			if logPath == "-" {
				logWriter = os.Stdout
			} else {
				f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
				if err != nil {
					return fmt.Errorf("failed to open log file: %w", err)
				}
				logWriter = f
			}
		}
		rg.console = ui.NewConsole(quiet, logWriter)
	}

	// 2. Initialize Controller and check for resume
	rg.controller = control.NewController(rg.outputPath)
	rg.httpClient = internalhttp.NewClient(rg.options) // Initialize once here
	var resumed bool
	var loadedCF *control.ControlFile

	if rg.controller.Exists() {
		// fmt.Printf("Found control file, attempting to resume...\n")
		loadedCF, err = rg.controller.Load()
		if err == nil {
			// Basic validation
			if loadedCF.TotalLength > 0 {
				resumed = true
				rg.totalLength = loadedCF.TotalLength
				// fmt.Printf("Resuming download of %s (Size: %d)\n", out, rg.totalLength)
			}
		} else {
			// fmt.Printf("Failed to load control file: %v. Starting fresh.\n", err)
		}
	}

	if !resumed {
		// Check for file conflict
		if _, err := os.Stat(rg.outputPath); err == nil {
			// File exists
			allowOverwrite, _ := rg.options.GetAsBool(option.AllowOverwrite)
			if !allowOverwrite {
				autoRename, _ := rg.options.GetAsBool(option.AutoFileRenaming)
				if autoRename {
					rg.outputPath = findNextAvailableName(rg.outputPath)
					out = rg.outputPath // Update local var for printing
					// Re-initialize controller for the new file
					rg.controller = control.NewController(rg.outputPath)
				} else {
					return fmt.Errorf("file already exists: %s", rg.outputPath)
				}
			}
		}

		rg.console.Printf("Downloading %s to %s\n", uriStr, out)
		// Get File Size (HEAD Request)
		headReq, err := http.NewRequestWithContext(ctx, "HEAD", uriStr, nil)
		if err != nil {
			return err
		}

		rg.enrichRequest(headReq)

		headResp, err := rg.httpClient.Do(headReq)
		if err != nil {
			return fmt.Errorf("failed to fetch headers: %w", err)
		}
		headResp.Body.Close()

		if headResp.StatusCode < 200 || headResp.StatusCode >= 300 {
			return fmt.Errorf("server returned error: %s", headResp.Status)
		}

		rg.totalLength = headResp.ContentLength
		// Check for single connection fallback
		if rg.totalLength <= 0 {
			// fmt.Println("File size unknown. Falling back to single connection download.")
			if err := rg.downloadSingle(ctx, uriStr, rg.httpClient); err != nil {
				return err
			}
			return rg.verifyChecksum()
		}

		// Check Accept-Ranges
		if headResp.Header.Get("Accept-Ranges") != "bytes" {
			// fmt.Println("Server does not support 'Accept-Ranges'. Falling back to single connection download.")
			if err := rg.downloadSingle(ctx, uriStr, rg.httpClient); err != nil {
				return err
			}
			return rg.verifyChecksum()
		}

		rg.console.Printf("File size: %d bytes.\n", rg.totalLength)
	}

	// 3. Initialize Segment System
	var pieceLength int64
	if resumed {
		pieceLength = loadedCF.PieceLength
	} else {
		pieceLength = segment.CalculateOptimalPieceLength(rg.totalLength)
	}

	// Initialize storage
	rg.pieceStorage = segment.NewDefaultPieceStorage(rg.totalLength, pieceLength)
	maxPieces, _ := rg.options.GetAsInt(option.MaxPiecesPerSegment)
	rg.segmentMan = segment.NewSegmentMan(rg.pieceStorage, maxPieces)

	// Set Piece Selector
	if sel := rg.options.Get(option.PieceSelector); sel == "random" {
		rg.segmentMan.SetSelector(segment.NewRandomSelector())
	}

	// Restore bitfield if resumed
	if resumed {
		if loadedCF.Bitfield != "" {
			if err := rg.pieceStorage.GetBitfield().FromHexString(loadedCF.Bitfield); err != nil {
				rg.console.Printf("Warning: failed to restore bitfield: %v\n", err)
			} else {
				// Update completedBytes based on restored bitfield
				// We must sum the actual length of each completed piece
				// because the last piece might be shorter than pieceLength
				restoredBytes := int64(0)
				for i := 0; i < rg.pieceStorage.GetNumPieces(); i++ {
					if rg.pieceStorage.HasPiece(i) {
						restoredBytes += rg.pieceStorage.GetPiece(i).Length
					}
				}
				rg.completedBytes.Store(restoredBytes)
			}
		}
	} else {
		// Save initial control file for fresh downloads
		// But only after we have totalLength
		if rg.totalLength > 0 {
			if err := rg.controller.Save(string(rg.gid), rg.pieceStorage, rg.uris, rg.outputPath); err != nil {
				// fmt.Printf("Initial save error: %v\n", err)
			}
		}
	}

	// 4. Open Disk Adaptor
	if err := rg.diskAdaptor.Open(rg.outputPath, rg.totalLength); err != nil {
		return err
	}
	defer rg.diskAdaptor.Close()

	// Start Workers
	maxConns, _ := rg.options.GetAsInt(option.Split)
	if maxConns <= 0 {
		maxConns = 1
	}

	var wg sync.WaitGroup
	errChan := make(chan error, maxConns)

	for i := 0; i < maxConns; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			if err := rg.downloadWorker(ctx, workerID, uriStr); err != nil {
				errChan <- err
			}
		}(i)
	}

	// Immediate save for testing/consistency
	rg.saveControlFile()

	// Auto-save and Stats ticker
	ticker := time.NewTicker(30 * time.Second)     // Auto-save every 30s
	statsTicker := time.NewTicker(1 * time.Second) // Stats every 1s
	defer ticker.Stop()
	defer statsTicker.Stop()

	// Wait for completion or error
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	for {
		select {
		case <-rg.cancelCh:
			// Download was cancelled
			rg.saveControlFile()
			return fmt.Errorf("download cancelled")
		case <-rg.pauseCh:
			// Download was paused, wait for resume or cancel
			rg.saveControlFile()
			select {
			case <-rg.resumeCh:
				// Resumed, continue
			case <-rg.cancelCh:
				return fmt.Errorf("download cancelled")
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			// Don't save if it was a cancellation during initialization?
			// Actually we want to save what we have.
			if rg.totalLength > 0 {
				rg.saveControlFile()
			}
			return ctx.Err()
		case err := <-errChan:
			rg.saveControlFile()
			return err
		case <-ticker.C:
			rg.saveControlFile()
		case <-statsTicker.C:
			// Skip stats if paused
			if rg.IsPaused() {
				continue
			}
			// Print stats
			speed := rg.speedCalc.GetSpeed()

			// Use actual written bytes for progress
			written := rg.completedBytes.Load()
			if written > rg.totalLength {
				written = rg.totalLength
			}

			rg.console.PrintProgress(string(rg.gid), rg.totalLength, written, speed, maxConns)

		case <-doneChan:
			// Send final progress update
			written := rg.completedBytes.Load()
			if written > rg.totalLength {
				written = rg.totalLength
			}
			rg.console.PrintProgress(string(rg.gid), rg.totalLength, written, 0, maxConns)

			rg.console.ClearLine()
			if !rg.segmentMan.IsAllComplete() {
				rg.saveControlFile()
				return fmt.Errorf("download incomplete")
			}
			rg.console.ClearLine()

			rg.console.Println("Download complete.")
			rg.controller.Remove() // Cleanup control file on success

			return rg.verifyChecksum()
		}
	}
}

func (rg *RequestGroup) saveControlFile() {
	err := rg.controller.Save(string(rg.gid), rg.pieceStorage, rg.uris, rg.outputPath)
	if err != nil {
		// fmt.Printf("Failed to save control file: %v\n", err)
	}
}

// verifyChecksum performs checksum validation
func (rg *RequestGroup) verifyChecksum() error {
	if checksum := rg.options.Get(option.Checksum); checksum != "" {
		rg.console.Printf("Verifying checksum %s...\n", checksum)
		valid, err := util.VerifyChecksum(rg.outputPath, checksum)

		rg.stateMu.Lock()
		rg.checksumOK = valid
		rg.checksumVerified = true
		rg.stateMu.Unlock()

		if err != nil {
			fmt.Printf("Checksum verification failed: %v\n", err)
			return fmt.Errorf("checksum verification error: %w", err)
		} else if valid {
			fmt.Println("Checksum OK")
		} else {
			fmt.Println("Checksum FAILED")
			return fmt.Errorf("checksum failed")
		}
	}
	return nil
}

// GetFullStatus returns the full status of the download
func (rg *RequestGroup) GetFullStatus() *DownloadStatus {
	rg.stateMu.RLock()
	defer rg.stateMu.RUnlock()

	speed := 0
	if rg.speedCalc != nil {
		speed = rg.speedCalc.GetSpeed()
	}

	return &DownloadStatus{
		GID:              rg.gid,
		Total:            rg.totalLength,
		Completed:        rg.completedBytes.Load(),
		Speed:            speed,
		State:            rg.state.Load(),
		OutputPath:       rg.outputPath,
		StartTime:        rg.startTime,
		EndTime:          rg.endTime,
		ChecksumOK:       rg.checksumOK,
		ChecksumVerified: rg.checksumVerified,
		Error:            rg.lastError,
	}
}

// enrichRequest adds headers and authentication to the request
func (rg *RequestGroup) enrichRequest(req *http.Request) {
	// User-Agent
	if ua := rg.options.Get(option.UserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}

	// Referer
	if ref := rg.options.Get(option.Referer); ref != "" {
		req.Header.Set("Referer", ref)
	}

	// Custom Headers
	if headers := rg.options.Get(option.Header); headers != "" {
		// Support multiple headers joined by \n
		lines := strings.Split(headers, "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	// Basic Auth
	user := rg.options.Get(option.HttpUser)
	pass := rg.options.Get(option.HttpPasswd)
	if user != "" || pass != "" {
		req.SetBasicAuth(user, pass)
	}
}

// downloadWorker runs a single download thread
func (rg *RequestGroup) downloadWorker(ctx context.Context, id int, uriStr string) error {
	maxTries, _ := rg.options.GetAsInt(option.MaxTries)
	if maxTries <= 0 {
		maxTries = 5 // Default
	}
	retryWait, _ := rg.options.GetAsInt(option.RetryWait)

	// Parse LowestSpeedLimit
	var lowestSpeedLimit int64
	if val := rg.options.Get(option.LowestSpeedLimit); val != "" {
		if v, err := option.ParseUnitNumber(val); err == nil {
			lowestSpeedLimit = v
		}
	}

	for {
		// Check for cancel
		select {
		case <-rg.cancelCh:
			return fmt.Errorf("download cancelled")
		default:
		}

		// Wait if paused
		for rg.IsPaused() {
			select {
			case <-rg.resumeCh:
				// Resumed
			case <-rg.cancelCh:
				return fmt.Errorf("download cancelled")
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
				// Check again
			}
		}

		// Get next segment
		seg := rg.segmentMan.GetSegment()
		if seg == nil {
			return nil // No more work
		}

		var lastErr error
		success := false

		for try := 0; try < maxTries; try++ {
			// Check context before retry
			select {
			case <-rg.cancelCh:
				rg.segmentMan.CancelSegment(seg.Index)
				return fmt.Errorf("download cancelled")
			case <-ctx.Done():
				rg.segmentMan.CancelSegment(seg.Index)
				return ctx.Err()
			default:
			}

			// Attempt download
			err := func() error {
				// Calculate range
				// Start from current written position to support resume within segment
				currentStart := seg.Position + seg.Written
				end := seg.Position + seg.Length - 1

				if currentStart > end {
					return nil // Already complete
				}

				req, err := http.NewRequestWithContext(ctx, "GET", uriStr, nil)
				if err != nil {
					return err
				}

				// Add Range header
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", currentStart, end))

				// Enrich with other headers
				rg.enrichRequest(req)

				resp, err := rg.httpClient.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
					return fmt.Errorf("server returned %s", resp.Status)
				}

				// Read and write body
				buf := util.GetBuffer()
				defer util.PutBuffer(buf)
				// buf := *bufPtr (Removed)

				// Wrap reader with limiter
				var reader io.Reader = resp.Body
				if rg.limiter != nil {
					reader = limit.NewReader(resp.Body, rg.limiter, ctx)
				}

				// Speed check variables
				lastCheckTime := time.Now()
				bytesSinceCheck := int64(0)
				checkInterval := rg.speedCheckInterval

				// Batch updates to reduce lock contention
				var pendingBytes int64
				const batchSize = 256 * 1024 // 256KB - matches buffer size for smoother progress

				for {
					n, readErr := reader.Read(buf)
					if n > 0 {
						_, writeErr := rg.diskAdaptor.WriteAt(buf[:n], currentStart)
						if writeErr != nil {
							return writeErr
						}

						currentStart += int64(n)
						pendingBytes += int64(n)

						if pendingBytes >= batchSize {
							rg.segmentMan.UpdateSegment(seg.Index, pendingBytes)
							rg.completedBytes.Add(pendingBytes)
							rg.speedCalc.Update(int(pendingBytes))
							pendingBytes = 0
						}

						// Lowest Speed Limit Check
						if lowestSpeedLimit > 0 {
							bytesSinceCheck += int64(n)
							if time.Since(lastCheckTime) >= checkInterval {
								speed := float64(bytesSinceCheck) / time.Since(lastCheckTime).Seconds()
								if speed < float64(lowestSpeedLimit) {
									return fmt.Errorf("speed %.0f < lowest limit %d", speed, lowestSpeedLimit)
								}
								lastCheckTime = time.Now()
								bytesSinceCheck = 0
							}
						}
					}

					if readErr == io.EOF {
						if pendingBytes > 0 {
							rg.segmentMan.UpdateSegment(seg.Index, pendingBytes)
							rg.completedBytes.Add(pendingBytes)
							rg.speedCalc.Update(int(pendingBytes))
							pendingBytes = 0
						}
						break
					}
					if readErr != nil {
						if pendingBytes > 0 {
							rg.segmentMan.UpdateSegment(seg.Index, pendingBytes)
							rg.completedBytes.Add(pendingBytes)
							rg.speedCalc.Update(int(pendingBytes))
						}
						return readErr
					}
				}
				return nil
			}()

			if err == nil {
				success = true
				rg.segmentMan.CompleteSegment(seg.Index)
				break
			}

			lastErr = err

			// Wait before retry
			if try < maxTries-1 {
				select {
				case <-ctx.Done():
					rg.segmentMan.CancelSegment(seg.Index)
					return ctx.Err()
				case <-time.After(time.Duration(retryWait) * time.Second):
					// Continue loop
				}
			}
		}

		if !success {
			// Failed after max tries
			rg.segmentMan.CancelSegment(seg.Index)
			return fmt.Errorf("worker %d failed segment %d after %d tries: %w", id, seg.Index, maxTries, lastErr)
		}
	}
}

// downloadSingle handles single-connection legacy download
func (rg *RequestGroup) downloadSingle(ctx context.Context, uriStr string, client *http.Client) error {
	maxTries, _ := rg.options.GetAsInt(option.MaxTries)
	if maxTries <= 0 {
		maxTries = 5
	}
	retryWait, _ := rg.options.GetAsInt(option.RetryWait)

	var lastErr error
	for try := 0; try < maxTries; try++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := func() error {
			// Check for existing file to support simple resume in single mode
			var startPos int64 = 0
			fileMode := os.O_CREATE | os.O_WRONLY

			if stat, err := os.Stat(rg.outputPath); err == nil {
				startPos = stat.Size()
				fileMode = os.O_APPEND | os.O_WRONLY
			}

			req, err := http.NewRequestWithContext(ctx, "GET", uriStr, nil)
			if err != nil {
				return err
			}

			if startPos > 0 {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startPos))
			}

			rg.enrichRequest(req)

			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if startPos > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
				// File already complete or range error
				return nil
			}

			if startPos > 0 && resp.StatusCode != http.StatusPartialContent {
				// Server doesn't support resume, restart
				startPos = 0
				fileMode = os.O_CREATE | os.O_WRONLY
			} else if startPos == 0 && resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server returned %s", resp.Status)
			}

			f, err := os.OpenFile(rg.outputPath, fileMode, 0666)
			if err != nil {
				return err
			}
			defer f.Close()

			buf := util.GetBuffer()
			defer util.PutBuffer(buf)
			var reader io.Reader = resp.Body
			if rg.limiter != nil {
				reader = limit.NewReader(resp.Body, rg.limiter, ctx)
			}

			totalWritten := startPos
			rg.completedBytes.Store(startPos)
			lastUpdate := time.Now()

			for {
				n, readErr := reader.Read(buf)
				if n > 0 {
					_, writeErr := f.Write(buf[:n])
					if writeErr != nil {
						return writeErr
					}
					written := int64(n)
					totalWritten += written
					rg.speedCalc.Update(n)
					rg.completedBytes.Add(written)

					if time.Since(lastUpdate) > time.Second {
						rg.console.PrintProgress(string(rg.gid), rg.totalLength, totalWritten, rg.speedCalc.GetSpeed(), 1)
						lastUpdate = time.Now()
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					return readErr
				}
			}
			return nil
		}()

		if err == nil {
			return nil
		}
		lastErr = err

		if try < maxTries-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(retryWait) * time.Second):
			}
		}
	}

	return fmt.Errorf("downloadSingle failed after %d tries: %w", maxTries, lastErr)
}

// findNextAvailableName finds the next available filename by appending a number
func findNextAvailableName(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
