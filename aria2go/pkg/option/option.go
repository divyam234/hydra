package option

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Option holds configuration values as key-value pairs
type Option struct {
	mu     sync.RWMutex
	values map[string]string
}

// NewOption creates a new Option instance
func NewOption() *Option {
	return &Option{
		values: make(map[string]string),
	}
}

// Put sets a configuration value
func (o *Option) Put(key, value string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.values[key] = value
}

// Get retrieves a configuration value, returning empty string if not found
func (o *Option) Get(key string) string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.values[key]
}

// GetAsInt retrieves a value as an integer
func (o *Option) GetAsInt(key string) (int, error) {
	val := o.Get(key)
	if val == "" {
		return 0, fmt.Errorf("option not found: %s", key)
	}
	return strconv.Atoi(val)
}

// GetAsBool retrieves a value as a boolean
func (o *Option) GetAsBool(key string) (bool, error) {
	val := o.Get(key)
	if val == "" {
		return false, fmt.Errorf("option not found: %s", key)
	}
	return strconv.ParseBool(val)
}

// GetAsDouble retrieves a value as a float64
func (o *Option) GetAsDouble(key string) (float64, error) {
	val := o.Get(key)
	if val == "" {
		return 0.0, fmt.Errorf("option not found: %s", key)
	}
	return strconv.ParseFloat(val, 64)
}

// Defined checks if a key exists
func (o *Option) Defined(key string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	_, ok := o.values[key]
	return ok
}

// Merge copies all values from another Option instance
func (o *Option) Merge(other *Option) {
	other.mu.RLock()
	defer other.mu.RUnlock()

	o.mu.Lock()
	defer o.mu.Unlock()

	for k, v := range other.values {
		o.values[k] = v
	}
}

// Clone creates a copy of the Option instance
func (o *Option) Clone() *Option {
	o.mu.RLock()
	defer o.mu.RUnlock()

	newOpt := NewOption()
	for k, v := range o.values {
		newOpt.values[k] = v
	}
	return newOpt
}

// ParseUnitNumber parses strings like "10M", "1k" into bytes
func ParseUnitNumber(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	lastChar := strings.ToUpper(s[len(s)-1:])
	multiplier := int64(1)

	numberPart := s
	if strings.ContainsAny(lastChar, "KMGTPE") {
		numberPart = s[:len(s)-1]
		switch lastChar {
		case "K":
			multiplier = 1024
		case "M":
			multiplier = 1024 * 1024
		case "G":
			multiplier = 1024 * 1024 * 1024
		case "T":
			multiplier = 1024 * 1024 * 1024 * 1024
		}
	}

	val, err := strconv.ParseInt(numberPart, 10, 64)
	if err != nil {
		return 0, err
	}

	return val * multiplier, nil
}
