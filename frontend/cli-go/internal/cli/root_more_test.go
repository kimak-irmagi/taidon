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
}

func TestIsCommandToken(t *testing.T) {
	cases := []string{"init", "ls", "rm", "plan", "prepare", "run", "watch", "status", "config", "prepare:psql", "prepare:lb", "plan:psql", "plan:lb", "run:psql"}
	for _, value := range cases {
		if !isCommandToken(value) {
			t.Fatalf("expected command token for %q", value)
		}
	}
	if isCommandToken("unknown") {
		t.Fatalf("unexpected command token")
	}
}
