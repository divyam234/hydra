package hydra

import (
	"errors"
	"fmt"
)

var (
	ErrServerNoRange = errors.New("server does not support byte ranges")
	ErrBadStatus     = errors.New("unexpected HTTP status")
)

type HTTPStatusError struct {
	URL        string
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string        { return fmt.Sprintf("%s: %s", e.URL, e.Status) }
func (e *HTTPStatusError) Is(target error) bool { return target == ErrBadStatus }
