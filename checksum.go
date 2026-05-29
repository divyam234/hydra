package hydra

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

var ErrChecksumMismatch = errors.New("checksum mismatch")

// Checksum describes an expected digest for the final file. Supported
// algorithms: sha256, sha512, sha1, md5.
type Checksum struct {
	Algorithm string `json:"algorithm,omitempty"`
	Value     string `json:"value,omitempty"`
}

func ParseChecksum(s string) (Checksum, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Checksum{}, nil
	}
	alg, val, ok := strings.Cut(s, ":")
	if !ok {
		return Checksum{}, fmt.Errorf("checksum must be algorithm:hex")
	}
	c := Checksum{Algorithm: strings.ToLower(strings.TrimSpace(alg)), Value: strings.ToLower(strings.TrimSpace(val))}
	if c.Algorithm == "" || c.Value == "" {
		return Checksum{}, fmt.Errorf("checksum must be algorithm:hex")
	}
	if _, err := c.newHash(); err != nil {
		return Checksum{}, err
	}
	if _, err := hex.DecodeString(c.Value); err != nil {
		return Checksum{}, fmt.Errorf("checksum value is not hex: %w", err)
	}
	return c, nil
}

func (c Checksum) Empty() bool {
	return strings.TrimSpace(c.Algorithm) == "" && strings.TrimSpace(c.Value) == ""
}

func (c Checksum) normalized() Checksum {
	c.Algorithm = strings.ToLower(strings.TrimSpace(c.Algorithm))
	c.Value = strings.ToLower(strings.TrimSpace(c.Value))
	return c
}

func (c Checksum) newHash() (hash.Hash, error) {
	switch strings.ToLower(strings.TrimSpace(c.Algorithm)) {
	case "sha256", "sha-256":
		return sha256.New(), nil
	case "sha512", "sha-512":
		return sha512.New(), nil
	case "sha1", "sha-1":
		return sha1.New(), nil
	case "md5":
		return md5.New(), nil
	default:
		return nil, fmt.Errorf("unsupported checksum algorithm: %s", c.Algorithm)
	}
}

func (c Checksum) VerifyFile(path string) (actual string, err error) {
	c = c.normalized()
	if c.Empty() {
		return "", nil
	}
	h, err := c.newHash()
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	actual = hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, c.Value) {
		return actual, fmt.Errorf("%w: %s expected %s actual %s", ErrChecksumMismatch, c.Algorithm, c.Value, actual)
	}
	return actual, nil
}
