package downloader

import (
	"fmt"

	"github.com/divyam234/hydra/pkg/option"
)

// config holds the configuration for a download
type config struct {
	opt        *option.Option
	progressCb func(Progress)
	messageCb  func(string)

	// Engine-level options
	maxConcurrent int
	sessionFile   string
	eventCb       func(Event)
	priority      int
}

// Option configures the download
type Option func(*config)

// WithDir sets the directory to store the downloaded file
func WithDir(dir string) Option {
	return func(c *config) {
		c.opt.Put(option.Dir, dir)
	}
}

// WithFilename sets the filename of the downloaded file
func WithFilename(name string) Option {
	return func(c *config) {
		c.opt.Put(option.Out, name)
	}
}

// WithSplit sets the number of connections to use
func WithSplit(n int) Option {
	return func(c *config) {
		c.opt.Put(option.Split, fmt.Sprintf("%d", n))
	}
}

// WithMaxSpeed sets the max download speed (e.g. "1M", "500K")
func WithMaxSpeed(limit string) Option {
	return func(c *config) {
		c.opt.Put(option.MaxDownloadLimit, limit)
	}
}

// WithLowestSpeed sets the lowest speed limit (e.g. "10K")
func WithLowestSpeed(limit string) Option {
	return func(c *config) {
		c.opt.Put(option.LowestSpeedLimit, limit)
	}
}

// WithRetries sets the number of retries on error
func WithRetries(n int) Option {
	return func(c *config) {
		c.opt.Put(option.MaxTries, fmt.Sprintf("%d", n))
	}
}

// WithRetryWait sets the wait time between retries in seconds
func WithRetryWait(seconds int) Option {
	return func(c *config) {
		c.opt.Put(option.RetryWait, fmt.Sprintf("%d", seconds))
	}
}

// WithTimeout sets the timeout in seconds
func WithTimeout(seconds int) Option {
	return func(c *config) {
		c.opt.Put(option.Timeout, fmt.Sprintf("%d", seconds))
	}
}

// WithConnectTimeout sets the connect timeout in seconds
func WithConnectTimeout(seconds int) Option {
	return func(c *config) {
		c.opt.Put(option.ConnectTimeout, fmt.Sprintf("%d", seconds))
	}
}

// WithProxy sets the proxy URL for all protocols
func WithProxy(url string) Option {
	return func(c *config) {
		c.opt.Put(option.AllProxy, url)
	}
}

// WithAuth sets the HTTP Basic Auth credentials
func WithAuth(user, pass string) Option {
	return func(c *config) {
		c.opt.Put(option.HttpUser, user)
		c.opt.Put(option.HttpPasswd, pass)
	}
}

// WithCookieFile sets the path to a Netscape/Mozilla formatted cookie file
func WithCookieFile(path string) Option {
	return func(c *config) {
		c.opt.Put(option.LoadCookies, path)
	}
}

// WithChecksum sets the checksum verification (e.g. "sha-1=digest")
func WithChecksum(checksum string) Option {
	return func(c *config) {
		c.opt.Put(option.Checksum, checksum)
	}
}

// WithUserAgent sets the User-Agent header
func WithUserAgent(ua string) Option {
	return func(c *config) {
		c.opt.Put(option.UserAgent, ua)
	}
}

// WithReferer sets the Referer header
func WithReferer(ref string) Option {
	return func(c *config) {
		c.opt.Put(option.Referer, ref)
	}
}

// WithHeader adds a custom header to the request
func WithHeader(key, value string) Option {
	return func(c *config) {
		current := c.opt.Get(option.Header)
		newHeader := fmt.Sprintf("%s: %s", key, value)
		if current != "" {
			c.opt.Put(option.Header, current+"\n"+newHeader)
		} else {
			c.opt.Put(option.Header, newHeader)
		}
	}
}

// WithMaxPiecesPerSegment sets the maximum pieces per segment (chunk size control)
func WithMaxPiecesPerSegment(n int) Option {
	return func(c *config) {
		c.opt.Put(option.MaxPiecesPerSegment, fmt.Sprintf("%d", n))
	}
}

// WithProgress sets a callback for download progress
func WithProgress(cb func(Progress)) Option {
	return func(c *config) {
		c.progressCb = cb
	}
}

// WithMessageCallback sets a callback for log messages
func WithMessageCallback(cb func(string)) Option {
	return func(c *config) {
		c.messageCb = cb
	}
}

// WithMaxConcurrentDownloads sets the maximum number of concurrent downloads (engine-level)
func WithMaxConcurrentDownloads(n int) Option {
	return func(c *config) {
		c.maxConcurrent = n
	}
}

// WithSessionFile sets the session file path for persistence (engine-level)
func WithSessionFile(path string) Option {
	return func(c *config) {
		c.sessionFile = path
	}
}

// OnEvent sets the event callback for download events (engine-level)
func OnEvent(cb func(Event)) Option {
	return func(c *config) {
		c.eventCb = cb
	}
}

// WithPriority sets the download priority (higher values run first, per-download)
func WithPriority(priority int) Option {
	return func(c *config) {
		c.priority = priority
	}
}
