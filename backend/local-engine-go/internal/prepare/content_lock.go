package prepare

import (
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

type contentLock struct {
	files map[string]*os.File
	order []string
}

var (
	lockFileSharedFn   = lockFileShared
	unlockFileSharedFn = unlockFileShared
	readAllFn          = io.ReadAll
)

func lockContentFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if err := lockFileSharedFn(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func lockContentFiles(paths []string) (*contentLock, error) {
	uniq := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		uniq[path] = struct{}{}
	}
	if len(uniq) == 0 {
		return &contentLock{files: map[string]*os.File{}}, nil
	}
	order := make([]string, 0, len(uniq))
	for path := range uniq {
		order = append(order, path)
	}
	sort.Strings(order)
	locks := &contentLock{
		files: make(map[string]*os.File, len(order)),
		order: order,
	}
	for _, path := range order {
		f, err := lockContentFile(path)
		if err != nil {
			_ = locks.Close()
			return nil, err
		}
		locks.files[path] = f
	}
	return locks, nil
}

func (c *contentLock) Close() error {
	if c == nil {
		return nil
	}
	var outErr error
	for _, path := range c.order {
		f := c.files[path]
		if f == nil {
			continue
		}
		if err := unlockFileSharedFn(f); err != nil && outErr == nil {
			outErr = err
		}
		if err := f.Close(); err != nil && outErr == nil {
			outErr = err
		}
	}
	return outErr
}

func (c *contentLock) readFile(path string) ([]byte, error) {
	if c == nil {
		return os.ReadFile(path)
	}
	f := c.files[path]
	if f == nil {
		return os.ReadFile(path)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	data, err := readAllFn(f)
	if errors.Is(err, os.ErrClosed) {
		return os.ReadFile(path)
	}
	return data, err
}
