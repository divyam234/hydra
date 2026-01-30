package util

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// VerifyChecksum verifies the file against the checksum string
// Format: "sha-1=digest" or just "digest" (implies sha-1 or auto-detected)
func VerifyChecksum(filePath string, checksumStr string) (bool, error) {
	if checksumStr == "" {
		return true, nil
	}

	parts := strings.SplitN(checksumStr, "=", 2)
	var algo string
	var expected string

	if len(parts) == 2 {
		algo = strings.ToLower(parts[0])
		expected = strings.ToLower(parts[1])
	} else {
		// Auto-detect based on length
		expected = strings.ToLower(parts[0])
		switch len(expected) {
		case 32:
			algo = "md5"
		case 40:
			algo = "sha-1"
		case 64:
			algo = "sha-256"
		default:
			return false, fmt.Errorf("unknown checksum type for length %d", len(expected))
		}
	}

	var h hash.Hash
	switch algo {
	case "md5":
		h = md5.New()
	case "sha-1", "sha1":
		h = sha1.New()
	case "sha-256", "sha256":
		h = sha256.New()
	default:
		return false, fmt.Errorf("unsupported checksum algorithm: %s", algo)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	return actual == expected, nil
}
