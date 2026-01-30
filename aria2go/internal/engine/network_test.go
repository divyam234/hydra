package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	// Use small split to force multiple connections if possible, though slow server might serialize
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
		// Send some data then hijack/close
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
	// Depending on implementation, this might not error until checksum verification
	// or might error during read. Basic http client usually errors on UnexpectedEOF.
	if err == nil {
		// Check file size
		stat, _ := os.Stat(filepath.Join(tmpDir, "index.html")) // Default name from URL
		if stat.Size() == 100 {
			t.Fatal("File successfully created with full size despite missing data")
		}
	}
}
