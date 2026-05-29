package hydra

import (
	"errors"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

const (
	DefaultUserAgent             = "hydra/0.2"
	DefaultMinSplitSize          = int64(20 << 20) // 20 MiB.
	DefaultSplit                 = 5
	DefaultMaxConns              = 8
	DefaultConnectTimeout        = 30 * time.Second
	DefaultTimeout               = 60 * time.Second
	DefaultRetryWait             = time.Second
	DefaultMaxRetryWait          = 30 * time.Second
	DefaultMaxRetries            = 5
	DefaultBufferSize            = 256 << 10
	DefaultProgressInterval      = 200 * time.Millisecond
	DefaultMetadataFlushInterval = 2 * time.Second
	DefaultResumeBlockSize       = int64(1 << 20) // 1 MiB bitfield resume granularity.
)

// ExistingFilePolicy controls what happens when the final output already exists.
type ExistingFilePolicy string

const (
	// ExistingFileResume keeps safe continuation behavior: resume partial
	// downloads where possible and overwrite only through the normal .part/final
	// promotion flow after a successful transfer.
	ExistingFileResume ExistingFilePolicy = "resume"
	// ExistingFileOverwrite removes final/partial metadata and starts fresh.
	ExistingFileOverwrite ExistingFilePolicy = "overwrite"
	// ExistingFileSkip returns success without downloading when the target exists.
	ExistingFileSkip ExistingFilePolicy = "skip"
	// ExistingFileError fails when the target exists.
	ExistingFileError ExistingFilePolicy = "error"
)

// Options configures a Downloader. It intentionally keeps only the knobs that
// matter for HTTP/HTTPS downloads in Go. Transport-level details are exposed via
// custom hooks instead of exposing redundant transport flags.
type Options struct {
	// Split is the maximum number of range pieces for one file.
	Split int

	// MaxConnectionsPerServer limits concurrent TCP connections per host.
	MaxConnectionsPerServer int

	// MaxConcurrentDownloads is used by Manager. One-shot Download ignores it.
	MaxConcurrentDownloads int

	// MinSplitSize prevents wasteful segmentation for small files. A file is
	// split only when size >= 2*MinSplitSize and server supports byte ranges.
	MinSplitSize int64

	ConnectTimeout time.Duration
	Timeout        time.Duration
	MaxRetries     int
	RetryWait      time.Duration
	MaxRetryWait   time.Duration

	// BufferSize controls copy buffer size per active connection.
	BufferSize int

	// ResumeBlockSize is the fixed block size represented by one bit in the
	// .hydra sidecar. Smaller values resume more precisely without deciding the
	// normal HTTP Range request size. Range planning is controlled by Split and
	// MinSplitSize. Defaults to 1MiB.
	ResumeBlockSize int64

	// MetadataFlushInterval batches .hydra sidecar fsyncs during segmented
	// downloads. A final save still happens on success, retry failure, or
	// cancellation. Lower values protect more progress against hard crashes;
	// higher values reduce filesystem overhead. Defaults to 2s.
	MetadataFlushInterval time.Duration

	// ProgressInterval throttles progress callbacks. Completion/failure events are
	// always forced immediately.
	ProgressInterval time.Duration

	// Proxy may be empty, an http(s) URL, or socks5/socks5h URL.
	// Examples: http://user:pass@127.0.0.1:8080, socks5h://127.0.0.1:1080.
	Proxy string

	// NoProxyFromEnvironment disables HTTP_PROXY/HTTPS_PROXY/NO_PROXY lookup when
	// Proxy is empty.
	NoProxyFromEnvironment bool

	UserAgent string
	Headers   http.Header

	// DisableResume disables completed-piece resume using the sidecar metadata file.
	DisableResume bool

	// StrictResumeValidation rejects sidecar metadata when ETag/Last-Modified
	// validators change between runs. The default is false because many CDNs,
	// signed URLs, and object gateways emit unstable validators even when the
	// byte object is identical. Size/path are still checked, and checksum
	// verification can be used for strong final integrity.
	StrictResumeValidation bool

	// ExistingFile controls existing final-file behavior.
	ExistingFile ExistingFilePolicy

	// KeepPartFile keeps the .part/sidecar files after success for debugging.
	KeepPartFile bool

	// DisableFileLock disables the target .hydra.lock guard. Keep enabled for CLIs.
	DisableFileLock bool

	// DisableCompression keeps byte offsets stable and avoids transparent gzip
	// breaking Range semantics.
	DisableCompression bool

	// OnProgress is called from worker goroutines. Keep it fast/non-blocking.
	OnProgress ProgressFunc

	// OnEvent receives lifecycle/retry/piece/checksum events.
	OnEvent EventFunc

	// Transport allows advanced users to provide their own transport. If set,
	// Proxy, ConnectTimeout and MaxConnectionsPerServer are ignored for transport
	// construction, but request headers/retries still apply.
	Transport http.RoundTripper
}

func DefaultOptions() Options {
	return Options{
		Split:                   DefaultSplit,
		MaxConnectionsPerServer: DefaultMaxConns,
		MaxConcurrentDownloads:  1,
		MinSplitSize:            DefaultMinSplitSize,
		ConnectTimeout:          DefaultConnectTimeout,
		Timeout:                 DefaultTimeout,
		MaxRetries:              DefaultMaxRetries,
		RetryWait:               DefaultRetryWait,
		MaxRetryWait:            DefaultMaxRetryWait,
		BufferSize:              DefaultBufferSize,
		ResumeBlockSize:         DefaultResumeBlockSize,
		MetadataFlushInterval:   DefaultMetadataFlushInterval,
		ProgressInterval:        DefaultProgressInterval,
		UserAgent:               DefaultUserAgent,
		Headers:                 make(http.Header),
		DisableCompression:      true,
		ExistingFile:            ExistingFileResume,
	}
}

func (o Options) normalized() (Options, error) {
	d := DefaultOptions()
	if o.Split == 0 {
		o.Split = d.Split
	}
	if o.MaxConnectionsPerServer == 0 {
		o.MaxConnectionsPerServer = d.MaxConnectionsPerServer
	}
	if o.MaxConcurrentDownloads == 0 {
		o.MaxConcurrentDownloads = d.MaxConcurrentDownloads
	}
	if o.MinSplitSize == 0 {
		o.MinSplitSize = d.MinSplitSize
	}
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = d.ConnectTimeout
	}
	if o.Timeout == 0 {
		o.Timeout = d.Timeout
	}
	if o.MaxRetries == 0 {
		o.MaxRetries = d.MaxRetries
	}
	if o.RetryWait == 0 {
		o.RetryWait = d.RetryWait
	}
	if o.MaxRetryWait == 0 {
		o.MaxRetryWait = d.MaxRetryWait
	}
	if o.BufferSize == 0 {
		o.BufferSize = d.BufferSize
	}
	if o.ProgressInterval == 0 {
		o.ProgressInterval = d.ProgressInterval
	}
	if o.ResumeBlockSize == 0 {
		o.ResumeBlockSize = d.ResumeBlockSize
	}
	if o.MetadataFlushInterval == 0 {
		o.MetadataFlushInterval = d.MetadataFlushInterval
	}
	if o.UserAgent == "" {
		o.UserAgent = d.UserAgent
	}
	if o.Headers == nil {
		o.Headers = make(http.Header)
	}
	if o.ExistingFile == "" {
		o.ExistingFile = d.ExistingFile
	}
	if o.Split < 1 {
		return o, errors.New("Split must be >= 1")
	}
	if o.MaxConnectionsPerServer < 1 {
		return o, errors.New("MaxConnectionsPerServer must be >= 1")
	}
	if o.MaxConcurrentDownloads < 1 {
		return o, errors.New("MaxConcurrentDownloads must be >= 1")
	}
	if o.MinSplitSize < 1 {
		return o, errors.New("MinSplitSize must be >= 1")
	}
	if o.MaxRetries < 0 {
		return o, errors.New("MaxRetries must be >= 0")
	}
	if o.BufferSize < 32*1024 {
		return o, errors.New("BufferSize must be >= 32KiB")
	}
	if o.ResumeBlockSize < 32*1024 {
		return o, errors.New("ResumeBlockSize must be >= 32KiB")
	}
	if o.MetadataFlushInterval < 0 {
		return o, errors.New("MetadataFlushInterval must be >= 0")
	}
	switch o.ExistingFile {
	case ExistingFileResume, ExistingFileOverwrite, ExistingFileSkip, ExistingFileError:
	default:
		return o, errors.New("invalid ExistingFile policy: " + string(o.ExistingFile))
	}
	// Avoid accidental fd explosions while still permitting aggressive downloads.
	maxReasonable := max(64, runtime.GOMAXPROCS(0)*64)
	if o.MaxConnectionsPerServer > maxReasonable {
		o.MaxConnectionsPerServer = maxReasonable
	}
	if o.Split > maxReasonable {
		o.Split = maxReasonable
	}
	if o.MaxConcurrentDownloads > maxReasonable {
		o.MaxConcurrentDownloads = maxReasonable
	}
	if o.Proxy != "" {
		u, err := url.Parse(o.Proxy)
		if err != nil {
			return o, err
		}
		switch u.Scheme {
		case "http", "https", "socks5", "socks5h":
		default:
			return o, errors.New("unsupported proxy scheme: " + u.Scheme)
		}
	}
	return o, nil
}
