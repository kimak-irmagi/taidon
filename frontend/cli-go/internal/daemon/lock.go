package daemon

import (
	"errors"
	"os"
	"time"
)

var ErrLockTimeout = errors.New("lock timeout")

type FileLock struct {
	file *os.File
}

func AcquireLock(path string, timeout time.Duration) (*FileLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	for {
		ok, err := tryLock(file)
		if err != nil {
			file.Close()
			return nil, err
		}
		if ok {
			return &FileLock{file: file}, nil
		}
		if timeout == 0 || time.Now().After(deadline) {
			file.Close()
			return nil, ErrLockTimeout
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (l *FileLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlock(l.file)
	closeErr := l.file.Close()
	if err != nil {
		return err
	}
	return closeErr
}
