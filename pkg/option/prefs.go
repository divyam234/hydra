package option

// Preference constants for Hydra download options
const (
	// Connection Options
	Timeout             = "timeout"
	ConnectTimeout      = "connect-timeout"
	MaxTries            = "max-tries"
	RetryWait           = "retry-wait"
	MaxConnPerServer    = "max-connection-per-server"
	Split               = "split"
	MinSplitSize        = "min-split-size"
	MaxPiecesPerSegment = "max-pieces-per-segment"
	LowestSpeedLimit    = "lowest-speed-limit"
	MaxFileNotFound     = "max-file-not-found"

	// Advanced Network Tuning
	ReadBufferSize      = "read-buffer-size"
	WriteBufferSize     = "write-buffer-size"
	MaxIdleConns        = "max-idle-conns"
	MaxIdleConnsPerHost = "max-idle-conns-per-host"
	IdleConnTimeout     = "idle-conn-timeout"

	// HTTP Options
	UserAgent            = "user-agent"
	Header               = "header"
	Referer              = "referer"
	EnableHttpKeepAlive  = "enable-http-keep-alive"
	EnableHttpPipelining = "enable-http-pipelining"
	HttpNoCache          = "http-no-cache"
	HttpAcceptGzip       = "http-accept-gzip"
	ConditionalGet       = "conditional-get"
	RemoteTime           = "remote-time"

	// Authentication
	HttpUser    = "http-user"
	HttpPasswd  = "http-passwd"
	NoNetrc     = "no-netrc"
	NetrcPath   = "netrc-path"
	LoadCookies = "load-cookies"

	// Security
	CheckCertificate = "check-certificate" // bool, default true

	// Proxy
	Proxy       = "proxy"
	NoProxy     = "no-proxy" // comma separated list of domains
	ProxyMethod = "proxy-method"

	// Download Options
	Dir                     = "dir"
	Out                     = "out"
	MaxDownloadLimit        = "max-download-limit"
	MaxOverallDownloadLimit = "max-overall-download-limit"
	MaxConcurrentDownloads  = "max-concurrent-downloads"
	ForceSequential         = "force-sequential" // bool, default false
	Continue                = "continue"
	AutoFileRenaming        = "auto-file-renaming"
	AllowOverwrite          = "allow-overwrite"

	// Session Options
	InputFile           = "input-file"
	SaveSession         = "save-session"
	SaveSessionInterval = "save-session-interval"
	ConfPath            = "conf-path"
	NoConf              = "no-conf"

	// Checksum
	Checksum = "checksum"

	// Logging
	Log             = "log"
	ConsoleLogLevel = "console-log-level"
	Quiet           = "quiet"

	// PieceSelector sets the strategy for selecting pieces (inorder, random)
	PieceSelector = "piece-selector"

	// FileAllocation sets the file allocation method (none, trunc, falloc)
	FileAllocation = "file-allocation"
)

// Default values
const (
	DefaultTimeout                = "30"
	DefaultConnectTimeout         = "15"
	DefaultMaxTries               = "5"
	DefaultRetryWait              = "0"
	DefaultMaxConnPerServer       = "1"
	DefaultSplit                  = "5"
	DefaultMinSplitSize           = "20M"
	DefaultMaxPiecesPerSegment    = "20"
	DefaultUserAgent              = "hydra/0.1.0"
	DefaultEnableHttpKeepAlive    = "true"
	DefaultEnableHttpPipelining   = "false"
	DefaultHttpNoCache            = "false"
	DefaultHttpAcceptGzip         = "true"
	DefaultProxyMethod            = "get"
	DefaultMaxConcurrentDownloads = "5"
	DefaultContinue               = "false"
	DefaultAutoFileRenaming       = "true"
	DefaultAllowOverwrite         = "false"
	DefaultPieceSelector          = "inorder"
	DefaultFileAllocation         = "trunc"
	DefaultCheckCertificate       = "true"
	DefaultForceSequential        = "false"
	DefaultQuiet                  = "false"

	// Network Tuning Defaults
	DefaultReadBufferSize      = "256K"
	DefaultWriteBufferSize     = "64K"
	DefaultMaxIdleConns        = "1000"
	DefaultMaxIdleConnsPerHost = "32"
	DefaultIdleConnTimeout     = "120"
)

// GetDefaultOptions returns a new Option populated with default values
func GetDefaultOptions() *Option {
	opt := NewOption()
	opt.Put(Timeout, DefaultTimeout)
	opt.Put(ConnectTimeout, DefaultConnectTimeout)
	opt.Put(MaxTries, DefaultMaxTries)
	opt.Put(RetryWait, DefaultRetryWait)
	opt.Put(MaxConnPerServer, DefaultMaxConnPerServer)
	opt.Put(Split, DefaultSplit)
	opt.Put(MinSplitSize, DefaultMinSplitSize)
	opt.Put(MaxPiecesPerSegment, DefaultMaxPiecesPerSegment)
	opt.Put(UserAgent, DefaultUserAgent)
	opt.Put(EnableHttpKeepAlive, DefaultEnableHttpKeepAlive)
	opt.Put(EnableHttpPipelining, DefaultEnableHttpPipelining)
	opt.Put(HttpNoCache, DefaultHttpNoCache)
	opt.Put(HttpAcceptGzip, DefaultHttpAcceptGzip)
	opt.Put(ProxyMethod, DefaultProxyMethod)
	opt.Put(MaxConcurrentDownloads, DefaultMaxConcurrentDownloads)
	opt.Put(Continue, DefaultContinue)
	opt.Put(AutoFileRenaming, DefaultAutoFileRenaming)
	opt.Put(AllowOverwrite, DefaultAllowOverwrite)
	opt.Put(PieceSelector, DefaultPieceSelector)
	opt.Put(FileAllocation, DefaultFileAllocation)
	opt.Put(CheckCertificate, DefaultCheckCertificate)
	opt.Put(ForceSequential, DefaultForceSequential)
	opt.Put(Quiet, DefaultQuiet)

	// Network Tuning
	opt.Put(ReadBufferSize, DefaultReadBufferSize)
	opt.Put(WriteBufferSize, DefaultWriteBufferSize)
	opt.Put(MaxIdleConns, DefaultMaxIdleConns)
	opt.Put(MaxIdleConnsPerHost, DefaultMaxIdleConnsPerHost)
	opt.Put(IdleConnTimeout, DefaultIdleConnTimeout)

	return opt
}
