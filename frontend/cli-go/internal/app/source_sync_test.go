package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sqlrs/cli/internal/cli"
	"github.com/sqlrs/cli/internal/client"
	"github.com/sqlrs/cli/internal/inputset"
	"github.com/sqlrs/cli/internal/refctx"
	"github.com/sqlrs/cli/internal/remotesource"
)

func TestExplainPrepareCacheWithSourceSyncRetriesAndUploads(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "query.sql"), []byte("select 1;\n"), 0o600); err != nil {
		t.Fatalf("write query.sql: %v", err)
	}
	hash := "sha256:" + inputset.HashContent([]byte("select 1;\n"))
	digest := strings.TrimPrefix(hash, "sha256:")
	postCount := 0
	putCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/cache/explain/prepare":
			postCount++
			var req client.PrepareJobRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode prepare request: %v", err)
			}
			if req.SourceManifest == nil {
				t.Fatal("expected source_manifest on cache explain request")
			}
			if postCount == 1 {
				w.WriteHeader(http.StatusConflict)
				io.WriteString(w, `{"code":"source_inputs_missing","message":"missing","missing_blobs":[{"path":"query.sql","hash":"`+hash+`"}]}`)
				return
			}
			if got := req.SourceManifest.Files["query.sql"]; got != hash {
				t.Fatalf("manifest hash = %q, want %q", got, hash)
			}
			io.WriteString(w, `{"decision":"hit","reason_code":"exact_state_match","signature":"sig"}`)
		case "/v1/source-blobs/sha256/" + digest:
			putCount++
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read blob: %v", err)
			}
			if string(data) != "select 1;\n" {
				t.Fatalf("uploaded body = %q", string(data))
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	var progress bytes.Buffer
	got, err := explainPrepareCache(context.Background(), cli.PrepareOptions{
		Mode:        "remote",
		Endpoint:    server.URL,
		ImageID:     "postgres:16",
		PrepareKind: "psql",
		PsqlArgs:    []string{"-f", "query.sql"},
		SourceSync: &remotesource.Options{
			Enabled:       true,
			MaxRounds:     2,
			WorkspaceRoot: root,
			Progress:      &progress,
		},
	})
	if err != nil {
		t.Fatalf("explainPrepareCache: %v", err)
	}
	if got.Decision != "hit" || got.Signature != "sig" || postCount != 2 || putCount != 1 {
		t.Fatalf("response=%+v postCount=%d putCount=%d", got, postCount, putCount)
	}
	if !strings.Contains(progress.String(), "source sync: round 1") || !strings.Contains(progress.String(), "uploaded 1 blobs") {
		t.Fatalf("progress = %q", progress.String())
	}
}

func TestBuildRemoteSourceSyncOptionsModesAndRefContext(t *testing.T) {
	root := t.TempDir()
	refRoot := t.TempDir()
	refFS := sentinelSourceFS{}

	got, err := buildRemoteSourceSyncOptions(io.Discard, cli.PrepareOptions{
		Mode:                "remote",
		ProfileName:         "remote",
		SourceSyncMaxRounds: 3,
	}, stageRunRequest{
		workspaceRoot: root,
		cwd:           t.TempDir(),
		invocationCwd: t.TempDir(),
	}, &refctx.Context{
		WorkspaceRoot: refRoot,
		FileSystem:    refFS,
	})
	if err != nil {
		t.Fatalf("buildRemoteSourceSyncOptions: %v", err)
	}
	if got == nil || !got.Enabled || got.WorkspaceRoot != refRoot || got.WorkspaceID != "remote" || got.MaxRounds != 3 {
		t.Fatalf("unexpected source sync options: %+v", got)
	}
	if got.FileSystem != refFS {
		t.Fatalf("source sync should use ref filesystem, got %#v", got.FileSystem)
	}

	off, err := buildRemoteSourceSyncOptions(io.Discard, cli.PrepareOptions{Mode: "remote", SourceSyncMode: "off"}, stageRunRequest{workspaceRoot: root}, nil)
	if err != nil {
		t.Fatalf("sourceSync.mode off: %v", err)
	}
	if off != nil {
		t.Fatalf("sourceSync.mode off returned %+v, want nil", off)
	}

	local, err := buildRemoteSourceSyncOptions(io.Discard, cli.PrepareOptions{Mode: "local"}, stageRunRequest{workspaceRoot: root}, nil)
	if err != nil {
		t.Fatalf("local mode: %v", err)
	}
	if local != nil {
		t.Fatalf("local mode returned %+v, want nil", local)
	}

	_, err = buildRemoteSourceSyncOptions(io.Discard, cli.PrepareOptions{Mode: "remote", SourceSyncMode: "manual"}, stageRunRequest{workspaceRoot: root}, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported sourceSync.mode") {
		t.Fatalf("expected unsupported sourceSync.mode error, got %v", err)
	}
}

type sentinelSourceFS struct{}

func (sentinelSourceFS) Stat(string) (fs.FileInfo, error) {
	return nil, errors.New("unused")
}

func (sentinelSourceFS) ReadFile(string) ([]byte, error) {
	return nil, errors.New("unused")
}

func (sentinelSourceFS) ReadDir(string) ([]fs.DirEntry, error) {
	return nil, errors.New("unused")
}
