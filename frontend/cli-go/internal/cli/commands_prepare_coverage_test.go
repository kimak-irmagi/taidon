package cli

import (
	"testing"

	"sqlrs/cli/internal/client"
)

func TestFormatPrepareEventVariants(t *testing.T) {
	cases := []struct {
		name  string
		event client.PrepareJobEvent
		want  string
	}{
		{
			name:  "status with message",
			event: client.PrepareJobEvent{Type: "status", Status: "running", Message: "ok"},
			want:  "prepare status: running - ok",
		},
		{
			name:  "status without status",
			event: client.PrepareJobEvent{Type: "status", Message: "waiting"},
			want:  "prepare status - waiting",
		},
		{
			name:  "task with id and status",
			event: client.PrepareJobEvent{Type: "task", TaskID: "task-1", Status: "running"},
			want:  "prepare task task-1: running",
		},
		{
			name:  "task with id only",
			event: client.PrepareJobEvent{Type: "task", TaskID: "task-1", Message: "init"},
			want:  "prepare task task-1 - init",
		},
		{
			name:  "task with status only",
			event: client.PrepareJobEvent{Type: "task", Status: "queued"},
			want:  "prepare task: queued",
		},
		{
			name:  "task with embedded error details",
			event: client.PrepareJobEvent{Type: "task", TaskID: "execute-0", Status: "failed", Error: &client.ErrorResponse{Message: "psql execution failed", Details: "exit status 3"}},
			want:  "prepare task execute-0: failed - psql execution failed: exit status 3",
		},
		{
			name:  "result",
			event: client.PrepareJobEvent{Type: "result"},
			want:  "prepare result: ready",
		},
		{
			name:  "error with details",
			event: client.PrepareJobEvent{Type: "error", Error: &client.ErrorResponse{Message: "bad", Details: "input"}},
			want:  "prepare error: bad: input",
		},
		{
			name:  "error with message",
			event: client.PrepareJobEvent{Type: "error", Error: &client.ErrorResponse{Message: "bad"}},
			want:  "prepare error: bad",
		},
		{
			name:  "error without details",
			event: client.PrepareJobEvent{Type: "error"},
			want:  "prepare error",
		},
		{
			name:  "unknown with message",
			event: client.PrepareJobEvent{Type: "note", Message: "hello"},
			want:  "prepare note: hello",
		},
		{
			name:  "unknown without message",
			event: client.PrepareJobEvent{Type: "note"},
			want:  "prepare note",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatPrepareEvent(tc.event); got != tc.want {
				t.Fatalf("formatPrepareEvent() = %q, want %q", got, tc.want)
			}
		})
	}
}
