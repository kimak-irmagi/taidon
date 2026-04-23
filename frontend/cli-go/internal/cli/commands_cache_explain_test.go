package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintCacheUsage(t *testing.T) {
	var buf bytes.Buffer
	PrintCacheUsage(&buf)
	out := buf.String()
	if !strings.Contains(out, "sqlrs cache explain prepare") {
		t.Fatalf("unexpected cache usage: %q", out)
	}
	if !strings.Contains(out, "--watch and --no-watch are not accepted") {
		t.Fatalf("expected watch rejection note, got %q", out)
	}
}

func TestPrintCacheExplainHumanAndJSON(t *testing.T) {
	result := CacheExplainResult{
		Decision:   "hit",
		ReasonCode: "exact_state_match",
		Prepare: CacheExplainPrepareSpec{
			Class: "alias",
			Kind:  "psql",
			Image: "postgres:17",
		},
		RefContext: &CacheExplainRefContext{
			Requested:      "origin/main",
			ResolvedCommit: "abcdef123456",
			Mode:           "worktree",
		},
		Cache: CacheExplainDecision{
			Signature:      "sig-1",
			MatchedStateID: "state-1",
		},
		Inputs: []CacheExplainInput{
			{Path: "examples/chinook.prep.s9s.yaml", Hash: "sha256:aaa"},
			{Path: "examples/chinook/prepare.sql", Hash: "sha256:bbb"},
		},
	}

	var human bytes.Buffer
	if err := PrintCacheExplain(&human, result, "human"); err != nil {
		t.Fatalf("PrintCacheExplain(human): %v", err)
	}
	out := human.String()
	for _, want := range []string{
		"decision: hit",
		"reasonCode: exact_state_match",
		"prepare.kind: psql",
		"cache.signature: sig-1",
		"cache.stateId: state-1",
		"ref.requested: origin/main",
		"ref.resolvedCommit: abcdef123456",
		"input.count: 2",
		"input[0]: examples/chinook.prep.s9s.yaml sha256:aaa",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in human output, got %q", want, out)
		}
	}

	var jsonOut bytes.Buffer
	if err := PrintCacheExplain(&jsonOut, result, "json"); err != nil {
		t.Fatalf("PrintCacheExplain(json): %v", err)
	}
	if !strings.Contains(jsonOut.String(), `"decision":"hit"`) || !strings.Contains(jsonOut.String(), `"reasonCode":"exact_state_match"`) {
		t.Fatalf("unexpected json output: %q", jsonOut.String())
	}
}

func TestIsCommandTokenIncludesCache(t *testing.T) {
	if !isCommandToken("cache") {
		t.Fatalf("expected cache to be a command token")
	}
}
