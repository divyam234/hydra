package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("HYDRA_CLI_TEST_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"hydra"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("hydra", flag.ExitOnError)
	main()
	os.Exit(0)
}

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestCLIHelperProcess$", "--"}, args...)...)
	cmd.Env = append(os.Environ(), "HYDRA_CLI_TEST_HELPER=1", "GORACE=atexit_sleep_ms=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("CLI did not finish: %v", ctx.Err())
	}
	return stdout.String(), stderr.String(), err
}

func TestCLIZeroRetriesMeansOneAttempt(t *testing.T) {
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "8")
			return
		}
		gets.Add(1)
		http.Error(w, "failure", http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, stderr, err := runCLI(t, "--quiet", "--no-env-proxy", "-s", "1", "--retry", "0", "--retry-wait", "1ms", "--max-retry-wait", "1ms", "-d", t.TempDir(), srv.URL)
	if err == nil {
		t.Fatal("failing origin returned success")
	}
	if n := gets.Load(); n != 1 {
		t.Fatalf("--retry 0 made %d attempts, want 1; stderr=%s", n, stderr)
	}
}

func TestCLIJSONFailuresRemainJSON(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	for _, extra := range [][]string{nil, {"--checksum", "sha256:zz"}} {
		args := append([]string{"--json-events", "--no-env-proxy", "-d", t.TempDir()}, extra...)
		_, stderr, err := runCLI(t, append(args, srv.URL)...)
		if err == nil {
			t.Fatal("expected CLI failure")
		}
		if strings.TrimSpace(stderr) == "" {
			t.Fatal("missing JSON failure event")
		}
		for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
			if !json.Valid([]byte(line)) {
				t.Errorf("non-JSON error output: %q", line)
			}
		}
	}
}

func TestCLIJSONSuccessChecksumAndCustomHeader(t *testing.T) {
	data := []byte("hydra CLI verified payload")
	var headers atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Hydra-Test") == "enabled" {
			headers.Add(1)
		}
		http.ServeContent(w, r, "source.bin", time.Time{}, bytes.NewReader(data))
	}))
	defer srv.Close()
	dir := t.TempDir()
	digest := sha256.Sum256(data)
	stdout, stderr, err := runCLI(t, "--json-events", "--no-env-proxy", "-s", "1", "-H", "X-Hydra-Test: enabled", "--checksum", fmt.Sprintf("sha256:%x", digest), "-d", dir, "--out", "custom.bin", srv.URL)
	if err != nil {
		t.Fatalf("CLI error=%v stderr=%s", err, stderr)
	}
	path := filepath.Join(dir, "custom.bin")
	if strings.TrimSpace(stdout) != path {
		t.Fatalf("output path=%q want=%q", stdout, path)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) || headers.Load() < 2 {
		t.Fatalf("CLI output/header mismatch: headers=%d err=%v", headers.Load(), err)
	}
	complete, verifying := false, false
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		var event struct{ Kind string }
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("invalid JSON event: %s", line)
		}
		complete = complete || event.Kind == "completed"
		verifying = verifying || event.Kind == "verifying"
	}
	if !complete || !verifying {
		t.Fatalf("missing verified completion: %s", stderr)
	}
}

func TestCLICtrlCReportsCancellationOnce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt cannot be sent to a Windows process")
	}
	for _, scenario := range []struct {
		name  string
		split string
		extra []string
	}{
		{"single", "1", nil},
		{"segmented", "4", nil},
		{"json", "4", []string{"--json-events"}},
		{"quiet", "4", []string{"--quiet"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ready := make(chan struct{})
			var once sync.Once
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				const size = 4 << 20
				start, end := int64(0), int64(size-1)
				if value := r.Header.Get("Range"); value != "" {
					if _, err := fmt.Sscanf(value, "bytes=%d-%d", &start, &end); err != nil {
						t.Errorf("invalid test range: %q", value)
						return
					}
				}
				w.Header().Set("ETag", `"ctrl-c-object"`)
				w.Header().Set("Accept-Ranges", "bytes")
				w.Header().Set("Content-Length", fmt.Sprint(end-start+1))
				if r.Method == http.MethodHead {
					return
				}
				if r.Header.Get("Range") != "" {
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
					w.WriteHeader(http.StatusPartialContent)
				}
				_, _ = w.Write(bytes.Repeat([]byte("x"), 32<<10))
				w.(http.Flusher).Flush()
				once.Do(func() { close(ready) })
				<-r.Context().Done()
			}))
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			args := []string{"-test.run=^TestCLIHelperProcess$", "--", "--no-env-proxy", "-s", scenario.split, "-x", scenario.split, "-k", "32K", "-d", t.TempDir()}
			args = append(args, scenario.extra...)
			cmd := exec.CommandContext(ctx, os.Args[0], append(args, srv.URL)...)
			cmd.Env = append(os.Environ(), "HYDRA_CLI_TEST_HELPER=1", "GORACE=atexit_sleep_ms=0")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Process.Kill()
			select {
			case <-ready:
			case <-ctx.Done():
				_ = cmd.Wait()
				t.Fatal("CLI never started downloading")
			}
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			err := cmd.Wait()
			if ctx.Err() != nil || err == nil {
				t.Fatalf("cancellation exit=%v timeout=%v stderr=%s", err, ctx.Err(), stderr.String())
			}
			output := stderr.String()
			if scenario.name == "quiet" {
				if strings.TrimSpace(output) != "" {
					t.Fatalf("quiet cancellation printed redundant output: %s", output)
				}
				return
			}
			terminal := 0
			for _, line := range strings.Split(strings.ReplaceAll(output, "\r", "\n"), "\n") {
				if scenario.name == "json" {
					if strings.TrimSpace(line) == "" {
						continue
					}
					var event struct{ Kind string }
					if err := json.Unmarshal([]byte(line), &event); err != nil {
						t.Fatalf("non-JSON cancellation output: %q", line)
					}
					if event.Kind == "failed" {
						t.Fatalf("Ctrl+C reported as failure: %s", line)
					}
					if event.Kind == "cancelled" {
						terminal++
					}
				} else if fields := strings.Fields(line); len(fields) > 0 && fields[0] == "cancelled" {
					terminal++
				}
			}
			if terminal != 1 || strings.Contains(output, "error: context canceled") {
				t.Fatalf("expected one cancellation report without duplicate error: %s", output)
			}
		})
	}
}
