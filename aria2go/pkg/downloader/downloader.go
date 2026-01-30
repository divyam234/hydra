package downloader

import (
	"context"
	"time"
)

// Download performs a single file download.
// This is the simplest entry point for the library.
func Download(ctx context.Context, url string, opts ...Option) (*Result, error) {
	// Use NewEngine to handle configuration consistently
	eng := NewEngine(opts...)
	defer eng.Shutdown()

	// 4. Start download
	// Note: We do NOT pass opts here, as they are already applied in NewEngine
	id, err := eng.AddDownload(ctx, []string{url})
	if err != nil {
		return nil, err
	}

	// 5. Wait for completion
	if err := eng.Wait(); err != nil {
		return nil, err
	}

	// 6. Get status/result
	status, err := eng.Status(id)
	if err != nil {
		return nil, err
	}

	return &Result{
		Filename:         status.Filename,
		TotalBytes:       status.Progress.Total,
		Duration:         status.Duration,
		AverageSpeed:     calculateAverageSpeed(status.Progress.Total, status.Duration),
		ChecksumOK:       status.ChecksumOK,
		ChecksumVerified: status.ChecksumVerified,
	}, nil
}

func calculateAverageSpeed(bytes int64, d time.Duration) int64 {
	if d.Seconds() == 0 {
		return 0
	}
	return int64(float64(bytes) / d.Seconds())
}
