package runtime

import (
	"context"
	"testing"
)

func TestLogSinkContext(t *testing.T) {
	if logSinkFromContext(nil) != nil {
		t.Fatalf("expected nil sink for nil context")
	}
	if logSinkFromContext(context.Background()) != nil {
		t.Fatalf("expected nil sink for empty context")
	}
	base := context.Background()
	if WithLogSink(base, nil) != base {
		t.Fatalf("expected WithLogSink to return base context for nil sink")
	}
	ctx := WithLogSink(context.Background(), func(string) {})
	if logSinkFromContext(ctx) == nil {
		t.Fatalf("expected sink from context")
	}
	if LogSinkFromContext(ctx) == nil {
		t.Fatalf("expected exported sink from context")
	}
}
