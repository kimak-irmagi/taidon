package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/config"
)

func TestCacheExplainMatchesPrepareBindingForRawStage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "prepare.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write prepare.sql: %v", err)
	}

	prepareOpts, explainCalls, artifact, cacheResult := captureRawPrepareAndCacheParity(
		t,
		root,
		root,
		[]string{"--provenance-path", filepath.Join("artifacts", "prepare.json"), "--image", "img", "--", "-f", "prepare.sql"},
		[]string{"explain", "prepare:psql", "--image", "img", "--", "-f", "prepare.sql"},
	)

	if len(explainCalls) != 2 {
		t.Fatalf("explain calls = %d, want 2", len(explainCalls))
	}
	assertPrepareOptionsParity(t, prepareOpts, explainCalls[0])
	assertPrepareOptionsParity(t, prepareOpts, explainCalls[1])
	if !reflect.DeepEqual(cacheResult.Inputs, artifact.Inputs) {
		t.Fatalf("cache inputs = %+v, want %+v", cacheResult.Inputs, artifact.Inputs)
	}
	if cacheResult.Prepare != artifact.Prepare {
		t.Fatalf("cache prepare = %+v, want %+v", cacheResult.Prepare, artifact.Prepare)
	}
	if cacheResult.Cache.ResolvedImageID != artifact.Cache.ResolvedImageID {
		t.Fatalf("cache resolved image = %q, want %q", cacheResult.Cache.ResolvedImageID, artifact.Cache.ResolvedImageID)
	}
}

func TestCacheExplainMatchesPrepareBindingForAliasStage(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "prepare.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write prepare.sql: %v", err)
	}
	writePrepareAliasFile(t, root, "demo.prep.s9s.yaml", "kind: psql\nimage: img\nargs:\n  - -f\n  - prepare.sql\n")

	var prepareOpts cli.PrepareOptions
	var explainCalls []cli.PrepareOptions

	prevRunPrepare := runPrepareFn
	runPrepareFn = func(_ context.Context, opts cli.PrepareOptions) (client.PrepareJobResult, error) {
		prepareOpts = opts
		return client.PrepareJobResult{DSN: "dsn"}, nil
	}
	t.Cleanup(func() { runPrepareFn = prevRunPrepare })

	prevExplain := explainPrepareCacheFn
	explainPrepareCacheFn = func(_ context.Context, opts cli.PrepareOptions) (client.CacheExplainPrepareResponse, error) {
		explainCalls = append(explainCalls, opts)
		return client.CacheExplainPrepareResponse{
			Decision:   "miss",
			ReasonCode: "no_matching_state",
			Signature:  "sig-alias",
		}, nil
	}
	t.Cleanup(func() { explainPrepareCacheFn = prevExplain })

	alias, aliasPath, ref, err := resolvePrepareAliasWithOptionalRef(root, root, "demo", "", "", false)
	if err != nil {
		t.Fatalf("resolvePrepareAliasWithOptionalRef: %v", err)
	}
	alias.Args = rebasePrepareAliasArgs(alias.Kind, alias.Args, aliasPath)

	provenancePath := filepath.Join(root, "artifacts", "prepare.json")
	_, handled, err := prepareResultStageRequest(
		stdoutAndErr{stdout: &bytes.Buffer{}, stderr: io.Discard},
		cli.PrepareOptions{},
		config.LoadedConfig{},
		stageRunRequest{
			mode:          stageModePrepare,
			class:         "alias",
			kind:          alias.Kind,
			parsed:        prepareArgs{Image: alias.Image, PsqlArgs: alias.Args, Watch: true, ProvenancePath: provenancePath},
			workspaceRoot: root,
			cwd:           root,
			invocationCwd: root,
			aliasPath:     aliasPath,
			ref:           ref,
		},
	)
	if err != nil {
		t.Fatalf("prepareResultStageRequest: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false")
	}

	var cacheOut bytes.Buffer
	if err := runCache(&cacheOut, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, root, root, []string{"explain", "prepare", "demo"}, "json"); err != nil {
		t.Fatalf("runCache: %v", err)
	}

	if len(explainCalls) != 2 {
		t.Fatalf("explain calls = %d, want 2", len(explainCalls))
	}
	assertPrepareOptionsParity(t, prepareOpts, explainCalls[0])
	assertPrepareOptionsParity(t, prepareOpts, explainCalls[1])

	artifact := decodeProvenanceArtifact(t, provenancePath)
	cacheResult := decodeCacheExplainResult(t, cacheOut.Bytes())
	if !reflect.DeepEqual(cacheResult.Inputs, artifact.Inputs) {
		t.Fatalf("cache inputs = %+v, want %+v", cacheResult.Inputs, artifact.Inputs)
	}
	if cacheResult.Prepare != artifact.Prepare {
		t.Fatalf("cache prepare = %+v, want %+v", cacheResult.Prepare, artifact.Prepare)
	}
}

