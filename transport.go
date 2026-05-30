package hydra

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

func newHTTPClient(o Options) (*http.Client, error) {
	if o.Transport != nil {
		return &http.Client{
			Transport: o.Transport,
			Timeout:   o.Timeout,
		}, nil
	}

	dialer := &net.Dialer{
		Timeout:   o.ConnectTimeout,
		KeepAlive: 30 * time.Second,
	}

	tr := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          max(100, o.MaxConnectionsPerServer*4),
		MaxIdleConnsPerHost:   o.MaxConnectionsPerServer,
		MaxConnsPerHost:       o.MaxConnectionsPerServer,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   o.ConnectTimeout,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    o.DisableCompression,
	}

	if o.Proxy != "" {
		pu, err := url.Parse(o.Proxy)
		if err != nil {
			return nil, err
		}

		switch pu.Scheme {
		case "http", "https", "socks5", "socks5h":
			tr.Proxy = http.ProxyURL(pu)
		default:
			return nil, fmt.Errorf("unsupported proxy scheme %q", pu.Scheme)
		}
	}

	return &http.Client{
		Transport: tr,
		Timeout:   o.Timeout,
	}, nil
}
