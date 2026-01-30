package stats

import (
	"fmt"
)

// TransferStat holds transfer statistics
type TransferStat struct {
	TotalLength     int64
	CompletedLength int64
	DownloadSpeed   int
	UploadSpeed     int
	NumConnections  int
	HexGID          string
}

// String returns a formatted status string
// e.g., [DL: 2.5MiB 25% 1.2MiB/s]
func (t *TransferStat) String() string {
	percent := 0
	if t.TotalLength > 0 {
		percent = int(float64(t.CompletedLength) / float64(t.TotalLength) * 100)
	}

	return fmt.Sprintf("[DL:%s %d%% %s/s]",
		formatSize(t.CompletedLength),
		percent,
		formatSize(int64(t.DownloadSpeed)))
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
