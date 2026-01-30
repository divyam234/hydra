package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhunter/aria2go/pkg/option"
)

// Category 5: Edge Cases

func TestEdge_ZeroLengthFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_zero")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "zero.dat")

	rg := NewRequestGroup("zero-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "zero.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 0 {
		t.Errorf("Expected 0 bytes, got %d", len(content))
	}
}

func TestEdge_ExactPieceBoundary(t *testing.T) {
	// File size exactly 2 * pieceLength (assuming auto calculation or fixed)
	// Let's fix piece length to 1MB and file size 2MB
	// But minimal piece length is usually 1M.
	// Let's use small pieces via option if supported?
	// Auto calculator usually picks 1MB minimum.
	// Let's use 2MB file.

	size := 2 * 1024 * 1024
	data := make([]byte, size)
	server := setupRangeServer(t, data)
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_boundary")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.MinSplitSize, "1M") // default is 20M, so we need to lower it to split?
	opt.Put(option.Split, "4")

	rg := NewRequestGroup("boundary-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, "index.html"))
	if len(content) != size {
		t.Errorf("Size mismatch: %d", len(content))
	}
}

func TestEdge_LongFilename(t *testing.T) {
	// 200 chars filename
	longName := "long_filename_"
	for i := 0; i < 20; i++ {
		longName += "0123456789"
	}
	longName += ".dat"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_long")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, longName)

	rg := NewRequestGroup("long-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(tmpDir, longName))
	if err != nil {
		t.Error("Long filename file not found")
	}
}

func TestEdge_UnicodeFilename(t *testing.T) {
	name := "テスト.dat" // "Test.dat" in Japanese
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_unicode")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, name)

	rg := NewRequestGroup("unicode-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(tmpDir, name))
	if err != nil {
		t.Error("Unicode filename file not found")
	}
}

func TestEdge_SingleByteFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1")
		w.Write([]byte("x"))
	}))
	defer server.Close()

	tmpDir, _ := os.MkdirTemp("", "aria2go_single")
	defer os.RemoveAll(tmpDir)

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "single.dat")

	rg := NewRequestGroup("single-gid", []string{server.URL}, opt)
	err := rg.Execute(context.Background())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "single.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 1 {
		t.Errorf("Expected 1 byte, got %d", len(content))
	}
}
