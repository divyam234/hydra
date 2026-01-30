package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bhunter/hydra/pkg/option"
)

func TestOption_RateLimiting(t *testing.T) {
	// 500KB file to overcome burst token bucket effect
	// Limiter burst is set to limit size (50KB), so first 50KB is instant.
	// Remaining 450KB should take ~9s at 50KB/s.
	data := make([]byte, 500*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "512000")
		w.Write(data)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_limit")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.MaxDownloadLimit, "50K") // 50KB/s

	start := time.Now()
	rg := NewRequestGroup("limit-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	duration := time.Since(start)

	// Expected: ~9-10s
	// Assert it took at least 5s to be safe
	if duration < 5*time.Second {
		t.Errorf("Download too fast for 50K limit: %v (expected > 5s)", duration)
	}
}

func TestOption_CustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "FoundIt" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_headers")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Header, "X-Test: FoundIt")

	rg := NewRequestGroup("header-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Errorf("Custom header not sent: %v", err)
	}
}

func TestOption_UserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "MyBot/1.0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_ua")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.UserAgent, "MyBot/1.0")

	rg := NewRequestGroup("ua-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Errorf("User-Agent not sent: %v", err)
	}
}

func TestOption_LowestSpeedLimit(t *testing.T) {
	// Server sends data very slowly and supports Range
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "1000")

		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusPartialContent)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		// Send 1 byte every 100ms = 10 bytes/sec
		for i := 0; i < 100; i++ {
			time.Sleep(100 * time.Millisecond)
			w.Write([]byte{byte(i)})
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_lowspeed")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.LowestSpeedLimit, "1K")
	opt.Put(option.Split, "2")
	opt.Put(option.MaxTries, "1")

	rg := NewRequestGroup("lowspeed-gid", []string{server.URL}, opt)
	rg.speedCheckInterval = 200 * time.Millisecond

	err := rg.Execute(context.Background())
	if err == nil {
		t.Error("Expected error due to low speed")
	} else if !strings.Contains(err.Error(), "lowest limit") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestOption_Timeout(t *testing.T) {
	// Server accepts connection but hangs response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_timeout")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Timeout, "1") // 1 second read timeout

	rg := NewRequestGroup("timeout-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())

	// We expect a timeout error
	if err == nil {
		t.Error("Expected timeout, got success")
	} else {
		// Log the error to ensure it's a timeout
		t.Logf("Got expected error: %v", err)
	}
}

func TestOption_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "aladdin" || pass != "opensesame" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("access granted"))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_auth")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.HttpUser, "aladdin")
	opt.Put(option.HttpPasswd, "opensesame")

	rg := NewRequestGroup("auth-gid", []string{server.URL}, opt)
	if err := rg.Execute(context.Background()); err != nil {
		t.Errorf("Basic Auth failed: %v", err)
	}
}

func TestOption_Cookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil || cookie.Value != "xyz123" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte("cookie ok"))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_cookies")
	defer os.RemoveAll(tmpDir)

	cookieFile := filepath.Join(tmpDir, "cookies.txt")

	// Expiration needs to be valid future timestamp
	// 2147483647 is Jan 2038
	cookieContent := "127.0.0.1\tFALSE\t/\tFALSE\t2147483647\tsession_id\txyz123\n"
	os.WriteFile(cookieFile, []byte(cookieContent), 0644)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.LoadCookies, cookieFile)

	rg := NewRequestGroup("cookie-gid", []string{server.URL}, opt)
	if err := rg.Execute(context.Background()); err != nil {
		t.Errorf("Cookie test failed: %v", err)
	}
}

func TestOption_Proxy(t *testing.T) {
	// Proxy server
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple check that we are receiving the request
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("proxied response"))
	}))
	defer proxy.Close()

	// Target server (should NOT be reached directly if proxy works,
	// but the proxy we built just returns "proxied response", so we verify content)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("direct response"))
	}))
	defer target.Close()

	tmpDir, _ := os.MkdirTemp("", "hydra_proxy")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.AllProxy, proxy.URL)
	opt.Put(option.Out, "proxy.dat")

	rg := NewRequestGroup("proxy-gid", []string{target.URL}, opt)
	if err := rg.Execute(context.Background()); err != nil {
		t.Fatalf("Proxy test failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "proxy.dat"))
	if string(content) != "proxied response" {
		t.Errorf("Expected 'proxied response' from proxy, got '%s'", content)
	}
}
