package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/refctx"
)

func TestRunPlanParseErrorDoesNotWriteProvenance(t *testing.T) {
	root := t.TempDir()
	provenancePath := filepath.Join(root, "artifacts", "plan.json")

	err := runPlanKindWithPathMode(io.Discard, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, root, root, []string{"--provenance-path", provenancePath}, "human", "psql", true)
	if err == nil {
		t.Fatalf("expected parse/bind error")
	}
	if _, statErr := os.Stat(provenancePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no provenance artifact, stat err=%v", statErr)
	}
}

func TestPrepareFailureWritesFailedProvenance(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "prepare.sql")
	if err := os.WriteFile(scriptPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}
	provenancePath := filepath.Join(root, "artifacts", "prepare.json")

	prevBind := bindPreparePsqlInputsFn
	bindPreparePsqlInputsFn = func(cli.PrepareOptions, string, string, prepareArgs, *refctx.Context, io.Reader) (prepareStageBinding, error) {
		return prepareStageBinding{PsqlArgs: []string{"-f", scriptPath}}, nil
	}
	t.Cleanup(func() { bindPreparePsqlInputsFn = prevBind })

	prevExplain := explainPrepareCacheFn
	explainPrepareCacheFn = func(context.Context, cli.PrepareOptions) (client.CacheExplainPrepareResponse, error) {
		return client.CacheExplainPrepareResponse{
			Decision:   "miss",
			ReasonCode: "no_matching_state",
			Signature:  "sig-prepare",
		}, nil
	}
	t.Cleanup(func() { explainPrepareCacheFn = prevExplain })

	prevRunPrepare := runPrepareFn
	runPrepareFn = func(context.Context, cli.PrepareOptions) (client.PrepareJobResult, error) {
		return client.PrepareJobResult{}, errors.New("prepare boom")
	}
	t.Cleanup(func() { runPrepareFn = prevRunPrepare })

	_, _, err := prepareResultParsed(stdoutAndErr{stdout: &bytes.Buffer{}, stderr: io.Discard}, cli.PrepareOptions{Mode: "remote", Endpoint: "http://example"}, config.LoadedConfig{}, root, root, prepareArgs{
		Image:          "img",
		PsqlArgs:       []string{"-f", "prepare.sql"},
		Watch:          true,
		ProvenancePath: provenancePath,
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "prepare boom") {
		t.Fatalf("expected prepare error, got %v", err)
	}

	data, readErr := os.ReadFile(provenancePath)
	if readErr != nil {
		t.Fatalf("read provenance: %v", readErr)
	}
	if !strings.Contains(string(data), `"status":"failed"`) {
		t.Fatalf("expected failed provenance artifact, got %q", string(data))
	}
}

func TestProvenanceWriteErrorIsReturned(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "prepare.sql")
	if err := os.WriteFile(scriptPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	prevBind := bindPreparePsqlInputsFn
	bindPreparePsqlInputsFn = func(cli.PrepareOptions, string, string, prepareArgs, *refctx.Context, io.Reader) (prepareStageBinding, error) {
		return prepareStageBinding{PsqlArgs: []string{"-f", scriptPath}}, nil
	}
	t.Cleanup(func() { bindPreparePsqlInputsFn = prevBind })

	prevExplain := explainPrepareCacheFn
	explainPrepareCacheFn = func(context.Context, cli.PrepareOptions) (client.CacheExplainPrepareResponse, error) {
		return client.CacheExplainPrepareResponse{Decision: "hit", ReasonCode: "exact_state_match", Signature: "sig-1"}, nil
	}
	t.Cleanup(func() { explainPrepareCacheFn = prevExplain })

	prevRunPlan := runPlanFn
	runPlanFn = func(context.Context, cli.PrepareOptions) (cli.PlanResult, error) {
		return cli.PlanResult{
			PrepareKind:           "psql",
			ImageID:               "img",
			PrepareArgsNormalized: "-f prepare.sql",
			Tasks: []client.PlanTask{
				{TaskID: "plan", Type: "plan", PlannerKind: "psql"},
				{TaskID: "execute-0", Type: "state_execute", OutputStateID: "state-1"},
			},
		}, nil
	}
	t.Cleanup(func() { runPlanFn = prevRunPlan })

	prevWrite := writeProvenanceArtifactFn
	writeProvenanceArtifactFn = func(string, provenanceArtifact) error {
		return errors.New("write provenance boom")
	}
	t.Cleanup(func() { writeProvenanceArtifactFn = prevWrite })

	err := runPlanKindParsedWithPathMode(io.Discard, io.Discard, cli.PrepareOptions{Mode: "remote", Endpoint: "http://example"}, config.LoadedConfig{}, root, root, prepareArgs{
		Image:          "img",
		PsqlArgs:       []string{"-f", "prepare.sql"},
		ProvenancePath: filepath.Join(root, "artifacts", "plan.json"),
	}, nil, "human", "psql", true)
	if err == nil || !strings.Contains(err.Error(), "write provenance boom") {
		t.Fatalf("expected provenance write error, got %v", err)
	}
}
