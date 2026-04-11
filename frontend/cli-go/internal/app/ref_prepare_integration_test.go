package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/config"
)

func TestPrepareResultRefPsqlUsesSelectedRevision(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	for _, mode := range []string{"worktree", "blob"} {
		t.Run(mode, func(t *testing.T) {
			repo, parentRef := initPrepareRefTestRepo(t)
			examplesDir := filepath.Join(repo, "examples")

			var capturedContent string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
					var request map[string]any
					body, _ := io.ReadAll(r.Body)
					_ = json.Unmarshal(body, &request)
					if args, ok := request["psql_args"].([]any); ok && len(args) >= 2 {
						if path, ok := args[1].(string); ok {
							data, err := os.ReadFile(path)
							if err != nil {
								t.Fatalf("read submitted script %q: %v", path, err)
							}
							capturedContent = string(data)
						}
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusAccepted)
					io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
				case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
					w.Header().Set("Content-Type", "application/x-ndjson")
					io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"succeeded"}`+"\n")
				case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"job_id":"job-1","status":"succeeded","prepare_kind":"psql","image_id":"image","result":{"dsn":"postgres://sqlrs@127.0.0.1:5432/postgres","instance_id":"inst","state_id":"state","image_id":"image","prepare_kind":"psql","prepare_args_normalized":"-f prepare.sql"}}`)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			_, handled, err := prepareResult(
				stdoutAndErr{stdout: &bytes.Buffer{}, stderr: io.Discard},
				cli.PrepareOptions{Mode: "remote", Endpoint: server.URL},
				config.LoadedConfig{},
				repo,
				examplesDir,
				[]string{"--ref", parentRef, "--ref-mode", mode, "--image", "image", "--", "-f", filepath.Join("chinook", "prepare.sql")},
			)
			if err != nil {
				t.Fatalf("prepareResult: %v", err)
			}
			if handled {
				t.Fatal("expected watched prepare result")
			}
			if strings.ReplaceAll(capturedContent, "\r\n", "\n") != "select 1;\n" {
				t.Fatalf("submitted script content = %q, want %q", capturedContent, "select 1;\n")
			}
		})
	}
}

func TestRunPlanAliasRefUsesSelectedRevisionAlias(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repo, parentRef := initPrepareRefTestRepo(t)
	temp := t.TempDir()
	setTestDirs(t, temp)

	prevGetwd := getwdFn
	getwdFn = func() (string, error) {
		return filepath.Join(repo, "examples"), nil
	}
	t.Cleanup(func() { getwdFn = prevGetwd })

	var capturedContent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/prepare-jobs":
			var request map[string]any
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &request)
			if args, ok := request["psql_args"].([]any); ok && len(args) >= 2 {
				if path, ok := args[1].(string); ok {
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatalf("read submitted script %q: %v", path, err)
					}
					capturedContent = string(data)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"job_id":"job-1","status_url":"/v1/prepare-jobs/job-1","events_url":"/v1/prepare-jobs/job-1/events"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1/events":
			w.Header().Set("Content-Type", "application/x-ndjson")
			io.WriteString(w, `{"type":"status","ts":"2026-01-24T00:00:00Z","status":"succeeded"}`+"\n")
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"succeeded","plan_only":true,"prepare_kind":"psql","image_id":"image","prepare_args_normalized":"-f prepare.sql","tasks":[{"task_id":"plan","type":"plan","planner_kind":"psql"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	err := Run([]string{"--mode", "remote", "--endpoint", server.URL, "--output", "json", "--workspace", repo, "plan", "--ref", parentRef, "chinook"})
	if err != nil {
		t.Fatalf("Run(plan alias --ref): %v", err)
	}
	if strings.ReplaceAll(capturedContent, "\r\n", "\n") != "select 1;\n" {
		t.Fatalf("submitted alias content = %q, want %q", capturedContent, "select 1;\n")
	}
}

func initPrepareRefTestRepo(t *testing.T) (string, string) {
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
		t.Skipf("git init skipped (need writable temp; run tests outside sandbox): %v\n%s", err, out)
	}
	runGit("config", "user.email", "t@e.st")
	runGit("config", "user.name", "t")

	firstScript := filepath.Join(repo, "examples", "first.sql")
	preparePath := filepath.Join(repo, "examples", "chinook", "prepare.sql")
	aliasPath := filepath.Join(repo, "examples", "chinook.prep.s9s.yaml")
	if err := os.MkdirAll(filepath.Dir(preparePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(firstScript, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preparePath, []byte("select 1;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aliasPath, []byte("kind: psql\nimage: image\nargs:\n  - -f\n  - ./first.sql\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "examples")
	runGit("commit", "-m", "first")

	secondScript := filepath.Join(repo, "examples", "second.sql")
	if err := os.WriteFile(secondScript, []byte("select 2;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preparePath, []byte("select 2;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(aliasPath, []byte("kind: psql\nimage: image\nargs:\n  - -f\n  - ./second.sql\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "examples")
	runGit("commit", "-m", "second")

	parentRef, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD^").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD^: %v", err)
	}
	return repo, strings.TrimSpace(string(parentRef))
}
