package cli

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseArgs(t *testing.T) {
	opts, cmds, err := ParseArgs([]string{
		"--profile", "p",
		"--endpoint", "http://example",
		"--mode", "remote",
		"--output", "json",
		"--timeout", "5s",
		"-v",
		"status",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}

	if len(cmds) != 1 || cmds[0].Name != "status" {
		t.Fatalf("expected command status, got %+v", cmds)
	}
	if opts.Profile != "p" {
		t.Fatalf("expected profile p, got %q", opts.Profile)
	}
	if opts.Endpoint != "http://example" {
		t.Fatalf("expected endpoint override, got %q", opts.Endpoint)
	}
	if opts.Mode != "remote" {
		t.Fatalf("expected mode remote, got %q", opts.Mode)
	}
	if opts.Output != "json" {
		t.Fatalf("expected output json, got %q", opts.Output)
	}
	if opts.Timeout != 5*time.Second {
		t.Fatalf("expected timeout 5s, got %v", opts.Timeout)
	}
	if !opts.Verbose {
		t.Fatalf("expected verbose true")
	}
}

func TestParseArgsHelp(t *testing.T) {
	_, _, err := ParseArgs([]string{"--help"})
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestParseArgsMissingCommand(t *testing.T) {
	_, _, err := ParseArgs([]string{"--profile", "p"})
	if err == nil {
		t.Fatalf("expected missing command error")
	}
}

func TestParseArgsInvalidTimeout(t *testing.T) {
	_, _, err := ParseArgs([]string{"--timeout", "bad", "status"})
	if err == nil || !strings.Contains(err.Error(), "invalid timeout") {
		t.Fatalf("expected invalid timeout error, got %v", err)
	}
}

func TestParseArgsUnknownFlag(t *testing.T) {
	_, _, err := ParseArgs([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseArgsUnicodeDashHint(t *testing.T) {
	_, _, err := ParseArgs([]string{"—workspace", "/tmp", "status"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "Unicode dash") {
		t.Fatalf("expected unicode dash hint, got %v", err)
	}
	if !strings.Contains(err.Error(), "--workspace") {
		t.Fatalf("expected suggested normalized flag, got %v", err)
	}
}

func TestParseArgsMultipleCommands(t *testing.T) {
	_, cmds, err := ParseArgs([]string{
		"prepare:psql", "--", "-c", "select 1",
		"run:psql", "--", "-c", "select 1",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 2 || cmds[0].Name != "prepare:psql" || cmds[1].Name != "run:psql" {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
}

func TestParseArgsPrepareAliasMode(t *testing.T) {
	_, cmds, err := ParseArgs([]string{"prepare", "chinook"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "prepare" {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	if len(cmds[0].Args) != 1 || cmds[0].Args[0] != "chinook" {
		t.Fatalf("unexpected prepare args: %+v", cmds[0].Args)
	}
}

func TestParseArgsPlanAliasMode(t *testing.T) {
	_, cmds, err := ParseArgs([]string{"plan", "path/chinook"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "plan" {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	if len(cmds[0].Args) != 1 || cmds[0].Args[0] != "path/chinook" {
		t.Fatalf("unexpected plan args: %+v", cmds[0].Args)
	}
}

func TestParseArgsPrepareAliasExactFileMode(t *testing.T) {
	_, cmds, err := ParseArgs([]string{"prepare", "chinook.txt."})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "prepare" {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	if len(cmds[0].Args) != 1 || cmds[0].Args[0] != "chinook.txt." {
		t.Fatalf("unexpected prepare args: %+v", cmds[0].Args)
	}
}

func TestParseArgsCompositePrepareAliasRunRaw(t *testing.T) {
	_, cmds, err := ParseArgs([]string{
		"prepare", "chinook",
		"run:psql", "--", "-f", "queries.sql",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %+v", cmds)
	}
	if cmds[0].Name != "prepare" || len(cmds[0].Args) != 1 || cmds[0].Args[0] != "chinook" {
		t.Fatalf("unexpected first command: %+v", cmds[0])
	}
	if cmds[1].Name != "run:psql" || len(cmds[1].Args) != 3 || cmds[1].Args[0] != "--" || cmds[1].Args[1] != "-f" || cmds[1].Args[2] != "queries.sql" {
		t.Fatalf("unexpected second command: %+v", cmds[1])
	}
}

func TestParseArgsCompositePrepareAliasParentRefRunRaw(t *testing.T) {
	_, cmds, err := ParseArgs([]string{
		"prepare", "../chinook",
		"run:psql", "--", "-f", "queries.sql",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %+v", cmds)
	}
	if cmds[0].Name != "prepare" || len(cmds[0].Args) != 1 || cmds[0].Args[0] != "../chinook" {
		t.Fatalf("unexpected first command: %+v", cmds[0])
	}
	if cmds[1].Name != "run:psql" || len(cmds[1].Args) != 3 || cmds[1].Args[0] != "--" || cmds[1].Args[1] != "-f" || cmds[1].Args[2] != "queries.sql" {
		t.Fatalf("unexpected second command: %+v", cmds[1])
	}
}

func TestParseArgsCompositePrepareRawRunAlias(t *testing.T) {
	_, cmds, err := ParseArgs([]string{
		"prepare:psql", "--", "-f", "prepare.sql",
		"run", "smoke",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %+v", cmds)
	}
	if cmds[0].Name != "prepare:psql" || len(cmds[0].Args) != 3 || cmds[0].Args[0] != "--" || cmds[0].Args[1] != "-f" || cmds[0].Args[2] != "prepare.sql" {
		t.Fatalf("unexpected first command: %+v", cmds[0])
	}
	if cmds[1].Name != "run" || len(cmds[1].Args) != 1 || cmds[1].Args[0] != "smoke" {
		t.Fatalf("unexpected second command: %+v", cmds[1])
	}
}

func TestParseArgsCompositePrepareAliasRunAlias(t *testing.T) {
	_, cmds, err := ParseArgs([]string{
		"prepare", "chinook",
		"run", "smoke",
	})
	if err != nil {
		t.Fatalf("parse args: %v", err)
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

func TestParseArgsRunAliasStandalone(t *testing.T) {
	_, cmds, err := ParseArgs([]string{"run", "smoke", "--instance", "dev"})
	if err != nil {
		t.Fatalf("parse args: %v", err)
	}
	if len(cmds) != 1 || cmds[0].Name != "run" {
		t.Fatalf("unexpected commands: %+v", cmds)
	}
	if got := strings.Join(cmds[0].Args, "|"); got != "smoke|--instance|dev" {
		t.Fatalf("unexpected run args: %q", got)
	}
}
