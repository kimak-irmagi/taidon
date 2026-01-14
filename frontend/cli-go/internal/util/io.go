package util

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o700)
}

func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}

	temp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(tempName)
		return err
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return err
	}
	if err := os.Chmod(tempName, perm); err != nil {
		os.Remove(tempName)
		return err
	}
	return os.Rename(tempName, path)
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
