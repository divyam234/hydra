package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/divyam234/hydra"
)

type headerFlags http.Header

func (h *headerFlags) String() string { return fmt.Sprint(http.Header(*h)) }
func (h *headerFlags) Set(v string) error {
	k, val, ok := strings.Cut(v, ":")
	if !ok {
		return fmt.Errorf("header must be Name: value")
	}
	k = strings.TrimSpace(k)
	val = strings.TrimSpace(val)
	if k == "" {
		return fmt.Errorf("header name is empty")
	}
	http.Header(*h).Add(k, val)
	return nil
}

// Version is set via ldflags at build time (e.g. -X main.Version=v1.0.0).
var Version = "dev"

func main() {
	var headers = headerFlags(http.Header{})
	opts := hydra.DefaultOptions()
	dir := flag.String("d", ".", "output directory")
	var out string
	flag.StringVar(&out, "o", "", "output filename")
	flag.StringVar(&out, "out", "", "output filename")
	split := flag.Int("s", opts.Split, "number of split connections")
	maxConn := flag.Int("x", opts.MaxConnectionsPerServer, "max connections per server")
	minSplit := flag.String("k", "20M", "minimum split size, e.g. 1M, 20M, 512K")
	resumeBlock := flag.String("resume-block-size", "1M", "bitfield resume checkpoint granularity, e.g. 512K, 1M, 4M")
	metadataFlush := flag.Duration("metadata-flush-interval", opts.MetadataFlushInterval, "how often to fsync .hydra metadata during segmented downloads; final save is always forced")
	proxy := flag.String("proxy", "", "proxy URL: http://, https://, socks5://, socks5h://")
	connectTimeout := flag.Duration("connect-timeout", opts.ConnectTimeout, "connect/proxy timeout")
	timeout := flag.Duration("timeout", opts.Timeout, "per-request timeout")
	retries := flag.Int("retry", opts.MaxRetries, "max retries per piece")
	retryWait := flag.Duration("retry-wait", opts.RetryWait, "initial retry wait; doubles up to --max-retry-wait")
	maxRetryWait := flag.Duration("max-retry-wait", opts.MaxRetryWait, "maximum retry backoff")
	checksumText := flag.String("checksum", "", "verify final file, e.g. sha256:<hex>")
	existing := flag.String("if-exists", string(opts.ExistingFile), "existing target policy: resume, overwrite, skip, error")
	noContinue := flag.Bool("no-continue", false, "disable resume metadata/.part continuation")
	strictResume := flag.Bool("strict-resume-validation", false, "reject resume metadata when ETag/Last-Modified validators change")
	quiet := flag.Bool("quiet", false, "disable progress UI")
	jsonEvents := flag.Bool("json-events", false, "print lifecycle events as JSON lines to stderr")
	noEnvProxy := flag.Bool("no-env-proxy", false, "ignore HTTP_PROXY/HTTPS_PROXY/NO_PROXY when --proxy is empty")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Var(&headers, "H", "HTTP header, repeatable: -H 'Authorization: Bearer ...'")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] URL [mirror URL ...]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("hydra", Version)
		os.Exit(0)
	}

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	minSplitBytes, err := parseSize(*minSplit)
	if err != nil {
		die(err)
	}
	resumeBlockBytes, err := parseSize(*resumeBlock)
	if err != nil {
		die(err)
	}
	checksum, err := hydra.ParseChecksum(*checksumText)
	if err != nil {
		die(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ui := newTerminalUI(120 * time.Millisecond)
	opts.Split = *split
	opts.MaxConnectionsPerServer = *maxConn
	opts.MinSplitSize = minSplitBytes
	opts.ResumeBlockSize = resumeBlockBytes
	opts.MetadataFlushInterval = *metadataFlush
	opts.Proxy = *proxy
	opts.ConnectTimeout = *connectTimeout
	opts.Timeout = *timeout
	opts.MaxRetries = *retries
	opts.RetryWait = *retryWait
	opts.MaxRetryWait = *maxRetryWait
	opts.DisableResume = *noContinue
	opts.StrictResumeValidation = *strictResume
	opts.Headers = http.Header(headers)
	opts.ExistingFile = hydra.ExistingFilePolicy(*existing)
	opts.NoProxyFromEnvironment = *noEnvProxy
	if !*quiet && !*jsonEvents {
		opts.OnProgress = ui.Render
	}
	if *jsonEvents {
		opts.OnEvent = func(e hydra.Event) {
			b, _ := json.Marshal(e)
			fmt.Fprintln(os.Stderr, string(b))
		}
	}

	res, err := hydra.Download(ctx, hydra.Request{URLs: flag.Args(), Dir: *dir, Out: out, Checksum: checksum}, opts)
	if err != nil {
		die(err)
	}
	fmt.Println(res.Path)
}

type terminalUI struct {
	mu        sync.Mutex
	interval  time.Duration
	lastPrint time.Time
	speed     float64
	lastState hydra.State
}

func newTerminalUI(interval time.Duration) *terminalUI {
	if interval <= 0 {
		interval = 120 * time.Millisecond
	}
	return &terminalUI{interval: interval}
}

func (ui *terminalUI) Render(p hydra.Progress) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	now := time.Now()
	final := p.State == hydra.StateComplete || p.State == hydra.StateFailed || p.State == hydra.StateSkipped || p.State == hydra.StateCancelled
	if !final && !ui.lastPrint.IsZero() && now.Sub(ui.lastPrint) < ui.interval {
		return
	}
	ui.lastPrint = now

	rawSpeed := p.Speed
	if rawSpeed <= 0 {
		rawSpeed = p.AvgSpeed
	}
	// Terminal speed should be readable, not a per-buffer spike. The Progress API
	// already uses a rolling window; this extra EMA only smooths the display.
	if rawSpeed <= 0 {
		ui.speed *= 0.70
		if ui.speed < 1 {
			ui.speed = 0
		}
	} else if ui.speed <= 0 || p.State != ui.lastState {
		ui.speed = rawSpeed
	} else {
		ui.speed = ui.speed*0.75 + rawSpeed*0.25
	}
	ui.lastState = p.State

	fmt.Fprint(os.Stderr, "\r"+renderProgress(p, ui.speed))
	if final {
		fmt.Fprintln(os.Stderr)
	}
}

