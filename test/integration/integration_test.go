package integration_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	hydraBinary string
)

func TestMain(m *testing.M) {
	// Build hydra binary
	tmpDir, err := os.MkdirTemp("", "hydra-test-build")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	hydraBinary = filepath.Join(tmpDir, "hydra")

	// We are in test/integration, we need to go up to root
	rootDir := "../../"
	cmd := exec.Command("go", "build", "-o", hydraBinary, "./cmd/hydra")
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build hydra: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestInputFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("content"))
	}))
	defer ts.Close()

	// Create input file
	tmpFile, err := os.CreateTemp("", "hydra-input")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Write 2 URLs
	file1 := ts.URL + "/file1"
	file2 := ts.URL + "/file2"
	tmpFile.WriteString(file1 + "\n" + file2 + "\n")
	tmpFile.Close()

	// Run hydra
	outDir, err := os.MkdirTemp("", "hydra-out")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outDir)

	cmd := exec.Command(hydraBinary, "download", "-i", tmpFile.Name(), "-d", outDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Hydra failed: %v\nOutput: %s", err, output)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(outDir, "file1")); os.IsNotExist(err) {
		t.Error("file1 not downloaded")
	}
	if _, err := os.Stat(filepath.Join(outDir, "file2")); os.IsNotExist(err) {
		t.Error("file2 not downloaded")
	}
}

func TestForceSequential(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("content"))
	}))
	defer ts.Close()

	outDir, err := os.MkdirTemp("", "hydra-out-seq")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outDir)

	// -Z with multiple args
	file1 := ts.URL + "/seq1"
	file2 := ts.URL + "/seq2"

	cmd := exec.Command(hydraBinary, "download", "-Z", "-d", outDir, file1, file2)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Hydra failed: %v\nOutput: %s", err, output)
	}

	if _, err := os.Stat(filepath.Join(outDir, "seq1")); os.IsNotExist(err) {
		t.Error("seq1 not downloaded")
	}
	if _, err := os.Stat(filepath.Join(outDir, "seq2")); os.IsNotExist(err) {
		t.Error("seq2 not downloaded")
	}
}

func TestQuietMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("content"))
	}))
	defer ts.Close()

	outDir, err := os.MkdirTemp("", "hydra-out-quiet")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outDir)

	cmd := exec.Command(hydraBinary, "download", "-q", "-d", outDir, ts.URL+"/quiet_file")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Hydra failed: %v\nOutput: %s", err, output)
	}

	// Output should be minimal (maybe just "Download complete" if that's printed outside console?
	// Console.Printf respects quiet, but fmt.Println in main might not?
	// Let's check main.go again.
	// main.go uses fmt.Println("Download complete.") which is NOT guarded by quiet flag logic in Console.
	// But console progress bars should be gone.

	if strings.Contains(string(output), "[DL:") {
		t.Error("Output contains progress bar in quiet mode")
	}
}

func TestInsecure(t *testing.T) {
	// Start HTTPS server with self-signed cert
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("secure content"))
	}))
	defer ts.Close()

	outDir, err := os.MkdirTemp("", "hydra-out-secure")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outDir)

	// 1. Without -k (should fail)
	// We need to make sure the client doesn't trust the cert by default.
	// Go's httptest server certs are trusted by the client provided by ts.Client(),
	// but external processes won't trust them unless we set up CA pool.
	// So plain 'hydra download' should fail.

	cmd := exec.Command(hydraBinary, "download", "-d", outDir, ts.URL+"/fail")
	err = cmd.Run()
	if err == nil {
		t.Error("Expected failure without -k for self-signed cert")
	}

	// 2. With -k (should succeed)
	cmd = exec.Command(hydraBinary, "download", "-k", "-d", outDir, ts.URL+"/success")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Hydra failed with -k: %v\nOutput: %s", err, output)
	}

	if _, err := os.Stat(filepath.Join(outDir, "success")); os.IsNotExist(err) {
		t.Error("File not downloaded with -k")
	}
}

func TestProxy(t *testing.T) {
	// 1. Start a dummy proxy server
	proxyCalled := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalled = true
		// Handle CONNECT (HTTPS) or plain GET (HTTP)
		if r.Method == http.MethodConnect {
			// simplified CONNECT handling
			destConn, err := net.Dial("tcp", r.Host)
			if err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				return
			}
			clientConn, _, err := hijacker.Hijack()
			if err != nil {
				return
			}
			go transfer(destConn, clientConn)
			go transfer(clientConn, destConn)
		} else {
			// HTTP Proxy
			resp, err := http.Get(r.URL.String())
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			io.Copy(w, resp.Body)
		}
	}))
	defer proxy.Close()

	// 2. Start Target Server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("proxy content"))
	}))
	defer ts.Close()

	outDir, err := os.MkdirTemp("", "hydra-out-proxy")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outDir)

	// 3. Run with --proxy
	// Note: We use HTTP target to keep proxy simple
	cmd := exec.Command(hydraBinary, "download", "--proxy", proxy.URL, "-d", outDir, ts.URL+"/proxyfile")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Hydra failed with proxy: %v\nOutput: %s", err, output)
	}

	if !proxyCalled {
		t.Error("Proxy was not used")
	}
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}
