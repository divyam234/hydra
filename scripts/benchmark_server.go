// HTTP range test server for benchmark_three_way.py.
// Provides per-request rate limiting and an optional slow range.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type counters struct {
	gets, heads, bytes, active, peak atomic.Int64
}

func (c *counters) reset() {
	c.gets.Store(0)
	c.heads.Store(0)
	c.bytes.Store(0)
	c.active.Store(0)
	c.peak.Store(0)
}

func (c *counters) enter() {
	n := c.active.Add(1)
	for {
		old := c.peak.Load()
		if n <= old || c.peak.CompareAndSwap(old, n) {
			return
		}
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "loopback address (port 0 selects a free port)")
	path := flag.String("file", "", "immutable payload file")
	rate := flag.Int64("rate", 8*1024*1024, "per-GET bytes/second (0 for unlimited)")
	slowStart := flag.Int64("slow-start", -1, "range start to throttle further (-1 disables)")
	slowRate := flag.Int64("slow-rate", 2*1024*1024, "bytes/second for selected slow-start range")
	delay := flag.Duration("latency", 40*time.Millisecond, "artificial latency per request")
	flag.Parse()
	if *path == "" {
		log.Fatal("-file is required")
	}
	f, err := os.Open(*path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		log.Fatal(err)
	}
	size := info.Size()
	var stats counters
	mux := http.NewServeMux()

	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int64{
			"gets": stats.gets.Load(), "heads": stats.heads.Load(),
			"bytes": stats.bytes.Load(), "active": stats.active.Load(),
			"peak_gets": stats.peak.Load(),
		})
	})
	mux.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		stats.reset()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/test.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if *delay > 0 {
			time.Sleep(*delay)
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("ETag", `"bench-fixed-zeros-v1"`)
		w.Header().Set("Last-Modified", "Wed, 18 Nov 2026 00:00:00 GMT")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="test.bin"`)
		start, end := int64(0), size-1
		value := r.Header.Get("Range")
		if value != "" {
			if !strings.HasPrefix(value, "bytes=") {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			fields := strings.SplitN(strings.TrimPrefix(value, "bytes="), "-", 2)
			if len(fields) != 2 {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			parsedStart, parseErr := strconv.ParseInt(fields[0], 10, 64)
			if parseErr != nil {
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			start = parsedStart
			if fields[1] != "" {
				parsedEnd, parseErr := strconv.ParseInt(fields[1], 10, 64)
				if parseErr != nil {
					http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
					return
				}
				end = parsedEnd
			}
			if start >= size || start < 0 || end < start {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
				http.Error(w, "bad range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if end >= size {
				end = size - 1
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		}
		count := end - start + 1
		w.Header().Set("Content-Length", strconv.FormatInt(count, 10))
		if value != "" {
			w.WriteHeader(http.StatusPartialContent)
		}
		if r.Method == http.MethodHead {
			stats.heads.Add(1)
			return
		}
		stats.gets.Add(1)
		stats.enter()
		defer stats.active.Add(-1)
		src := io.NewSectionReader(f, start, count)
		buf := make([]byte, 256*1024)
		began := time.Now()
		currentRate := *rate
		if *slowStart >= 0 && start == *slowStart {
			currentRate = *slowRate
		}
		var sent int64
		for {
			n, readErr := src.Read(buf)
			if n > 0 {
				written, writeErr := w.Write(buf[:n])
				sent += int64(written)
				stats.bytes.Add(int64(written))
				if writeErr != nil {
					return
				}
				if currentRate > 0 {
					deadline := began.Add(time.Duration(float64(sent) / float64(currentRate) * float64(time.Second)))
					if wait := time.Until(deadline); wait > 0 {
						time.Sleep(wait)
					}
				}
			}
			if readErr != nil {
				return
			}
		}
	})

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	// The benchmark harness reads this line to discover the assigned port.
	fmt.Printf("READY %s\n", listener.Addr().String())
	log.Fatal(http.Serve(listener, mux))
}
