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
	cases := []string{"init", "ls", "rm", "plan", "prepare", "run", "watch", "status", "config", "alias", "prepare:psql", "prepare:lb", "plan:psql", "plan:lb", "run:psql"}
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