func renderProgress(p hydra.Progress, displaySpeed float64) string {
	state := string(p.State)
	bar := ""
	percent := ""
	if p.Total > 0 {
		pct := float64(p.Completed) * 100 / float64(p.Total)
		percent = fmt.Sprintf("%5.1f%%", pct)
		bar = progressBar(p.Completed, p.Total, 30)
	} else {
		percent = "  n/a "
		bar = strings.Repeat("=", 8) + ">"
	}
	if displaySpeed <= 0 {
		displaySpeed = p.Speed
	}
	if displaySpeed <= 0 {
		displaySpeed = p.AvgSpeed
	}
	speed := humanBytes(int64(displaySpeed)) + "/s"
	eta := "--"
	if p.ETA > 0 {
		eta = compactDuration(p.ETA)
	}
	active := fmt.Sprintf("%d", p.Active)
	if p.Connections > 0 {
		active = fmt.Sprintf("%d/%d", p.Active, p.Connections)
	}
	resumed := ""
	if p.Resumed > 0 {
		resumed = " resumed=" + humanBytes(p.Resumed)
	}
	avg := ""
	if p.AvgSpeed > 0 {
		avg = " avg=" + humanBytes(int64(p.AvgSpeed)) + "/s"
	}
	msg := fmt.Sprintf("%-11s %s %s %s/%s %s%s eta=%s active=%s retries=%d%s", state, bar, percent, humanBytes(p.Completed), humanBytes(p.Total), speed, avg, eta, active, p.Retries, resumed)
	if p.Error != "" {
		msg += " error=" + p.Error
	}
	if len(msg) < 150 {
		msg += strings.Repeat(" ", 150-len(msg))
	}
	return msg
}

func progressBar(done, total int64, width int) string {
	if total <= 0 || width <= 0 {
		return ""
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	if filled == width {
		return "[" + strings.Repeat("=", width) + "]"
	}
	return "[" + strings.Repeat("=", filled) + ">" + strings.Repeat(".", width-filled-1) + "]"
}

func compactDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return d.String()
	}
	if d < time.Hour {
		m := int(d / time.Minute)
		s := int((d % time.Minute) / time.Second)
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	return fmt.Sprintf("%dh%02dm", h, m)
}

func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "K"):
		mult = 1024
		s = strings.TrimSuffix(s, "K")
	case strings.HasSuffix(s, "M"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "G"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("size must be >= 0")
	}
	return n * mult, nil
}

func humanBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func die(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }
