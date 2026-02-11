package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sqlrs/cli/internal/client"
	"sqlrs/cli/internal/daemon"
)

func TestPrintLsQuietSeparatesSections(t *testing.T) {
	result := LsResult{
		Names: &[]client.NameEntry{
			{
				Name:    "dev",
				ImageID: "image-1",
				StateID: "state-1",
				Status:  "active",
			},
		},
		Instances: &[]client.InstanceEntry{
			{
				InstanceID: "instance-1",
				ImageID:    "image-1",
				StateID:    "state-1",
				CreatedAt:  "2025-01-01T00:00:00Z",
				Status:     "active",
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{Quiet: true})

	out := buf.String()
	if strings.Contains(out, "Names") || strings.Contains(out, "Instances") {
		t.Fatalf("unexpected section titles in quiet output: %q", out)
	}
	if !strings.Contains(out, "\n\n") {
		t.Fatalf("expected blank line between sections, got %q", out)
	}
}

func TestRunLsRemoteRequiresEndpoint(t *testing.T) {
	_, err := RunLs(context.Background(), LsOptions{Mode: "remote", IncludeNames: true})
	if err == nil || !strings.Contains(err.Error(), "explicit endpoint") {
		t.Fatalf("expected endpoint error, got %v", err)
	}
}

func TestRunLsLocalExplicitEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "local",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || len(*result.Names) != 1 {
		t.Fatalf("unexpected names: %+v", result.Names)
	}
}

func TestRunLsLocalAutoUsesEngineState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/health":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok":true,"instanceId":"inst"}`)
		case "/v1/names":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	stateDir := t.TempDir()
	if err := daemon.WriteEngineState(filepath.Join(stateDir, "engine.json"), daemon.EngineState{
		Endpoint:   server.URL,
		AuthToken:  "token",
		InstanceID: "inst",
	}); err != nil {
		t.Fatalf("WriteEngineState: %v", err)
	}

	opts := LsOptions{
		Mode:         "local",
		Endpoint:     "",
		StateDir:     stateDir,
		Timeout:      time.Second,
		IncludeNames: true,
		Verbose:      true,
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || len(*result.Names) != 1 {
		t.Fatalf("unexpected names: %+v", result.Names)
	}
}

func TestRunLsUsesNameItemEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"dev","image_id":"img","state_id":"state","status":"active"}`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
		FilterName:   "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/names/dev" {
		t.Fatalf("expected name item endpoint, got %q", gotPath)
	}
	if result.Names == nil || len(*result.Names) != 1 {
		t.Fatalf("unexpected names result: %+v", result.Names)
	}
}

func TestRunLsUsesInstanceItemEndpoint(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          3 * time.Second,
		IncludeInstances: true,
		FilterName:       "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/instances/dev" {
		t.Fatalf("expected instance item endpoint, got %q", gotPath)
	}
	if result.Instances == nil || len(*result.Instances) != 1 {
		t.Fatalf("unexpected instances result: %+v", result.Instances)
	}
}

func TestRunLsUsesInstancePrefixListEndpoint(t *testing.T) {
	var gotPath string
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"instance_id":"abc123456789abcd","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterInstance:   "abc12345",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if gotPath != "/v1/instances" {
		t.Fatalf("expected list instances endpoint, got %q", gotPath)
	}
	if !strings.Contains(gotQuery, "id_prefix=abc12345") {
		t.Fatalf("expected id_prefix in query, got %q", gotQuery)
	}
	if result.Instances == nil || len(*result.Instances) != 1 {
		t.Fatalf("unexpected instances result: %+v", result.Instances)
	}
}

func TestRunLsStatePrefixNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeNames:     true,
		IncludeInstances: true,
		IncludeStates:    true,
		FilterState:      "deadbeef",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || result.Instances == nil || result.States == nil {
		t.Fatalf("expected all sections, got %+v", result)
	}
	if len(*result.Names) != 0 || len(*result.Instances) != 0 || len(*result.States) != 0 {
		t.Fatalf("expected empty results, got %+v", result)
	}
}

