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

func TestIsKnown(t *testing.T) {
	cases := []struct {
		kind string
		want bool
	}{
		{kind: "psql", want: true},
		{kind: " PSQL ", want: true},
		{kind: "pgbench", want: true},
		{kind: " PgBench ", want: true},
		{kind: "", want: false},
		{kind: "unknown", want: false},
	}
	for _, tc := range cases {
		if got := IsKnown(tc.kind); got != tc.want {
			t.Fatalf("IsKnown(%q) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

func TestHasConnectionArgsPsql(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{args: []string{"-h", "localhost"}, want: true},
		{args: []string{"--host", "localhost"}, want: true},
		{args: []string{"--port", "5432"}, want: true},
		{args: []string{"--username", "user"}, want: true},
		{args: []string{"--dbname", "db"}, want: true},
		{args: []string{"--database", "db"}, want: true},
		{args: []string{"--host=localhost"}, want: true},
		{args: []string{"-p5432"}, want: true},
		{args: []string{"-hlocalhost"}, want: true},
		{args: []string{"-Uuser"}, want: true},
		{args: []string{"-ddb"}, want: true},
		{args: []string{"-d", "db"}, want: true},
		{args: []string{"postgres://user@localhost/db"}, want: true},
		{args: []string{"", " "}, want: false},
		{args: []string{"-f", "init.sql"}, want: false},
		{args: []string{"-x"}, want: false},
		{args: []string{"--echo-all"}, want: false},
	}
	for _, tc := range cases {
		if got := HasConnectionArgs("psql", tc.args); got != tc.want {
			t.Fatalf("HasConnectionArgs(psql,%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestHasConnectionArgsPgbench(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{args: []string{"", " "}, want: false},
		{args: []string{"-h", "localhost"}, want: true},
		{args: []string{"-hlocalhost"}, want: true},
		{args: []string{"-p"}, want: true},
		{args: []string{"-p5432"}, want: true},
		{args: []string{"-p9999"}, want: true},
		{args: []string{"-Uuser"}, want: true},
		{args: []string{"-ddb"}, want: true},
		{args: []string{"-d", "db"}, want: true},
		{args: []string{"-M", "prepared"}, want: false},
	}
	for _, tc := range cases {
		if got := HasConnectionArgs("pgbench", tc.args); got != tc.want {
			t.Fatalf("HasConnectionArgs(pgbench,%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestHasConnectionArgsUnknown(t *testing.T) {
	if HasConnectionArgs("unknown", []string{"-h", "localhost"}) {
		t.Fatalf("expected false for unknown kind")
	}
}
