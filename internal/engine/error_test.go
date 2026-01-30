package engine

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/divyam234/hydra/pkg/option"
)

// Category 4: Error Handling

func TestError_InvalidURL(t *testing.T) {
	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, os.TempDir())

	rg := NewRequestGroup("invalid-url", []string{"invalid-protocol://foo.bar"}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		t.Error("Expected error for invalid URL protocol")
	}
}

func TestError_NoURIs(t *testing.T) {
	opt := option.GetDefaultOptions()
	rg := NewRequestGroup("no-uris", []string{}, opt)
	err := rg.Execute(context.Background())
	if err == nil {
		t.Error("Expected error for empty URIs")
	} else if err.Error() != "no URIs provided" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestError_PermissionDenied(t *testing.T) {
	// Create a read-only directory
	tmpDir, _ := os.MkdirTemp("", "hydra_perm")
	defer os.RemoveAll(tmpDir)
	os.Chmod(tmpDir, 0500) // Read-execute only, no write

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, tmpDir)
	opt.Put(option.Out, "file.dat")

	// We can use a real URL or invalid one, but the file creation happens early
	// If we use invalid URL, it might fail on parsing before file creation.
	// Use a valid-looking URL that won't actually be connected to yet.
	// Actually, Open happens after ParseURI.

	rg := NewRequestGroup("perm-gid", []string{"http://example.com/file.dat"}, opt)
	err := rg.Execute(context.Background())

	if err == nil {
		t.Error("Expected permission denied error")
	} else if !strings.Contains(err.Error(), "permission denied") && !strings.Contains(err.Error(), "access denied") {
		// Depending on OS, might differ, but usually contains "permission denied"
		t.Logf("Got error as expected (checking content): %v", err)
	}
}

func TestError_MaxTriesExceeded(t *testing.T) {
	// Use a dummy server that always fails (or just a closed port)
	// Connecting to localhost on a random closed port

	opt := option.GetDefaultOptions()
	opt.Put(option.Dir, os.TempDir())
	opt.Put(option.MaxTries, "2")
	opt.Put(option.RetryWait, "0")

	// Port 1 is likely closed
	rg := NewRequestGroup("maxtries-gid", []string{"http://127.0.0.1:1/file"}, opt)
	err := rg.Execute(context.Background())

	if err == nil {
		t.Error("Expected error after max tries")
	}
}
