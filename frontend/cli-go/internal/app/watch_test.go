package app

import "testing"

func TestParseWatchArgs(t *testing.T) {
	jobID, showHelp, err := parseWatchArgs([]string{"job-1"})
	if err != nil {
		t.Fatalf("parseWatchArgs: %v", err)
	}
	if showHelp {
		t.Fatalf("expected showHelp=false")
	}
	if jobID != "job-1" {
		t.Fatalf("expected job-1, got %q", jobID)
	}
}

func TestParseWatchArgsHelp(t *testing.T) {
	_, showHelp, err := parseWatchArgs([]string{"--help"})
	if err != nil {
		t.Fatalf("parseWatchArgs: %v", err)
	}
	if !showHelp {
		t.Fatalf("expected showHelp=true")
	}
}

func TestParseWatchArgsRejectsExtraArgs(t *testing.T) {
	_, _, err := parseWatchArgs([]string{"job-1", "job-2"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