func TestCacheExplainMatchesPrepareBindingForRefStage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, head := initCacheExplainBindingRepo(t)
	workspaceRoot := filepath.Join(repo, "workspace")
	cwd := filepath.Join(workspaceRoot, "app")

	prepareOpts, explainCalls, artifact, cacheResult := captureRawPrepareAndCacheParity(
		t,
		workspaceRoot,
		cwd,
		[]string{"--provenance-path", filepath.Join("artifacts", "prepare.json"), "--ref", head, "--ref-mode", "blob", "--image", "img", "--", "-c", "select 1"},
		[]string{"explain", "prepare:psql", "--ref", head, "--ref-mode", "blob", "--image", "img", "--", "-c", "select 1"},
	)

	if len(explainCalls) != 2 {
		t.Fatalf("explain calls = %d, want 2", len(explainCalls))
	}
	assertPrepareOptionsParity(t, prepareOpts, explainCalls[0])
	assertPrepareOptionsParity(t, prepareOpts, explainCalls[1])
	if cacheResult.RefContext == nil || artifact.RefContext == nil {
		t.Fatalf("expected ref context in both outputs, got cache=%+v artifact=%+v", cacheResult.RefContext, artifact.RefContext)
	}
	if *cacheResult.RefContext != *artifact.RefContext {
		t.Fatalf("cache ref = %+v, want %+v", *cacheResult.RefContext, *artifact.RefContext)
	}
	if !reflect.DeepEqual(cacheResult.Inputs, artifact.Inputs) {
		t.Fatalf("cache inputs = %+v, want %+v", cacheResult.Inputs, artifact.Inputs)
	}
}

func captureRawPrepareAndCacheParity(t *testing.T, workspaceRoot string, cwd string, prepareArgs []string, cacheArgs []string) (cli.PrepareOptions, []cli.PrepareOptions, provenanceArtifact, cli.CacheExplainResult) {
	t.Helper()

	var prepareOpts cli.PrepareOptions
	var explainCalls []cli.PrepareOptions

	prevRunPrepare := runPrepareFn
	runPrepareFn = func(_ context.Context, opts cli.PrepareOptions) (client.PrepareJobResult, error) {
		prepareOpts = opts
		return client.PrepareJobResult{DSN: "dsn"}, nil
	}
	t.Cleanup(func() { runPrepareFn = prevRunPrepare })

	prevExplain := explainPrepareCacheFn
	explainPrepareCacheFn = func(_ context.Context, opts cli.PrepareOptions) (client.CacheExplainPrepareResponse, error) {
		explainCalls = append(explainCalls, opts)
		return client.CacheExplainPrepareResponse{
			Decision:        "miss",
			ReasonCode:      "no_matching_state",
			Signature:       "sig-raw",
			ResolvedImageID: "img@sha256:resolved",
		}, nil
	}
	t.Cleanup(func() { explainPrepareCacheFn = prevExplain })

	provenancePath := filepath.Join(cwd, "artifacts", "prepare.json")
	_, handled, err := prepareResult(
		stdoutAndErr{stdout: &bytes.Buffer{}, stderr: io.Discard},
		cli.PrepareOptions{},
		config.LoadedConfig{},
		workspaceRoot,
		cwd,
		prepareArgs,
	)
	if err != nil {
		t.Fatalf("prepareResult: %v", err)
	}
	if handled {
		t.Fatal("expected handled=false")
	}

	var cacheOut bytes.Buffer
	if err := runCache(&cacheOut, io.Discard, cli.PrepareOptions{}, config.LoadedConfig{}, workspaceRoot, cwd, cacheArgs, "json"); err != nil {
		t.Fatalf("runCache: %v", err)
	}

	return prepareOpts, explainCalls, decodeProvenanceArtifact(t, provenancePath), decodeCacheExplainResult(t, cacheOut.Bytes())
}

