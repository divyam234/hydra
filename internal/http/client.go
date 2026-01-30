package http

import (
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/divyam234/hydra/pkg/option"
)

// NewClient creates a new HTTP client with custom transport
func NewClient(opt *option.Option) *http.Client {
	// Proxy function
	proxyFunc := http.ProxyFromEnvironment
	if proxyURL := opt.Get(option.AllProxy); proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			proxyFunc = http.ProxyURL(u)
		}
	} else {
		// Fallback to protocol specific (simplified)
		// Real implementation would distinguish http vs https requests
		if proxyURL := opt.Get(option.HttpProxy); proxyURL != "" {
			if u, err := url.Parse(proxyURL); err == nil {
				proxyFunc = http.ProxyURL(u)
			}
		}
	}

	// Default transport settings
	connectTimeout := 30
	if t, _ := opt.GetAsInt(option.ConnectTimeout); t > 0 {
		connectTimeout = t
	}

	transport := &http.Transport{
		Proxy: proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(connectTimeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Apply options
	if maxConnPerServer, _ := opt.GetAsInt(option.MaxConnPerServer); maxConnPerServer > 0 {
		transport.MaxIdleConnsPerHost = maxConnPerServer
		transport.MaxConnsPerHost = maxConnPerServer
	} else {
		transport.MaxIdleConnsPerHost = 10
	}

	keepAlive, _ := opt.GetAsBool(option.EnableHttpKeepAlive)
	transport.DisableKeepAlives = !keepAlive

	timeout := 60
	if t, _ := opt.GetAsInt(option.Timeout); t > 0 {
		timeout = t
	}

	// Load cookies
	var jar *cookiejar.Jar
	if cookieFile := opt.Get(option.LoadCookies); cookieFile != "" {
		var err error
		jar, err = LoadCookiesFromNetscape(cookieFile)
		if err != nil {
			fmt.Printf("Warning: failed to load cookies from %s: %v\n", cookieFile, err)
		}
	}

	if jar == nil {
		jar, _ = cookiejar.New(nil)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
		Jar:       jar,
	}
}
