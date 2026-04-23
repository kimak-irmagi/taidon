package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
	"github.com/sqlrs/cli/internal/refctx"
)

func TestRunPlanWritesProvenanceArtifactWithoutChangingOutput(t *testing.T) {
	root := t.TempDir()
	scriptPath := filepath.Join(root, "prepare.sql")
	if err := os.WriteFile(scriptPath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}
	provenancePath := filepath.Join(root, "artifacts", "plan.json")

	prevBind := bindPreparePsqlInputsFn
	bindPreparePsqlInputsFn = func(cli.PrepareOptions, string, string, prepareArgs, *refctx.Context, io.Reader) (prepareStageBinding, error) {
		return prepareStageBinding{PsqlArgs: []string{"-f", scriptPath}}, nil
	}
	t.Cleanup(func() { bindPreparePsqlInputsFn = prevBind })

	prevExplain := explainPrepareCacheFn
	explainPrepareCacheFn = func(context.Context, cli.PrepareOptions) (client.CacheExplainPrepareResponse, error) {
		return client.CacheExplainPrepareResponse{
			Decision:       "hit",
			ReasonCode:     "exact_state_match",
			Signature:      "sig-1",
			MatchedStateID: "state-1",
		}, nil
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

	var stdout bytes.Buffer
	err := runPlanKindParsedWithPathMode(&stdout, io.Discard, cli.PrepareOptions{Mode: "remote", Endpoint: "http://example"}, config.LoadedConfig{}, root, root, prepareArgs{
		Image:          "img",
		PsqlArgs:       []string{"-f", "prepare.sql"},
		ProvenancePath: provenancePath,
	}, nil, "json", "psql", true)
	if err != nil {
		t.Fatalf("runPlanKindParsedWithPathMode: %v", err)
	}

	var plan cli.PlanResult
	if err := json.Unmarshal(stdout.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan output: %v", err)
	}
	if plan.PrepareKind != "psql" || plan.ImageID != "img" {
		t.Fatalf("unexpected plan output: %+v", plan)
	}

	data, err := os.ReadFile(provenancePath)
	if err != nil {
		t.Fatalf("read provenance: %v", err)
	}
	var artifact map[string]any
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("decode provenance: %v", err)
	}
	command, ok := artifact["command"].(map[string]any)
	if !ok || command["family"] != "plan" {
		t.Fatalf("unexpected command payload: %+v", artifact["command"])
	}
	cache, ok := artifact["cache"].(map[string]any)
	if !ok || cache["decision"] != "hit" || cache["signature"] != "sig-1" {
		t.Fatalf("unexpected cache payload: %+v", artifact["cache"])
	}
	outcome, ok := artifact["outcome"].(map[string]any)
	if !ok || outcome["status"] != "succeeded" {
		t.Fatalf("unexpected outcome payload: %+v", artifact["outcome"])
	}
}
