package option

// Preference constants mapped to aria2 option names
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

	// Proxy
	HttpProxy   = "http-proxy"
	HttpsProxy  = "https-proxy"
	AllProxy    = "all-proxy"
	NoProxy     = "no-proxy"
	ProxyMethod = "proxy-method"

	// Download Options
	Dir                     = "dir"
	Out                     = "out"
	MaxDownloadLimit        = "max-download-limit"
	MaxOverallDownloadLimit = "max-overall-download-limit"
	MaxConcurrentDownloads  = "max-concurrent-downloads"
	Continue                = "continue"
	AutoFileRenaming        = "auto-file-renaming"
	AllowOverwrite          = "allow-overwrite"

	// RPC Options
	EnableRpc         = "enable-rpc"
	RpcListenPort     = "rpc-listen-port"
	RpcListenAll      = "rpc-listen-all"
	RpcSecret         = "rpc-secret"
	RpcMaxRequestSize = "rpc-max-request-size"
	RpcAllowOriginAll = "rpc-allow-origin-all"

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
	DefaultUserAgent              = "aria2go/0.1.0"
	DefaultEnableHttpKeepAlive    = "true"
	DefaultEnableHttpPipelining   = "false"
	DefaultHttpNoCache            = "false"
	DefaultHttpAcceptGzip         = "true"
	DefaultProxyMethod            = "get"
	DefaultMaxConcurrentDownloads = "5"
	DefaultContinue               = "false"
	DefaultAutoFileRenaming       = "true"
	DefaultAllowOverwrite         = "false"
	DefaultRpcListenPort          = "6800"
	DefaultRpcMaxRequestSize      = "2M"
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
	opt.Put(RpcListenPort, DefaultRpcListenPort)
	opt.Put(RpcMaxRequestSize, DefaultRpcMaxRequestSize)
	return opt
}
