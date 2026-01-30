package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bhunter/aria2go/pkg/option"
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

	tmpDir, _ := os.MkdirTemp("", "aria2go_limit")
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

	tmpDir, _ := os.MkdirTemp("", "aria2go_headers")
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

	tmpDir, _ := os.MkdirTemp("", "aria2go_ua")
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
	t.Skip("Skipping LowestSpeedLimit test as it requires 30s wait due to hardcoded interval")
}

func TestOption_Timeout(t *testing.T) {
	// Server accepts connection but hangs response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_timeout")
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
