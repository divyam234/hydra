package hydra

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrServerNoRange       = errors.New("server does not support byte ranges")
	ErrBadStatus           = errors.New("unexpected HTTP status")
	ErrInvalidContentRange = errors.New("invalid content-range")
	ErrRemoteChanged       = errors.New("remote representation changed")
)

type HTTPStatusError struct {
	URL        string
	StatusCode int
	Status     string
	RetryAfter time.Duration
}

func (e *HTTPStatusError) Error() string        { return fmt.Sprintf("%s: %s", e.URL, e.Status) }
func (e *HTTPStatusError) Is(target error) bool { return target == ErrBadStatus }