func decodeProvenanceArtifact(t *testing.T, path string) provenanceArtifact {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read provenance artifact: %v", err)
	}
	var artifact provenanceArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatalf("decode provenance artifact: %v", err)
	}
	return artifact
}

func decodeCacheExplainResult(t *testing.T, data []byte) cli.CacheExplainResult {
	t.Helper()

	var result cli.CacheExplainResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("decode cache explain result: %v", err)
	}
	return result
}

func assertPrepareOptionsParity(t *testing.T, prepareOpts cli.PrepareOptions, explainOpts cli.PrepareOptions) {
	t.Helper()

	if prepareOpts.PrepareKind != explainOpts.PrepareKind {
		t.Fatalf("PrepareKind = %q, want %q", explainOpts.PrepareKind, prepareOpts.PrepareKind)
	}
	if prepareOpts.ImageID != explainOpts.ImageID {
		t.Fatalf("ImageID = %q, want %q", explainOpts.ImageID, prepareOpts.ImageID)
	}
	if !reflect.DeepEqual(prepareOpts.PsqlArgs, explainOpts.PsqlArgs) {
		t.Fatalf("PsqlArgs = %+v, want %+v", explainOpts.PsqlArgs, prepareOpts.PsqlArgs)
	}
	if !reflect.DeepEqual(prepareOpts.LiquibaseArgs, explainOpts.LiquibaseArgs) {
		t.Fatalf("LiquibaseArgs = %+v, want %+v", explainOpts.LiquibaseArgs, prepareOpts.LiquibaseArgs)
	}
	if prepareOpts.WorkDir != explainOpts.WorkDir {
		t.Fatalf("WorkDir = %q, want %q", explainOpts.WorkDir, prepareOpts.WorkDir)
	}
	if prepareOpts.PlanOnly != explainOpts.PlanOnly {
		t.Fatalf("PlanOnly = %v, want %v", explainOpts.PlanOnly, prepareOpts.PlanOnly)
	}
	if valueOrEmptyPtr(prepareOpts.Stdin) != valueOrEmptyPtr(explainOpts.Stdin) {
		t.Fatalf("Stdin = %q, want %q", valueOrEmptyPtr(explainOpts.Stdin), valueOrEmptyPtr(prepareOpts.Stdin))
	}
}

func valueOrEmptyPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func initCacheExplainBindingRepo(t *testing.T) (string, string) {
	t.Helper()

	emptyTemplate := t.TempDir()
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	initCmd := exec.Command("git", "-C", repo, "init", "--template", emptyTemplate)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Skipf("git init skipped: %v\n%s", err, out)
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")

	appDir := filepath.Join(repo, "workspace", "app")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		t.Fatalf("mkdir app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "query.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write query.sql: %v", err)
	}
	runGit("add", "workspace")
	runGit("commit", "-m", "initial")

	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return repo, strings.TrimSpace(string(out))
}
