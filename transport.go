package hydra

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

type netDialerAdapter struct{ d *net.Dialer }

func (d netDialerAdapter) Dial(network, addr string) (net.Conn, error) {
	return d.d.Dial(network, addr)
}

func newHTTPClient(o Options) (*http.Client, error) {
	if o.Transport != nil {
		return &http.Client{Transport: o.Transport, Timeout: o.Timeout}, nil
	}
	dialer := &net.Dialer{Timeout: o.ConnectTimeout, KeepAlive: 30 * time.Second}
	tr := &http.Transport{
		Proxy:                 proxyFuncFromOptions(o),
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          max(100, o.MaxConnectionsPerServer*4),
		MaxIdleConnsPerHost:   o.MaxConnectionsPerServer,
		MaxConnsPerHost:       o.MaxConnectionsPerServer,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   o.ConnectTimeout,
		ExpectContinueTimeout: time.Second,
		DisableCompression:    o.DisableCompression,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	if o.Proxy != "" {
		pu, err := url.Parse(o.Proxy)
		if err != nil {
			return nil, err
		}
		switch pu.Scheme {
		case "socks5", "socks5h":
			dc, err := socksDialContext(pu, dialer)
			if err != nil {
				return nil, err
			}
			tr.Proxy = nil
			tr.DialContext = dc
		case "http", "https":
			tr.Proxy = http.ProxyURL(pu)
		default:
			return nil, errors.New("unsupported proxy scheme: " + pu.Scheme)
		}
	}
	return &http.Client{Transport: tr, Timeout: o.Timeout}, nil
}

func socksDialContext(u *url.URL, forward *net.Dialer) (func(context.Context, string, string) (net.Conn, error), error) {
	auth := (*proxy.Auth)(nil)
	if u.User != nil {
		pass, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: pass}
	}
	dialer, err := proxy.SOCKS5("tcp", u.Host, auth, netDialerAdapter{d: forward})
	if err != nil {
		return nil, err
	}
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		return cd.DialContext, nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		type result struct {
			c   net.Conn
			err error
		}
		ch := make(chan result, 1)
		go func() {
			c, err := dialer.Dial(network, addr)
			ch <- result{c: c, err: err}
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r := <-ch:
			if ctx.Err() != nil {
				if r.c != nil {
					_ = r.c.Close()
				}
				return nil, ctx.Err()
			}
			return r.c, r.err
		}
	}, nil
}

func proxyFuncFromOptions(o Options) func(*http.Request) (*url.URL, error) {
	if o.NoProxyFromEnvironment {
		return nil
	}
	return http.ProxyFromEnvironment
}
