package hydra

import (
	"sort"
	"sync"
	"time"
)

type mirrorMetric struct {
	Successes  int64
	Failures   int64
	Bytes      int64
	Duration   time.Duration
	LastUpdate time.Time
}

type mirrorScorer struct {
	mu      sync.RWMutex
	metrics map[string]mirrorMetric
}

func newMirrorScorer() *mirrorScorer {
	return &mirrorScorer{metrics: make(map[string]mirrorMetric)}
}

func (s *mirrorScorer) ordered(urls []string, salt int) []string {
	if len(urls) <= 1 {
		return append([]string(nil), urls...)
	}
	s.mu.RLock()
	out := append([]string(nil), urls...)
	metrics := make(map[string]mirrorMetric, len(out))
	for _, u := range out {
		metrics[u] = s.metrics[u]
	}
	s.mu.RUnlock()

	// Rotate first so equal/unobserved mirrors still receive fair exploration.
	out = rotated(out, salt)
	sort.SliceStable(out, func(i, j int) bool {
		return mirrorScore(metrics[out[i]]) > mirrorScore(metrics[out[j]])
	})
	return out
}

func mirrorScore(m mirrorMetric) float64 {
	if m.Successes == 0 && m.Failures == 0 {
		return 0
	}
	throughput := float64(0)
	if m.Duration > 0 && m.Bytes > 0 {
		throughput = float64(m.Bytes) / m.Duration.Seconds()
	}
	reliability := float64(m.Successes+1) / float64(m.Successes+m.Failures+2)
	return throughput*reliability - float64(m.Failures)*1024
}

func (s *mirrorScorer) observe(url string, bytes int64, elapsed time.Duration, err error) {
	if url == "" {
		return
	}
	s.mu.Lock()
	m := s.metrics[url]
	if err == nil {
		m.Successes++
		m.Bytes += max(int64(0), bytes)
		if elapsed > 0 {
			m.Duration += elapsed
		}
	} else {
		m.Failures++
	}
	m.LastUpdate = time.Now()
	s.metrics[url] = m
	s.mu.Unlock()
}