func TestRunLsListsAll(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/names":
			io.WriteString(w, `[{"name":"dev","image_id":"img","state_id":"state","status":"active"}]`)
		case "/v1/instances":
			io.WriteString(w, `[{"instance_id":"abc","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case "/v1/states":
			io.WriteString(w, `[{"state_id":"state","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"","created_at":"2025-01-01T00:00:00Z","refcount":0}]`)
		case "/v1/prepare-jobs":
			io.WriteString(w, `[{"job_id":"job-1","status":"queued","prepare_kind":"psql","image_id":"img"}]`)
		case "/v1/tasks":
			io.WriteString(w, `[{"task_id":"plan","job_id":"job-1","type":"plan","status":"queued"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeNames:     true,
		IncludeInstances: true,
		IncludeStates:    true,
		IncludeJobs:      true,
		IncludeTasks:     true,
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || result.Instances == nil || result.States == nil || result.Jobs == nil || result.Tasks == nil {
		t.Fatalf("expected all sections, got %+v", result)
	}
	if len(*result.Names) != 1 || len(*result.Instances) != 1 || len(*result.States) != 1 || len(*result.Jobs) != 1 || len(*result.Tasks) != 1 {
		t.Fatalf("unexpected results: %+v", result)
	}
}

func TestRunLsNameNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names/missing" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
		FilterName:   "missing",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || len(*result.Names) != 0 {
		t.Fatalf("expected empty names, got %+v", result.Names)
	}
}

func TestRunLsInstanceFromNameNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances/dev" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterName:       "dev",
	}
	result, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Instances == nil || len(*result.Instances) != 0 {
		t.Fatalf("expected empty instances, got %+v", result.Instances)
	}
}

func TestPrintLsStatesTable(t *testing.T) {
	size := int64(42)
	result := LsResult{
		States: &[]client.StateEntry{
			{
				StateID:     "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				ImageID:     "image-1",
				PrepareKind: "psql",
				PrepareArgs: "-c select 1",
				CreatedAt:   "2025-01-01T00:00:00Z",
				SizeBytes:   &size,
				RefCount:    2,
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{NoHeader: true, LongIDs: true})
	out := buf.String()
	if !strings.Contains(out, "psql") || !strings.Contains(out, "42") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRunLsTasksJobFilter(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	opts := LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeTasks: true,
		FilterJob:    "job-1",
	}
	_, err := RunLs(context.Background(), opts)
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if !strings.Contains(gotQuery, "job=job-1") {
		t.Fatalf("expected job filter in query, got %q", gotQuery)
	}
}

func TestPrintLsJobsAndTasksTables(t *testing.T) {
	result := LsResult{
		Jobs: &[]client.PrepareJobEntry{
			{
				JobID:       "job-1",
				Status:      "queued",
				PrepareKind: "psql",
				ImageID:     "image-1",
			},
		},
		Tasks: &[]client.TaskEntry{
			{
				TaskID: "plan",
				JobID:  "job-1",
				Type:   "plan",
				Status: "queued",
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{})
	out := buf.String()
	if !strings.Contains(out, "Jobs") || !strings.Contains(out, "Tasks") {
		t.Fatalf("expected jobs/tasks headers, got %q", out)
	}
	if !strings.Contains(out, "JOB_ID") || !strings.Contains(out, "TASK_ID") {
		t.Fatalf("expected table headers, got %q", out)
	}
}

func TestOptionalInt64(t *testing.T) {
	if optionalInt64(nil) != "" {
		t.Fatalf("expected empty string for nil")
	}
	value := int64(5)
	if optionalInt64(&value) != "5" {
		t.Fatalf("expected formatted value")
	}
}

func TestResolvedMatchValueOr(t *testing.T) {
	match := resolvedMatch{value: "state-1"}
	if match.valueOr("fallback") != "state-1" {
		t.Fatalf("expected match value")
	}
	empty := resolvedMatch{}
	if empty.valueOr("fallback") != "fallback" {
		t.Fatalf("expected fallback value")
	}
}

func TestNormalizeHexPrefix(t *testing.T) {
	if _, err := normalizeHexPrefix("abc"); err == nil {
		t.Fatalf("expected error for short prefix")
	}
	if _, err := normalizeHexPrefix("bad!1234"); err == nil {
		t.Fatalf("expected error for non-hex prefix")
	}
	value, err := normalizeHexPrefix("AbCdEf12")
	if err != nil {
		t.Fatalf("normalizeHexPrefix: %v", err)
	}
	if value != "abcdef12" {
		t.Fatalf("expected lowercase prefix, got %q", value)
	}
}

func TestOptionalString(t *testing.T) {
	if optionalString(nil) != "" {
		t.Fatalf("expected empty string for nil")
	}
	value := "value"
	if optionalString(&value) != "value" {
		t.Fatalf("expected value to be returned")
	}
}

func TestFormatBool(t *testing.T) {
	if formatBool(true) != "true" {
		t.Fatalf("expected true")
	}
	if formatBool(false) != "false" {
		t.Fatalf("expected false")
	}
}

func TestFormatIDPtr(t *testing.T) {
	if formatIDPtr(nil, false) != "" {
		t.Fatalf("expected empty id for nil")
	}
	value := "abcdef1234567890"
	if formatIDPtr(&value, false) != "abcdef123456" {
		t.Fatalf("unexpected formatted id")
	}
}

func TestPrintNamesTableUsesFingerprint(t *testing.T) {
	row := client.NameEntry{
		Name:             "dev",
		StateFingerprint: "state-fp",
		Status:           "active",
	}
	var buf bytes.Buffer
	printNamesTable(&buf, []client.NameEntry{row}, true, false)
	if !strings.Contains(buf.String(), "state-fp") {
		t.Fatalf("expected fingerprint output, got %q", buf.String())
	}
}

func TestResolveStatePrefixAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"state_id":"deadbeef00000000","image_id":"img"},{"state_id":"deadbeef11111111","image_id":"img"}]`)
	}))
	defer server.Close()

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := resolveStatePrefix(context.Background(), cliClient, "deadbeef", "", "")
	var ambErr *AmbiguousPrefixError
	if !errors.As(err, &ambErr) || ambErr.Kind != "state" {
		t.Fatalf("expected ambiguous state prefix error, got %v", err)
	}
}

func TestResolveStatePrefixMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"state_id":"deadbeef00000000","image_id":"img"}]`)
	}))
	defer server.Close()

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	match, err := resolveStatePrefix(context.Background(), cliClient, "deadbeef", "", "")
	if err != nil {
		t.Fatalf("resolveStatePrefix: %v", err)
	}
	if match.value != "deadbeef00000000" {
		t.Fatalf("unexpected match: %+v", match)
	}
}

func TestResolveInstancePrefixNoMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	match, err := resolveInstancePrefix(context.Background(), cliClient, "deadbeef", "", "")
	if err != nil {
		t.Fatalf("resolveInstancePrefix: %v", err)
	}
	if !match.noMatch {
		t.Fatalf("expected noMatch, got %+v", match)
	}
}

func TestResolveInstancePrefixAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"instance_id":"deadbeef00000000","image_id":"img"},{"instance_id":"deadbeef11111111","image_id":"img"}]`)
	}))
	defer server.Close()

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := resolveInstancePrefix(context.Background(), cliClient, "deadbeef", "", "")
	var ambErr *AmbiguousPrefixError
	if !errors.As(err, &ambErr) || ambErr.Kind != "instance" {
		t.Fatalf("expected ambiguous instance prefix error, got %v", err)
	}
}

func TestRunLsLocalAutostartDisabled(t *testing.T) {
	_, err := RunLs(context.Background(), LsOptions{
		Mode:          "local",
		Endpoint:      "",
		StateDir:      t.TempDir(),
		Autostart:     false,
		Timeout:       time.Second,
		IncludeNames:  true,
		IncludeStates: true,
	})
	if err == nil || !strings.Contains(err.Error(), "local engine is not running") {
		t.Fatalf("expected autostart error, got %v", err)
	}
}

func TestRunLsRemoteVerbose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/names" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
		Verbose:      true,
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil {
		t.Fatalf("expected names result")
	}
}

func TestRunLsGetNameError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunLs(context.Background(), LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
		FilterName:   "dev",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLsListNamesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunLs(context.Background(), LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeNames: true,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLsInstanceMatchNoMatchFromState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterState:      "deadbeef",
		FilterInstance:   "deadbeef",
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Instances == nil || len(*result.Instances) != 0 {
		t.Fatalf("expected empty instances, got %+v", result.Instances)
	}
}

func TestRunLsInstanceFromNameFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances/dev" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"instance_id":"inst","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}`)
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterName:       "dev",
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Instances == nil || len(*result.Instances) != 1 {
		t.Fatalf("expected instance result, got %+v", result.Instances)
	}
}

func TestRunLsStateResolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"state_id":"deadbeef00000000","image_id":"img","prepare_kind":"psql","prepare_args_normalized":"","created_at":"2025-01-01T00:00:00Z","refcount":0}]`)
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:          "remote",
		Endpoint:      server.URL,
		Timeout:       time.Second,
		IncludeStates: true,
		FilterState:   "deadbeef",
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.States == nil || len(*result.States) != 1 {
		t.Fatalf("expected state result, got %+v", result.States)
	}
}

func TestRunLsListStatesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunLs(context.Background(), LsOptions{
		Mode:          "remote",
		Endpoint:      server.URL,
		Timeout:       time.Second,
		IncludeStates: true,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLsJobsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunLs(context.Background(), LsOptions{
		Mode:        "remote",
		Endpoint:    server.URL,
		Timeout:     time.Second,
		IncludeJobs: true,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLsTasksError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunLs(context.Background(), LsOptions{
		Mode:         "remote",
		Endpoint:     server.URL,
		Timeout:      time.Second,
		IncludeTasks: true,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestPrintLsMultipleSections(t *testing.T) {
	result := LsResult{
		Names: &[]client.NameEntry{
			{
				Name:    "dev",
				ImageID: "img",
				StateID: "state",
				Status:  "active",
			},
		},
		States: &[]client.StateEntry{
			{
				StateID:     "state",
				ImageID:     "img",
				PrepareKind: "psql",
				PrepareArgs: "-c select 1",
				CreatedAt:   "2025-01-01T00:00:00Z",
				RefCount:    1,
			},
		},
		Jobs: &[]client.PrepareJobEntry{
			{
				JobID:       "job-1",
				Status:      "queued",
				PrepareKind: "psql",
				ImageID:     "img",
			},
		},
	}

	var buf bytes.Buffer
	PrintLs(&buf, result, LsPrintOptions{})
	out := buf.String()
	if !strings.Contains(out, "Names") || !strings.Contains(out, "States") || !strings.Contains(out, "Jobs") {
		t.Fatalf("expected section headers, got %q", out)
	}
	if !strings.Contains(out, "STATE_ID") {
		t.Fatalf("expected states header, got %q", out)
	}
	if !strings.Contains(out, "\n\n") {
		t.Fatalf("expected blank line between sections, got %q", out)
	}
}

func TestResolveStatePrefixInvalidPrefix(t *testing.T) {
	cliClient := client.New("http://127.0.0.1:1", client.Options{Timeout: time.Second})
	_, err := resolveStatePrefix(context.Background(), cliClient, "bad", "", "")
	var prefixErr *IDPrefixError
	if !errors.As(err, &prefixErr) {
		t.Fatalf("expected prefix error, got %v", err)
	}
}

func TestResolveStatePrefixListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := resolveStatePrefix(context.Background(), cliClient, "deadbeef", "", "")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveInstancePrefixInvalidPrefix(t *testing.T) {
	cliClient := client.New("http://127.0.0.1:1", client.Options{Timeout: time.Second})
	_, err := resolveInstancePrefix(context.Background(), cliClient, "bad", "", "")
	var prefixErr *IDPrefixError
	if !errors.As(err, &prefixErr) {
		t.Fatalf("expected prefix error, got %v", err)
	}
}

func TestResolveInstancePrefixListError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cliClient := client.New(server.URL, client.Options{Timeout: time.Second})
	_, err := resolveInstancePrefix(context.Background(), cliClient, "deadbeef", "", "")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLsNamesResolvedFilters(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/states":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"state_id":"state-1","image_id":"img"}]`)
		case "/v1/instances":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"instance_id":"inst-1","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
		case "/v1/names":
			gotQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `null`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:           "remote",
		Endpoint:       server.URL,
		Timeout:        time.Second,
		IncludeNames:   true,
		FilterState:    "deadbeef",
		FilterInstance: "deadbeef",
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Names == nil || len(*result.Names) != 0 {
		t.Fatalf("expected empty names, got %+v", result.Names)
	}
	if !strings.Contains(gotQuery, "state=state-1") || !strings.Contains(gotQuery, "instance=inst-1") {
		t.Fatalf("unexpected query: %q", gotQuery)
	}
}

func TestRunLsVerboseNilLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `null`)
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		IncludeStates:    true,
		IncludeJobs:      true,
		IncludeTasks:     true,
		Verbose:          true,
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Instances == nil || result.States == nil || result.Jobs == nil || result.Tasks == nil {
		t.Fatalf("expected empty slices, got %+v", result)
	}
}

func TestRunLsListInstancesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunLs(context.Background(), LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunLsFilterInstanceEmptyID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/instances" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "id_prefix") {
			io.WriteString(w, `[{"instance_id":"","image_id":"img","state_id":"state","created_at":"2025-01-01T00:00:00Z","status":"active"}]`)
			return
		}
		io.WriteString(w, `null`)
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterInstance:   "deadbeef",
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.Instances == nil || len(*result.Instances) != 0 {
		t.Fatalf("expected empty instances, got %+v", result.Instances)
	}
}

func TestRunLsFilterStateEmptyID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.RawQuery, "id_prefix") {
			io.WriteString(w, `[{"state_id":"","image_id":"img"}]`)
			return
		}
		io.WriteString(w, `null`)
	}))
	defer server.Close()

	result, err := RunLs(context.Background(), LsOptions{
		Mode:          "remote",
		Endpoint:      server.URL,
		Timeout:       time.Second,
		IncludeStates: true,
		FilterState:   "deadbeef",
	})
	if err != nil {
		t.Fatalf("RunLs: %v", err)
	}
	if result.States == nil || len(*result.States) != 0 {
		t.Fatalf("expected empty states, got %+v", result.States)
	}
}

func TestRunLsGetInstanceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := RunLs(context.Background(), LsOptions{
		Mode:             "remote",
		Endpoint:         server.URL,
		Timeout:          time.Second,
		IncludeInstances: true,
		FilterName:       "dev",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}
