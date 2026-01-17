package conntrack

import (
	"context"
	"testing"
)

func TestNoopReturnsZeroConnections(t *testing.T) {
	count, err := Noop{}.ActiveConnections(context.Background(), "instance")
	if err != nil {
		t.Fatalf("ActiveConnections: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 connections, got %d", count)
	}
}
