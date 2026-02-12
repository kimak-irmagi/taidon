package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"example.com", "http://example.com"},
		{"http://example.com/", "http://example.com"},
		{"https://example.com/", "https://example.com"},
	}
	for _, tc := range cases {
		if got := normalizeBaseURL(tc.raw); got != tc.want {
			t.Fatalf("normalizeBaseURL(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestListNamesAndInstances(t *testing.T) {
	var namesQuery url.Values
	var instancesQuery url.Values
	var gotAuth string
	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/names":
			namesQuery = r.URL.Query()
			gotAuth = r.Header.Get("Authorization")
			gotUA = r.Header.Get("User-Agent")
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"name":"name-1","image_id":"img","state_id":"state-1","status":"ready"}]`)
		case "/v1/instances":
			instancesQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"instance_id":"inst-1","image_id":"img","state_id":"state-1","created_at":"2026-01-01T00:00:00Z","status":"ready"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second, AuthToken: "token", UserAgent: "sqlrs-cli"})
	names, err := cli.ListNames(context.Background(), ListFilters{Instance: "inst-1", State: "state-1", Image: "img"})
	if err != nil {
		t.Fatalf("ListNames: %v", err)
	}
	if len(names) != 1 || names[0].Name != "name-1" {
		t.Fatalf("unexpected names: %+v", names)
	}
	if namesQuery.Get("instance") != "inst-1" || namesQuery.Get("state") != "state-1" || namesQuery.Get("image") != "img" {
		t.Fatalf("unexpected names query: %+v", namesQuery)
	}
	if gotAuth != "Bearer token" || gotUA != "sqlrs-cli" {
		t.Fatalf("unexpected headers: auth=%q ua=%q", gotAuth, gotUA)
	}

	instances, err := cli.ListInstances(context.Background(), ListFilters{State: "state-1", Image: "img", IDPrefix: "inst"})
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if len(instances) != 1 || instances[0].InstanceID != "inst-1" {
		t.Fatalf("unexpected instances: %+v", instances)
	}
	if instancesQuery.Get("state") != "state-1" || instancesQuery.Get("image") != "img" || instancesQuery.Get("id_prefix") != "inst" {
		t.Fatalf("unexpected instances query: %+v", instancesQuery)
	}
}

func TestGetNameAndInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/names/name-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"name":"name-1","image_id":"img","state_id":"state-1","status":"ready"}`)
		case "/v1/instances/inst-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"instance_id":"inst-1","image_id":"img","state_id":"state-1","created_at":"2026-01-01T00:00:00Z","status":"ready"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	name, found, err := cli.GetName(context.Background(), "name-1")
	if err != nil || !found {
		t.Fatalf("GetName: found=%v err=%v", found, err)
	}
	if name.Name != "name-1" {
		t.Fatalf("unexpected name: %+v", name)
	}

	inst, found, err := cli.GetInstance(context.Background(), "inst-1")
	if err != nil || !found {
		t.Fatalf("GetInstance: found=%v err=%v", found, err)
	}
	if inst.InstanceID != "inst-1" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
}

func TestPrepareJobsAndTasks(t *testing.T) {
	var jobsQuery url.Values
	var tasksQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/prepare-jobs/job-1":
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"job_id":"job-1","status":"running","prepare_kind":"psql","image_id":"img"}`)
		case "/v1/prepare-jobs":
			jobsQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"job_id":"job-1","status":"running","prepare_kind":"psql","image_id":"img"}]`)
		case "/v1/tasks":
			tasksQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `[{"task_id":"task-1","job_id":"job-1","type":"psql","status":"running"}]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	job, found, err := cli.GetPrepareJob(context.Background(), "job-1")
	if err != nil || !found {
		t.Fatalf("GetPrepareJob: found=%v err=%v", found, err)
	}
	if job.JobID != "job-1" {
		t.Fatalf("unexpected job: %+v", job)
	}

	jobs, err := cli.ListPrepareJobs(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListPrepareJobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "job-1" {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
	if jobsQuery.Get("job") != "job-1" {
		t.Fatalf("unexpected jobs query: %+v", jobsQuery)
	}

	tasks, err := cli.ListTasks(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != "task-1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	if tasksQuery.Get("job") != "job-1" {
		t.Fatalf("unexpected tasks query: %+v", tasksQuery)
	}
}

func TestDeletePrepareJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/prepare-jobs/job-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"dry_run":false,"outcome":"deleted","root":{"kind":"job","id":"job-1"}}`)
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})
	result, status, err := cli.DeletePrepareJob(context.Background(), "job-1", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeletePrepareJob: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("unexpected status: %d", status)
	}
	if result.Root.ID != "job-1" || result.Outcome != "deleted" {
		t.Fatalf("unexpected delete result: %+v", result)
	}
}

func TestConfigEndpoints(t *testing.T) {
	var gotConfigQuery url.Values
	var gotRemoveQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/config":
			if r.Method == http.MethodGet {
				gotConfigQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("path") == "" {
					io.WriteString(w, `{"dbms":{"image":"postgres"}}`)
				} else {
					io.WriteString(w, `{"path":"dbms.image","value":"postgres"}`)
				}
				return
			}
			if r.Method == http.MethodPatch {
				var req ConfigValue
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(req)
				return
			}
			if r.Method == http.MethodDelete {
				gotRemoveQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"path":"dbms.image","value":"postgres"}`)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	cli := New(server.URL, Options{Timeout: time.Second})

	value, err := cli.GetConfig(context.Background(), "", true)
	if err != nil {
		t.Fatalf("GetConfig(empty): %v", err)
	}
	if _, ok := value.(map[string]any); !ok {
		t.Fatalf("expected map response, got %T", value)
	}
	if gotConfigQuery.Get("effective") != "true" {
		t.Fatalf("expected effective=true, got %q", gotConfigQuery.Get("effective"))
	}

	value, err = cli.GetConfig(context.Background(), "dbms.image", false)
	if err != nil {
		t.Fatalf("GetConfig(path): %v", err)
	}
	if got, ok := value.(ConfigValue); !ok || got.Path != "dbms.image" {
		t.Fatalf("unexpected config value: %+v", value)
	}

	updated, err := cli.SetConfig(context.Background(), ConfigValue{Path: "dbms.image", Value: "postgres"})
	if err != nil || updated.Path != "dbms.image" {
		t.Fatalf("SetConfig: %+v err=%v", updated, err)
	}

	removed, err := cli.RemoveConfig(context.Background(), "dbms.image")
	if err != nil {
		t.Fatalf("RemoveConfig: %v", err)
	}
	if removed.Path != "dbms.image" {
		t.Fatalf("unexpected remove result: %+v", removed)
	}
	if gotRemoveQuery.Get("path") != "dbms.image" {
		t.Fatalf("unexpected remove query: %+v", gotRemoveQuery)
	}
}

func TestParseErrorResponseVariants(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       io.NopCloser(strings.NewReader(`{"message":"bad","details":"input"}`)),
	}
	if err := parseErrorResponse(resp); err == nil || err.Error() != "bad: input" {
		t.Fatalf("unexpected error: %v", err)
	}

	longBody := strings.Repeat("x", 250)
	resp = &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(strings.NewReader(longBody)),
	}
	err := parseErrorResponse(resp)
	if err == nil || !strings.Contains(err.Error(), "unexpected status") || !strings.HasSuffix(err.Error(), "...") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := parseErrorResponse(nil); err == nil || !strings.Contains(err.Error(), "missing response") {
		t.Fatalf("unexpected error: %v", err)
	}
}
