package hydra

import (
	"fmt"
	"os"
)

type fileLock struct{ path string }

func acquireFileLock(target string) (*fileLock, error) {
	lock := target + ".hydra.lock"
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("download lock exists for %s; remove %s if no downloader is running", target, lock)
		}
		return nil, err
	}
	_, _ = fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	_ = f.Close()
	return &fileLock{path: lock}, nil
}

func (l *fileLock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	return removeIfExists(l.path)
}
