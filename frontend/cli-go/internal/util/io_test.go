package util

import (
	"io"
	"strings"
	"testing"
)

func TestNDJSONReader(t *testing.T) {
	input := "{\"a\":1}\n\n{\"b\":2}"
	reader := NewNDJSONReader(strings.NewReader(input))

	line, err := reader.Next()
	if err != nil {
		t.Fatalf("read line 1: %v", err)
	}
	if string(line) != "{\"a\":1}" {
		t.Fatalf("unexpected line 1: %s", string(line))
	}

	line, err = reader.Next()
	if err != nil {
		t.Fatalf("read line 2: %v", err)
	}
	if string(line) != "" {
		t.Fatalf("expected empty line, got %q", string(line))
	}

	line, err = reader.Next()
	if err != io.EOF && err != nil {
		t.Fatalf("read line 3: %v", err)
	}
	if string(line) != "{\"b\":2}" {
		t.Fatalf("unexpected line 3: %s", string(line))
	}

	_, err = reader.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestNDJSONReaderLongLine(t *testing.T) {
	long := strings.Repeat("a", 200000)
	reader := NewNDJSONReader(strings.NewReader(long))

	line, err := reader.Next()
	if err != io.EOF && err != nil {
		t.Fatalf("read long line: %v", err)
	}
	if string(line) != long {
		t.Fatalf("unexpected long line length: %d", len(line))
	}
}
