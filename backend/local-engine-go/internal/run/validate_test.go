package run

import "testing"

func TestHasPsqlConnectionArgs(t *testing.T) {
	if !hasPsqlConnectionArgs([]string{"-h", "localhost"}) {
		t.Fatalf("expected connection args")
	}
	if !hasPsqlConnectionArgs([]string{"--host=localhost"}) {
		t.Fatalf("expected --host")
	}
	if !hasPsqlConnectionArgs([]string{"-hlocalhost"}) {
		t.Fatalf("expected -hvalue")
	}
	if !hasPsqlConnectionArgs([]string{"postgres://user@host/db"}) {
		t.Fatalf("expected dsn arg")
	}
	if hasPsqlConnectionArgs([]string{"-c", "select 1"}) {
		t.Fatalf("unexpected connection args")
	}
}

func TestHasPgbenchConnectionArgs(t *testing.T) {
	if !hasPgbenchConnectionArgs([]string{"-h", "localhost"}) {
		t.Fatalf("expected connection args")
	}
	if !hasPgbenchConnectionArgs([]string{"-p5432"}) {
		t.Fatalf("expected -pvalue")
	}
	if hasPgbenchConnectionArgs([]string{"-c", "10"}) {
		t.Fatalf("unexpected connection args")
	}
}

func TestIsPsqlConnFlag(t *testing.T) {
	cases := []string{"-h", "-p", "-U", "-d", "--host", "--port", "--username", "--dbname", "--database"}
	for _, value := range cases {
		if !isPsqlConnFlag(value) {
			t.Fatalf("expected conn flag for %q", value)
		}
	}
	if !isPsqlConnFlag("--host=127.0.0.1") {
		t.Fatalf("expected host= flag")
	}
	if !isPsqlConnFlag("--port=5432") {
		t.Fatalf("expected port= flag")
	}
	if !isPsqlConnFlag("--dbname=test") {
		t.Fatalf("expected dbname= flag")
	}
	if !isPsqlConnFlag("--database=test") {
		t.Fatalf("expected database= flag")
	}
	if !isPsqlConnFlag("-Uuser") {
		t.Fatalf("expected -Uvalue")
	}
	if !isPsqlConnFlag("-p5432") {
		t.Fatalf("expected -pvalue")
	}
	if !isPsqlConnFlag("-dtest") {
		t.Fatalf("expected -dvalue")
	}
	if isPsqlConnFlag("-c") {
		t.Fatalf("unexpected -c")
	}
}
