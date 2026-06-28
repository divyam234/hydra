package hydra

import (
	"fmt"
	"strconv"
	"strings"
)

type contentRange struct {
	Start int64
	End   int64
	Total int64
}

func parseContentRange(value string) (contentRange, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "bytes ") {
		return contentRange{}, fmt.Errorf("invalid content-range %q", value)
	}
	parts := strings.SplitN(strings.TrimPrefix(value, "bytes "), "/", 2)
	if len(parts) != 2 || parts[0] == "*" || parts[1] == "*" {
		return contentRange{}, fmt.Errorf("invalid content-range %q", value)
	}
	bounds := strings.SplitN(parts[0], "-", 2)
	if len(bounds) != 2 {
		return contentRange{}, fmt.Errorf("invalid content-range %q", value)
	}
	start, err := strconv.ParseInt(bounds[0], 10, 64)
	if err != nil || start < 0 {
		return contentRange{}, fmt.Errorf("invalid content-range %q", value)
	}
	end, err := strconv.ParseInt(bounds[1], 10, 64)
	if err != nil || end < start {
		return contentRange{}, fmt.Errorf("invalid content-range %q", value)
	}
	total, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || total <= end {
		return contentRange{}, fmt.Errorf("invalid content-range %q", value)
	}
	return contentRange{Start: start, End: end, Total: total}, nil
}

func validateContentRange(value string, wantStart, wantEnd, wantTotal int64) error {
	cr, err := parseContentRange(value)
	if err != nil {
		return err
	}
	if cr.Start != wantStart {
		return fmt.Errorf("unexpected content-range start: got %d want %d", cr.Start, wantStart)
	}
	if wantEnd >= 0 && cr.End != wantEnd {
		return fmt.Errorf("unexpected content-range end: got %d want %d", cr.End, wantEnd)
	}
	if wantTotal > 0 && cr.Total != wantTotal {
		return fmt.Errorf("unexpected content-range total: got %d want %d", cr.Total, wantTotal)
	}
	return nil
}
