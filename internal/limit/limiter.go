package limit

import (
	"context"
	"io"

	"golang.org/x/time/rate"
)

// BandwidthLimiter limits the rate of data transfer
type BandwidthLimiter struct {
	limiter *rate.Limiter
}

// NewBandwidthLimiter creates a new limiter with bytes per second limit
func NewBandwidthLimiter(limit int) *BandwidthLimiter {
	if limit <= 0 {
		return &BandwidthLimiter{limiter: nil}
	}
	// Burst size approx 1 second worth of data or fixed reasonable size
	burst := limit
	// Ensure burst is at least 256KB to accommodate larger read buffers (util.DefaultBufferSize)
	if burst < 256*1024 {
		burst = 256 * 1024
	}
	// Note: We don't cap burst anymore to prevent throttling on every read for high-speed downloads,
	// allowing 256KB chunks to pass through.

	return &BandwidthLimiter{
		limiter: rate.NewLimiter(rate.Limit(limit), burst),
	}
}

// Wait blocks until the limiter allows n events to happen
func (b *BandwidthLimiter) Wait(ctx context.Context, n int) error {
	if b.limiter == nil {
		return nil
	}
	return b.limiter.WaitN(ctx, n)
}

// Reader wraps an io.Reader with rate limiting
type Reader struct {
	r       io.Reader
	limiter *BandwidthLimiter
	ctx     context.Context
}

// NewReader creates a new rate-limited reader
func NewReader(r io.Reader, l *BandwidthLimiter, ctx context.Context) *Reader {
	return &Reader{
		r:       r,
		limiter: l,
		ctx:     ctx,
	}
}

func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		if err := r.limiter.Wait(r.ctx, n); err != nil {
			return n, err
		}
	}
	return n, err
}
