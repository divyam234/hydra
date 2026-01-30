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

	"github.com/bhunter/aria2go/internal/control"
	"github.com/bhunter/aria2go/internal/disk"
	internalhttp "github.com/bhunter/aria2go/internal/http"
	"github.com/bhunter/aria2go/internal/limit"
	"github.com/bhunter/aria2go/internal/segment"
	"github.com/bhunter/aria2go/internal/stats"
	"github.com/bhunter/aria2go/internal/ui"
	"github.com/bhunter/aria2go/internal/util"
	"github.com/bhunter/aria2go/pkg/option"
)

// RequestGroup represents a single download task
type RequestGroup struct {
	gid            GID
	uris           []string
	options        *option.Option
	diskAdaptor    disk.DiskAdaptor
	segmentMan     *segment.SegmentMan
	pieceStorage   segment.PieceStorage
	controller     *control.Controller
	httpClient     *http.Client
	limiter        *limit.BandwidthLimiter
	speedCalc      *stats.SpeedCalc
	console        *ui.Console
	totalLength    int64
	completedBytes atomic.Int64
	outputPath     string
	workers        int
}

// NewRequestGroup creates a new RequestGroup
func NewRequestGroup(gid GID, uris []string, opt *option.Option) *RequestGroup {
	return &RequestGroup{
		gid:         gid,
		uris:        uris,
		options:     opt,
		diskAdaptor: disk.NewDirectDiskAdaptor(),
	}
}

// Execute starts the download
func (rg *RequestGroup) Execute(ctx context.Context) error {
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
	rg.console = ui.NewConsole()

	// 2. Initialize Controller and check for resume
	rg.controller = control.NewController(rg.outputPath)
	var resumed bool
	var loadedCF *control.ControlFile

	if rg.controller.Exists() {
		fmt.Printf("Found control file, attempting to resume...\n")
		loadedCF, err = rg.controller.Load()
		if err == nil {
			// Basic validation
			if loadedCF.TotalLength > 0 {
				resumed = true
				rg.totalLength = loadedCF.TotalLength
				fmt.Printf("Resuming download of %s (Size: %d)\n", out, rg.totalLength)
			}
		} else {
			fmt.Printf("Failed to load control file: %v. Starting fresh.\n", err)
		}
	}

	if !resumed {
		fmt.Printf("Downloading %s to %s\n", uriStr, out)
		// Get File Size (HEAD Request)
		rg.httpClient = internalhttp.NewClient(rg.options)
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
			fmt.Println("File size unknown. Falling back to single connection download.")
			return rg.downloadSingle(ctx, uriStr, rg.httpClient)
		}

		// Check Accept-Ranges
		if headResp.Header.Get("Accept-Ranges") != "bytes" {
			fmt.Println("Server does not support 'Accept-Ranges'. Falling back to single connection download.")
			return rg.downloadSingle(ctx, uriStr, rg.httpClient)
		}

		fmt.Printf("File size: %d bytes.\n", rg.totalLength)
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
	rg.segmentMan = segment.NewSegmentMan(rg.pieceStorage)

	// Restore bitfield if resumed
	if resumed {
		rg.httpClient = internalhttp.NewClient(rg.options)

		if loadedCF.Bitfield != "" {
			if err := rg.pieceStorage.GetBitfield().FromHexString(loadedCF.Bitfield); err != nil {
				fmt.Printf("Warning: failed to restore bitfield: %v\n", err)
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
	}

	// 4. Open Disk Adaptor
	if err := rg.diskAdaptor.Open(rg.outputPath, rg.totalLength); err != nil {
		return err
	}
	defer rg.diskAdaptor.Close()

	// 5. Start Workers
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
		case <-ctx.Done():
			rg.saveControlFile()
			return ctx.Err()
		case err := <-errChan:
			rg.saveControlFile()
			return err
		case <-ticker.C:
			rg.saveControlFile()
		case <-statsTicker.C:
			// Print stats
			speed := rg.speedCalc.GetSpeed()

			// Use actual written bytes for progress
			written := rg.completedBytes.Load()
			if written > rg.totalLength {
				written = rg.totalLength
			}

			rg.console.PrintProgress(string(rg.gid), rg.totalLength, written, speed, maxConns)

		case <-doneChan:
			rg.console.ClearLine()
			if !rg.segmentMan.IsAllComplete() {
				rg.saveControlFile()
				return fmt.Errorf("download incomplete")
			}
			rg.console.ClearLine()
			fmt.Println("Download complete.")
			rg.controller.Remove() // Cleanup control file on success

			// Verify Checksum
			if checksum := rg.options.Get(option.Checksum); checksum != "" {
				fmt.Printf("Verifying checksum %s...\n", checksum)
				valid, err := util.VerifyChecksum(rg.outputPath, checksum)
				if err != nil {
					fmt.Printf("Checksum verification failed: %v\n", err)
				} else if valid {
					fmt.Println("Checksum OK")
				} else {
					fmt.Println("Checksum FAILED")
					return fmt.Errorf("checksum failed")
				}
			}

			return nil
		}
	}
}

func (rg *RequestGroup) saveControlFile() {
	err := rg.controller.Save(string(rg.gid), rg.pieceStorage, rg.uris, rg.outputPath)
	if err != nil {
		fmt.Printf("Failed to save control file: %v\n", err)
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
		// Expect multiple headers to be handled by option system potentially as list
		// For now simple single string splitting if manually joined, but option system likely stores last one
		// or we need GetAll. Assuming single header or manual implementation for now.
		parts := strings.SplitN(headers, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
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
				buf := make([]byte, 32*1024) // 32KB buffer

				// Wrap reader with limiter
				var reader io.Reader = resp.Body
				if rg.limiter != nil {
					reader = limit.NewReader(resp.Body, rg.limiter, ctx)
				}

				// Speed check variables
				lastCheckTime := time.Now()
				bytesSinceCheck := int64(0)
				checkInterval := 30 * time.Second

				for {
					n, readErr := reader.Read(buf)
					if n > 0 {
						_, writeErr := rg.diskAdaptor.WriteAt(buf[:n], currentStart)
						if writeErr != nil {
							return writeErr
						}

						currentStart += int64(n)
						rg.segmentMan.UpdateSegment(seg.Index, int64(n))
						rg.completedBytes.Add(int64(n))
						rg.speedCalc.Update(n)

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
						break
					}
					if readErr != nil {
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
			// Implement simple streaming download
			req, err := http.NewRequestWithContext(ctx, "GET", uriStr, nil)
			if err != nil {
				return err
			}
			rg.enrichRequest(req)

			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server returned %s", resp.Status)
			}

			// Prepare output file (overwrite for single mode simplification)
			f, err := os.Create(rg.outputPath)
			if err != nil {
				return err
			}
			defer f.Close()

			buf := make([]byte, 32*1024)
			var reader io.Reader = resp.Body
			if rg.limiter != nil {
				reader = limit.NewReader(resp.Body, rg.limiter, ctx)
			}

			totalWritten := int64(0)
			lastUpdate := time.Now()

			for {
				n, readErr := reader.Read(buf)
				if n > 0 {
					_, writeErr := f.Write(buf[:n])
					if writeErr != nil {
						return writeErr
					}
					totalWritten += int64(n)
					rg.speedCalc.Update(n)
					rg.completedBytes.Add(int64(n))

					// Simple progress update for single connection
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
