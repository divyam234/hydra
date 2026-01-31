package http

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/divyam234/hydra/pkg/option"
)

// NewTransport creates a new HTTP transport with custom settings
func NewTransport(opt *option.Option) *http.Transport {
	// Proxy function logic
	noProxy := opt.Get(option.NoProxy)
	proxyStr := opt.Get(option.Proxy)

	proxyFunc := func(req *http.Request) (*url.URL, error) {
		// Check NoProxy
		if noProxy != "" {
			host := strings.ToLower(req.URL.Hostname())
			for _, domain := range strings.Split(noProxy, ",") {
				domain = strings.TrimSpace(strings.ToLower(domain))
				if domain == "" {
					continue
				}
				if domain == "*" {
					return nil, nil
				}
				if host == domain || strings.HasSuffix(host, "."+domain) {
					return nil, nil
				}
			}
		}

		// Single Proxy Flag
		if proxyStr != "" {
			if u, err := url.Parse(proxyStr); err == nil {
				return u, nil
			}
			// If parsing fails, maybe log warning? For now ignore.
		}

		// Fallback to environment
		return http.ProxyFromEnvironment(req)
	}

	// Default transport settings
	connectTimeout := 30
	if t, _ := opt.GetAsInt(option.ConnectTimeout); t > 0 {
		connectTimeout = t
	}

	// Network Tuning Options
	maxIdleConns := 1000
	if n, _ := opt.GetAsInt(option.MaxIdleConns); n > 0 {
		maxIdleConns = n
	}

	maxIdleConnsPerHost := 32
	if n, _ := opt.GetAsInt(option.MaxIdleConnsPerHost); n > 0 {
		maxIdleConnsPerHost = n
	}

	idleConnTimeout := 120
	if t, _ := opt.GetAsInt(option.IdleConnTimeout); t > 0 {
		idleConnTimeout = t
	}

	readBufferSize := 256 * 1024
	if s := opt.Get(option.ReadBufferSize); s != "" {
		if val, err := option.ParseUnitNumber(s); err == nil {
			readBufferSize = int(val)
		}
	}

	writeBufferSize := 64 * 1024
	if s := opt.Get(option.WriteBufferSize); s != "" {
		if val, err := option.ParseUnitNumber(s); err == nil {
			writeBufferSize = int(val)
		}
	}

	// TLS Configuration
	checkCert, err := opt.GetAsBool(option.CheckCertificate)
	if err != nil {
		checkCert = true // Default to safe
	}

	transport := &http.Transport{
		Proxy: proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(connectTimeout) * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !checkCert,
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdleConns,
		IdleConnTimeout:       time.Duration(idleConnTimeout) * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ReadBufferSize:        readBufferSize,
		WriteBufferSize:       writeBufferSize,
	}

	// Apply options
	if maxConnPerServer, _ := opt.GetAsInt(option.MaxConnPerServer); maxConnPerServer > 0 {
		// If explicit per-server limit is set, use it. Otherwise use the tuning value.
		transport.MaxIdleConnsPerHost = maxConnPerServer
		transport.MaxConnsPerHost = maxConnPerServer
	} else {
		transport.MaxIdleConnsPerHost = maxIdleConnsPerHost
		transport.MaxConnsPerHost = maxIdleConnsPerHost
	}

	keepAlive, _ := opt.GetAsBool(option.EnableHttpKeepAlive)
	transport.DisableKeepAlives = !keepAlive

	return transport
}

// NewClient creates a new HTTP client with custom transport
func NewClient(opt *option.Option) *http.Client {
	return NewClientWithTransport(NewTransport(opt), opt)
}

// NewClientWithTransport creates a new HTTP client using an existing transport
func NewClientWithTransport(transport *http.Transport, opt *option.Option) *http.Client {
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
