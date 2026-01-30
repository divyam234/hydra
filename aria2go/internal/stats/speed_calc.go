package stats

import (
	"sync"
	"time"
)

// SpeedCalc calculates download/upload speed
type SpeedCalc struct {
	mu         sync.RWMutex
	bytes      []int
	lastTime   time.Time
	startTime  time.Time
	index      int
	totalBytes int64
	maxSpeed   int
}

const historySize = 10 // 10 seconds history

// NewSpeedCalc creates a new SpeedCalc
func NewSpeedCalc() *SpeedCalc {
	return &SpeedCalc{
		bytes:     make([]int, historySize),
		lastTime:  time.Now(),
		startTime: time.Now(),
	}
}

// Update adds bytes to the calculator
func (s *SpeedCalc) Update(bytes int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.totalBytes += int64(bytes)

	// Advance slots based on time elapsed
	diff := int(now.Sub(s.lastTime).Seconds())
	if diff > 0 {
		for i := 0; i < diff && i < historySize; i++ {
			s.index = (s.index + 1) % historySize
			s.bytes[s.index] = 0
		}
		s.lastTime = now
	}

	s.bytes[s.index] += bytes

	currentSpeed := s.calculateSpeedNoLock()
	if currentSpeed > s.maxSpeed {
		s.maxSpeed = currentSpeed
	}
}

// GetSpeed returns the current speed in bytes per second
func (s *SpeedCalc) GetSpeed() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.calculateSpeedNoLock()
}

func (s *SpeedCalc) calculateSpeedNoLock() int {
	// Simple average over history
	total := 0

	// Since we are not strictly ticking every second, this is an approximation.

	// A more robust implementation would sum up last N seconds of data.
	// For now, let's sum up the buffer and divide by history size (or elapsed time if shorter)

	elapsed := time.Since(s.startTime).Seconds()
	window := float64(historySize)
	if elapsed < window {
		window = elapsed
		if window < 1 {
			window = 1
		}
	}

	for _, b := range s.bytes {
		total += b
	}

	return int(float64(total) / window)
}

// GetTotalBytes returns total bytes transferred
func (s *SpeedCalc) GetTotalBytes() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalBytes
}

// GetAverageSpeed returns average speed since start
func (s *SpeedCalc) GetAverageSpeed() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	elapsed := time.Since(s.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return int(float64(s.totalBytes) / elapsed)
}
