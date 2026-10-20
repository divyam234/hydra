package hydra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Use genuinely different bytes, not the repeating testData pattern: a wrong
// block/version must change the independent final digest.
func integrityData(version byte) []byte {
	data := make([]byte, 512<<10)
	for off := 0; off < len(data); off += sha256.Size {
		h := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", version, off)))
		copy(data[off:], h[:])
	}
	return data
}

func seedIntegrityResume(t *testing.T, path, raw string, data []byte, etag string) {
	t.Helper()
	partial := make([]byte, len(data))
	copy(partial, data[:64<<10])
	if err := os.WriteFile(path, partial, 0600); err != nil {
		t.Fatal(err)
	}
	m := newMeta([]string{raw}, path, probeInfo{URL: raw, Size: int64(len(data)), ETag: etag}, 64<<10)
	if err := m.markDone(0, sidecarPath(path)); err != nil {
		t.Fatal(err)
	}
}

func seedSingleCheckpoint(t *testing.T, path, raw string, size int) {
	t.Helper()
	if err := saveSingleResume(partPath(path), path, []string{raw}, probeInfo{Size: int64(size), ETag: `"test-object"`}); err != nil {
		t.Fatal(err)
	}
}

func TestSingleResumeNeverTrustsDamagedOrUnidentifiedPrefix(t *testing.T) {
	for _, scenario := range []string{"bare-part", "damaged-prefix", "changed-etag", "no-validator"} {
		t.Run(scenario, func(t *testing.T) {
			old, current := integrityData(1), integrityData(2)
			etag := `"test-object"`
			if scenario == "no-validator" {
				etag = ""
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if etag != "" {
					w.Header().Set("ETag", etag)
				}
				http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(current))
			}))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "file")
			prefix := current[:64<<10]
			if scenario != "damaged-prefix" {
				prefix = old[:64<<10]
			}
			if err := os.WriteFile(partPath(path), prefix, 0600); err != nil {
				t.Fatal(err)
			}
			if scenario != "bare-part" {
				info := probeInfo{Size: int64(len(current)), ETag: etag}
				if scenario == "changed-etag" {
					info.ETag = `"old-object"`
				}
				if err := saveSingleResume(partPath(path), path, []string{srv.URL}, info); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "damaged-prefix" {
				if err := os.WriteFile(partPath(path), old[:64<<10], 0600); err != nil {
					t.Fatal(err)
				}
			}
			r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 1})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if r.Resumed || sha256.Sum256(got) != sha256.Sum256(current) {
				t.Fatal("unsafe prefix was reused")
			}
		})
	}
}

func TestResumeChangedSameSizeObjectNeverMixesVersions(t *testing.T) {
	old, current := integrityData(1), integrityData(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"version-2"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(current))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	seedIntegrityResume(t, path, srv.URL, old, `"version-1"`)
	r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 4, MinSplitSize: 1, ResumeBlockSize: 64 << 10})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != sha256.Sum256(current) {
		t.Fatalf("SUCCESS WITH CORRUPTION: resumed=%v; mixed old/new bytes", r.Resumed)
	}
	if r.Resumed {
		t.Fatal("reused bytes from a changed object")
	}
}

func TestRangesRejectVersionChangeAfterProbe(t *testing.T) {
	current := integrityData(2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		etag := `"version-1"`
		if r.Method != http.MethodHead {
			etag = `"version-2"`
		}
		// Deliberately ignore conditional headers: response validation must also
		// catch a broken origin returning a conflicting validator.
		w.Header().Set("ETag", etag)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(current))
	}))
	defer srv.Close()
	_, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 4, MinSplitSize: 1, ResumeBlockSize: 64 << 10})
	if err == nil {
		t.Fatal("accepted changed range representation after probe")
	}
}

func TestResumeRepairsSameSizeDamagedCompletedBlock(t *testing.T) {
	data := integrityData(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"version-1"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	path := filepath.Join(t.TempDir(), "file")
	seedIntegrityResume(t, path, srv.URL, data, `"version-1"`)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteAt([]byte{data[100] ^ 0xff}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 4, MinSplitSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(got) != sha256.Sum256(data) {
		t.Fatal("damaged block was not repaired")
	}
}

func TestResumeRejectsUnauthenticatedCompletionBits(t *testing.T) {
	data := integrityData(1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"version-1"`)
		http.ServeContent(w, r, "file", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	for _, scenario := range []string{"missing-hashes", "wrong-hashes", "legacy-v2", "flipped-bit", "replaced-output"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "file")
			seedIntegrityResume(t, path, srv.URL, data, `"version-1"`)
			b, err := os.ReadFile(sidecarPath(path))
			if err != nil {
				t.Fatal(err)
			}
			var meta map[string]any
			if err := json.Unmarshal(b, &meta); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "missing-hashes":
				delete(meta, "block_hashes")
			case "wrong-hashes":
				meta["block_hashes"] = base64.StdEncoding.EncodeToString(make([]byte, 8*sha256.Size))
			case "legacy-v2":
				meta["version"] = 2
				delete(meta, "block_hashes")
			case "flipped-bit":
				meta["bitfield"] = base64.StdEncoding.EncodeToString([]byte{3})
			case "replaced-output":
				if err := os.WriteFile(path, make([]byte, len(data)), 0600); err != nil {
					t.Fatal(err)
				}
			}
			b, err = json.Marshal(meta)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sidecarPath(path), b, 0600); err != nil {
				t.Fatal(err)
			}
			_, err = Download(context.Background(), Request{URLs: []string{srv.URL}, Out: path}, Options{Split: 4, MinSplitSize: 1})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if sha256.Sum256(got) != sha256.Sum256(data) {
				t.Fatal("unauthenticated bytes were published")
			}
		})
	}
}

func TestUnversionedDownloadsDoNotCombineResponses(t *testing.T) {
	for _, validator := range []string{"none", "weak-etag", "last-modified-only"} {
		t.Run(validator, func(t *testing.T) {
			data := integrityData(1)
			var gets atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if validator == "weak-etag" {
					w.Header().Set("ETag", `W/"not-byte-exact"`)
				}
				mod := time.Time{}
				if validator == "last-modified-only" {
					mod = time.Unix(1_700_000_000, 0)
				}
				response := data
				if r.Method != http.MethodHead && gets.Add(1) > 1 {
					response = integrityData(2)
				}
				http.ServeContent(w, r, "file", mod, bytes.NewReader(response))
			}))
			defer srv.Close()
			r, err := Download(context.Background(), Request{URLs: []string{srv.URL}, Dir: t.TempDir()}, Options{Split: 8, MinSplitSize: 1})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(r.Path)
			if err != nil {
				t.Fatal(err)
			}
			if r.Connections != 1 || gets.Load() != 1 || sha256.Sum256(got) != sha256.Sum256(data) {
				t.Fatal("combined unversioned responses")
			}
		})
	}
}
