package cli

import "testing"

func TestSplitCommandsNonComposite(t *testing.T) {
	cmds, err := splitCommands([]string{"run:psql", "--", "-c", "select 1"})
	if err != nil {
		t.Fatalf("splitCommands: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "run:psql" {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
}

func TestSplitCommandsCompositePrepareRun(t *testing.T) {
	cmds, err := splitCommands([]string{"prepare:psql", "--image", "img", "run:psql", "--", "-c", "select 1"})
	if err != nil {
		t.Fatalf("splitCommands: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %+v", cmds)
	}
	if cmds[0].Name != "prepare:psql" || len(cmds[0].Args) != 2 || cmds[0].Args[0] != "--image" {
		t.Fatalf("unexpected first command: %+v", cmds[0])
	}
	if cmds[1].Name != "run:psql" || len(cmds[1].Args) == 0 || cmds[1].Args[0] != "--" {
		t.Fatalf("unexpected second command: %+v", cmds[1])
	}
}

func TestSplitCommandsCompositePrepareAliasRunAlias(t *testing.T) {
	cmds, err := splitCommands([]string{"prepare", "chinook", "run", "smoke"})
	if err != nil {
		t.Fatalf("splitCommands: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %+v", cmds)
	}
	if cmds[0].Name != "prepare" || len(cmds[0].Args) != 1 || cmds[0].Args[0] != "chinook" {
		t.Fatalf("unexpected first command: %+v", cmds[0])
	}
	if cmds[1].Name != "run" || len(cmds[1].Args) != 1 || cmds[1].Args[0] != "smoke" {
		t.Fatalf("unexpected second command: %+v", cmds[1])
	}
}

func TestSplitCommandsPrepareRawDoesNotSplitToolArgNamedRun(t *testing.T) {
	cases := [][]string{
		{"prepare:psql", "--", "-f", "run", "smoke"},
		{"prepare:lb", "--", "update", "run"},
	}

	for _, args := range cases {
		cmds, err := splitCommands(args)
		if err != nil {
			t.Fatalf("splitCommands(%v): %v", args, err)
		}
		if len(cmds) != 1 {
			t.Fatalf("splitCommands(%v) produced %+v, want single command", args, cmds)
		}
		if cmds[0].Name != args[0] {
			t.Fatalf("command name = %q, want %q", cmds[0].Name, args[0])
		}
		if got := len(cmds[0].Args); got != len(args)-1 {
			t.Fatalf("args len = %d, want %d", got, len(args)-1)
		}
	}
}

func TestIsCompositePrepareRunFalse(t *testing.T) {
	if isCompositePrepareRun([]string{"run:psql", "prepare:psql"}) {
		t.Fatalf("expected false for non-prepare prefix")
	}
	if isCompositePrepareRun([]string{"prepare:psql"}) {
		t.Fatalf("expected false for single command")
	}
	if isCompositePrepareRun([]string{"prepare:psql", "--", "-c", "select 1"}) {
		t.Fatalf("expected false without run")
	}
	if isCompositePrepareRun([]string{"prepare:psql", "--", "-f", "run", "smoke"}) {
		t.Fatalf("expected false when run is a file argument value")
	}
	if isCompositePrepareRun([]string{"prepare:lb", "--", "update", "run"}) {
		t.Fatalf("expected false when run is a trailing liquibase argument")
	}
}

func TestIsCompositePrepareRunTrueForAliasModes(t *testing.T) {
	if !isCompositePrepareRun([]string{"prepare", "chinook", "run", "smoke"}) {
		t.Fatalf("expected true for prepare/run alias composite")
	}
	if !isCompositePrepareRun([]string{"prepare", "chinook", "run:psql", "--", "-c", "select 1"}) {
		t.Fatalf("expected true for prepare alias + raw run composite")
	}
	if !isCompositePrepareRun([]string{"prepare:psql", "--", "-f", "prepare.sql", "run", "smoke"}) {
		t.Fatalf("expected true for raw prepare + run alias composite")
	}
}

func TestIsCommandToken(t *testing.T) {
	cases := []string{"init", "ls", "rm", "diff", "plan", "prepare", "run", "watch", "status", "config", "alias", "prepare:psql", "prepare:lb", "plan:psql", "plan:lb", "run:psql"}
	for _, value := range cases {
		if !isCommandToken(value) {
			t.Fatalf("expected command token for %q", value)
		}
	}
	if isCommandToken("unknown") {
		t.Fatalf("unexpected command token")
	}
}

func TestSplitCommandsMissingCommand(t *testing.T) {
	_, err := splitCommands(nil)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestIsPrepareArgValueBranches(t *testing.T) {
	cases := []struct {
		name string
		args []string
		idx  int
		want bool
	}{
		{name: "idx too small", args: []string{"prepare", "--image"}, idx: 1, want: false},
		{name: "prepare psql provenance path", args: []string{"prepare:psql", "--provenance-path", "run"}, idx: 2, want: true},
		{name: "prepare psql ref", args: []string{"prepare:psql", "--ref", "run"}, idx: 2, want: true},
		{name: "prepare psql ref mode", args: []string{"prepare:psql", "--ref-mode", "run"}, idx: 2, want: true},
		{name: "prepare psql image", args: []string{"prepare:psql", "--image", "img"}, idx: 2, want: true},
		{name: "prepare psql file", args: []string{"prepare:psql", "-f", "file"}, idx: 2, want: true},
		{name: "prepare psql file flag", args: []string{"prepare:psql", "--file", "file"}, idx: 2, want: true},
		{name: "prepare lb provenance path", args: []string{"prepare:lb", "--provenance-path", "run"}, idx: 2, want: true},
		{name: "prepare lb changelog", args: []string{"prepare:lb", "--changelog-file", "file"}, idx: 2, want: true},
		{name: "prepare lb defaults", args: []string{"prepare:lb", "--defaults-file", "file"}, idx: 2, want: true},
		{name: "prepare lb search path camel", args: []string{"prepare:lb", "--searchPath", "dir"}, idx: 2, want: true},
		{name: "prepare lb search path kebab", args: []string{"prepare:lb", "--search-path", "dir"}, idx: 2, want: true},
		{name: "prepare default image", args: []string{"prepare", "--image", "img"}, idx: 2, want: true},
		{name: "prepare default other flag", args: []string{"prepare", "--file", "file"}, idx: 2, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPrepareArgValue(tc.args, tc.idx); got != tc.want {
				t.Fatalf("isPrepareArgValue(%v, %d) = %v, want %v", tc.args, tc.idx, got, tc.want)
			}
		})
	}
}

func TestFindPrepareAliasRunIndexBranches(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{name: "plain composite", args: []string{"prepare", "chinook", "run", "smoke"}, want: 2},
		{name: "skips watch flags", args: []string{"prepare", "chinook", "--watch", "--no-watch", "--help", "-h", "run", "smoke"}, want: 6},
		{name: "skips provenance path", args: []string{"prepare", "--provenance-path", "run", "chinook", "run", "smoke"}, want: 4},
		{name: "skips ref flags", args: []string{"prepare", "--ref", "HEAD", "--ref-mode", "blob", "--ref-keep-worktree", "chinook", "run", "smoke"}, want: 7},
		{name: "missing ref value", args: []string{"prepare", "--ref"}, want: -1},
		{name: "missing ref mode value", args: []string{"prepare", "--ref", "HEAD", "--ref-mode"}, want: -1},
		{name: "stops on separator", args: []string{"prepare", "chinook", "--", "run", "smoke"}, want: -1},
		{name: "stops on flag", args: []string{"prepare", "chinook", "-x", "run", "smoke"}, want: -1},
		{name: "second alias without run boundary", args: []string{"prepare", "chinook", "other"}, want: -1},
		{name: "no run token", args: []string{"prepare", "chinook"}, want: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := findPrepareAliasRunIndex(tc.args); got != tc.want {
				t.Fatalf("findPrepareAliasRunIndex(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

func TestIsCompositeRunBoundaryBranches(t *testing.T) {
	cases := []struct {
		name string
		args []string
		idx  int
		want bool
	}{
		{name: "idx too small", args: []string{"prepare:psql", "run", "smoke"}, idx: 0, want: false},
		{name: "idx too large", args: []string{"prepare:psql", "run", "smoke"}, idx: 3, want: false},
		{name: "raw run token", args: []string{"prepare:psql", "run:psql"}, idx: 1, want: true},
		{name: "empty raw run suffix", args: []string{"prepare:psql", "run:"}, idx: 1, want: false},
		{name: "help after run", args: []string{"prepare:psql", "run", "--help"}, idx: 1, want: true},
		{name: "short help after run", args: []string{"prepare:psql", "run", "-h"}, idx: 1, want: true},
		{name: "separator after run", args: []string{"prepare:psql", "run", "--"}, idx: 1, want: false},
		{name: "flag after run", args: []string{"prepare:psql", "run", "-c"}, idx: 1, want: false},
		{name: "missing next token", args: []string{"prepare:psql", "run"}, idx: 1, want: false},
		{name: "non-run token", args: []string{"prepare:psql", "alias", "smoke"}, idx: 1, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isCompositeRunBoundary(tc.args, tc.idx); got != tc.want {
				t.Fatalf("isCompositeRunBoundary(%v, %d) = %v, want %v", tc.args, tc.idx, got, tc.want)
			}
		})
	}
}
