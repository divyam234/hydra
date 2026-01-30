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

	"github.com/bhunter/aria2go/pkg/option"
)

// Category 1: Network Conditions

func TestDownload_SlowServer(t *testing.T) {
	// Server sends 1 byte every 10ms (approx 100 bytes/sec)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			w.Write([]byte{byte(i)})
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_slow")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "slow.dat")
	opt.Put(option.Split, "2")

	rg := NewRequestGroup("slow-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Slow download failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "slow.dat"))
	if len(content) != 100 {
		t.Errorf("Expected 100 bytes, got %d", len(content))
	}
}

func TestDownload_ConnectionReset(t *testing.T) {
	// Server closes connection mid-transfer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, 100))
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			conn.Close() // Hard reset
		}
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_reset")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.MaxTries, "2") // Should fail after retries
	opt.Put(option.RetryWait, "0")

	rg := NewRequestGroup("reset-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		t.Fatal("Expected error due to connection reset, got success")
	}
}

func TestDownload_Timeout(t *testing.T) {
	// Server hangs indefinitely
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than our test timeout
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, os.TempDir())

	rg := NewRequestGroup("timeout-gid", []string{server.URL}, opt)
	err := rg.Execute(ctx)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

func TestDownload_Redirect_301(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("success"))
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusMovedPermanently)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_redirect")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "redirect.dat")

	rg := NewRequestGroup("redirect-gid", []string{server.URL + "/redirect"}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Redirect failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "redirect.dat"))
	if string(content) != "success" {
		t.Errorf("Expected 'success', got '%s'", content)
	}
}

func TestDownload_ContentLength_Mismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Claim 100 bytes, send 50
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, 50))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_mismatch")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)

	rg := NewRequestGroup("mismatch-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		stat, _ := os.Stat(filepath.Join(tmpDir, "index.html"))
		if stat.Size() == 100 {
			t.Fatal("File successfully created with full size despite missing data")
		}
	}
}

func TestDownload_Redirect_Loop(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, os.TempDir())

	rg := NewRequestGroup("loop-gid", []string{server.URL + "/loop"}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		t.Error("Expected error for redirect loop")
	}
}

func TestDownload_DNS_Failure(t *testing.T) {
	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, os.TempDir())

	// Use invalid domain
	rg := NewRequestGroup("dns-gid", []string{"http://invalid.domain.test.local/file"}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		t.Error("Expected DNS error")
	}
}

func TestDownload_PartialContent_Mismatch(t *testing.T) {
	// Server claims to support ranges but returns 200 OK for Range request
	// This should trigger fallback to single connection, but if we forced multiple connections, it might be tricky.
	// Our client handles this by falling back.

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes") // Claim support
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			return
		}

		// If client asks for range, ignore it and send full 200
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK) // 200 OK, not 206
			w.Write(make([]byte, 100))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_partial_mismatch")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Split, "4") // Try to use multiple connections

	rg := NewRequestGroup("pmismatch-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Failed to handle partial content mismatch: %v", err)
	}

	// Should have succeeded via fallback
	content, _ := os.ReadFile(filepath.Join(tmpDir, "index.html"))
	if len(content) != 100 {
		t.Errorf("Expected 100 bytes, got %d", len(content))
	}
}

func TestDownload_SSL_Error(t *testing.T) {
	// HTTPS server with self-signed cert
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secure content"))
	}))
	defer server.Close()

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, os.TempDir())

	// Client should fail validation by default
	rg := NewRequestGroup("ssl-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		t.Error("Expected SSL error")
	}
}

func TestDownload_Redirect_Chain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/step1", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/step2", http.StatusFound)
	})
	mux.HandleFunc("/step2", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/step3", http.StatusFound)
	})
	mux.HandleFunc("/step3", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("chain complete"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_chain")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "chain.dat")

	rg := NewRequestGroup("chain-gid", []string{server.URL + "/step1"}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Redirect chain failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "chain.dat"))
	if string(content) != "chain complete" {
		t.Errorf("Expected 'chain complete', got '%s'", content)
	}
}

func TestError_ServerError_5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, os.TempDir())
	opt.Put(option.MaxTries, "1") // Don't retry forever

	rg := NewRequestGroup("5xx-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		t.Error("Expected 5xx error")
	} else if !strings.Contains(err.Error(), "503") {
		t.Errorf("Expected 503 error, got: %v", err)
	}
}
