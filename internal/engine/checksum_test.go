package engine

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/bhunter/hydra/pkg/option"
)

func TestChecksum_Validation(t *testing.T) {
	data := []byte("checksum test data")

	// Calculate hashes
	md5Hash := md5.Sum(data)
	sha1Hash := sha1.Sum(data)
	sha256Hash := sha256.Sum256(data)

	md5Str := hex.EncodeToString(md5Hash[:])
	sha1Str := hex.EncodeToString(sha1Hash[:])
	sha256Str := hex.EncodeToString(sha256Hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	defer server.Close()

	tests := []struct {
		name        string
		checksumOpt string
		shouldPass  bool
	}{
		{"MD5_Pass", "md5=" + md5Str, true},
		{"SHA1_Pass", "sha-1=" + sha1Str, true},
		{"SHA256_Pass", "sha-256=" + sha256Str, true},
		{"MD5_Fail", "md5=" + "00000000000000000000000000000000", false},
		{"SHA1_Fail", "sha-1=" + "0000000000000000000000000000000000000000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, _ := os.MkdirTemp("", "hydra_checksum")
			defer os.RemoveAll(tmpDir)

			opt := option.GetDefaultOptions()
			opt.Put(option.Dir, tmpDir)
			opt.Put(option.Checksum, tt.checksumOpt)
			opt.Put(option.Out, "test.dat")

			rg := NewRequestGroup(GID("checksum-"+tt.name), []string{server.URL}, opt)
			err := rg.Execute(context.Background())

			if tt.shouldPass {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected checksum failure, got success")
				} else if err.Error() != "checksum failed" {
					t.Errorf("Unexpected error message: %v", err)
				}
			}
		})
	}
}
