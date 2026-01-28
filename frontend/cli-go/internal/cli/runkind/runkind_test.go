package runkind

import "testing"

func TestDefaultCommand(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{kind: "psql", want: "psql"},
		{kind: " PSQL ", want: "psql"},
		{kind: "pgbench", want: "pgbench"},
		{kind: " PgBench ", want: "pgbench"},
		{kind: "", want: ""},
		{kind: "unknown", want: ""},
	}
	for _, tc := range cases {
		if got := DefaultCommand(tc.kind); got != tc.want {
			t.Fatalf("DefaultCommand(%q) = %q, want %q", tc.kind, got, tc.want)
		}
	}
}
