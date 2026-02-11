package util

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

var (
	ensureDirFn  = EnsureDir
	createTempFn = os.CreateTemp
	writeFileFn  = func(f *os.File, data []byte) (int, error) { return f.Write(data) }
	closeFileFn  = func(f *os.File) error { return f.Close() }
	chmodFn      = os.Chmod
	renameFn     = os.Rename
	removeFn     = os.Remove
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := ensureDirFn(dir); err != nil {
		return err
	}

	temp, err := createTempFn(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	if _, err := writeFileFn(temp, data); err != nil {
		_ = closeFileFn(temp)
		_ = removeFn(tempName)
		return err
	}
	if err := closeFileFn(temp); err != nil {
		_ = removeFn(tempName)
		return err
	}
	if err := chmodFn(tempName, perm); err != nil {
		_ = removeFn(tempName)
		return err
	}
	return renameFn(tempName, path)
}

type NDJSONReader struct {
	reader *bufio.Reader
}

func NewNDJSONReader(r io.Reader) *NDJSONReader {
	return &NDJSONReader{reader: bufio.NewReader(r)}
}

func (n *NDJSONReader) Next() ([]byte, error) {
	line, err := n.reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	if err == io.EOF && len(line) == 0 {
		return nil, io.EOF
	}

	trimmed := bytes.TrimRight(line, "\r\n")
	return trimmed, err
}
