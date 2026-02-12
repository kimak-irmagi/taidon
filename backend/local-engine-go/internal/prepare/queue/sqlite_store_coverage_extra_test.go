package queue

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestOpenMkdirAllError(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := osWriteFile(blocker, []byte("x")); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	if _, err := Open(filepath.Join(blocker, "state.db")); err == nil {
		t.Fatalf("expected mkdir error")
	}
}

func TestOpenSQLDriverError(t *testing.T) {
	orig := sqlOpenFn
	sqlOpenFn = func(string, string) (*sql.DB, error) {
		return nil, errors.New("open failed")
	}
	t.Cleanup(func() { sqlOpenFn = orig })

	if _, err := Open(filepath.Join(t.TempDir(), "state.db")); err == nil {
		t.Fatalf("expected sql open error")
	}
}

func TestOpenInitDBError(t *testing.T) {
	orig := sqlOpenFn
	sqlOpenFn = func(string, string) (*sql.DB, error) {
		return openErrorDB(t, []error{errors.New("boom")}), nil
	}
	t.Cleanup(func() { sqlOpenFn = orig })

	if _, err := Open(filepath.Join(t.TempDir(), "state.db")); err == nil {
		t.Fatalf("expected initDB error")
	}
}

func TestNewInitDBError(t *testing.T) {
	db := openErrorDB(t, []error{errors.New("boom")})
	if _, err := New(db); err == nil {
		t.Fatalf("expected initDB error")
	}
}

func TestGetJobReturnsDBError(t *testing.T) {
	store := newQueueStore(t)
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, _, err := store.GetJob(context.Background(), "job-1"); err == nil {
		t.Fatalf("expected db error")
	}
}

func TestAppendEventExecError(t *testing.T) {
	db := openErrorDB(t, []error{errors.New("boom")})
	store := &SQLiteStore{db: db}
	if _, err := store.AppendEvent(context.Background(), EventRecord{JobID: "job-1", Type: "status", Ts: "t"}); err == nil {
		t.Fatalf("expected exec error")
	}
}

func TestEnsureJobSignatureColumnExecError(t *testing.T) {
	db := openErrorDB(t, []error{errors.New("boom")})
	if err := ensureJobSignatureColumn(db); err == nil {
		t.Fatalf("expected exec error")
	}
}

func TestUpdateTaskAllFields(t *testing.T) {
	store := newQueueStore(t)
	if err := store.CreateJob(context.Background(), JobRecord{
		JobID:       "job-1",
		Status:      "queued",
		PrepareKind: "psql",
		ImageID:     "image-1",
		CreatedAt:   "2026-01-19T00:00:00Z",
	}); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if err := store.ReplaceTasks(context.Background(), "job-1", []TaskRecord{
		{JobID: "job-1", TaskID: "task-1", Position: 0, Type: "plan", Status: "queued"},
	}); err != nil {
		t.Fatalf("ReplaceTasks: %v", err)
	}
	status := "running"
	started := "2026-01-19T00:01:00Z"
	finished := "2026-01-19T00:02:00Z"
	errorJSON := `{"err":"fail"}`
	taskHash := "hash-1"
	outputState := "state-1"
	cachedTrue := true
	if err := store.UpdateTask(context.Background(), "job-1", "task-1", TaskUpdate{
		Status:       &status,
		StartedAt:    &started,
		FinishedAt:   &finished,
		ErrorJSON:    &errorJSON,
		TaskHash:     &taskHash,
		OutputStateID: &outputState,
		Cached:       &cachedTrue,
	}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	cachedFalse := false
	if err := store.UpdateTask(context.Background(), "job-1", "task-1", TaskUpdate{
		Cached: &cachedFalse,
	}); err != nil {
		t.Fatalf("UpdateTask cached false: %v", err)
	}
}

func TestListJobsScanAndRowsErrors(t *testing.T) {
	store := newQueryErrorStore(t, "scan-error-driver")
	if _, err := store.ListJobs(context.Background(), ""); err == nil {
		t.Fatalf("expected scan error")
	}
	if _, _, err := store.GetJob(context.Background(), "job-1"); err == nil {
		t.Fatalf("expected scan error for GetJob")
	}
	if _, err := store.ListTasks(context.Background(), ""); err == nil {
		t.Fatalf("expected scan error for ListTasks")
	}
	if _, err := store.ListJobsByStatus(context.Background(), []string{"queued"}); err == nil {
		t.Fatalf("expected scan error for ListJobsByStatus")
	}
	if _, err := store.ListJobsBySignature(context.Background(), "sig", nil); err == nil {
		t.Fatalf("expected scan error for ListJobsBySignature")
	}
	if _, err := store.ListEventsSince(context.Background(), "job-1", 0); err == nil {
		t.Fatalf("expected scan error for ListEventsSince")
	}
}

func TestListJobsRowsErr(t *testing.T) {
	store := newQueryErrorStore(t, "rows-error-driver")
	if _, err := store.ListJobs(context.Background(), ""); err == nil {
		t.Fatalf("expected rows error")
	}
	if _, err := store.ListTasks(context.Background(), ""); err == nil {
		t.Fatalf("expected rows error for ListTasks")
	}
	if _, err := store.ListJobsByStatus(context.Background(), []string{"queued"}); err == nil {
		t.Fatalf("expected rows error for ListJobsByStatus")
	}
	if _, err := store.ListJobsBySignature(context.Background(), "sig", nil); err == nil {
		t.Fatalf("expected rows error for ListJobsBySignature")
	}
	if _, err := store.ListEventsSince(context.Background(), "job-1", 0); err == nil {
		t.Fatalf("expected rows error for ListEventsSince")
	}
}

func TestNullBoolFalse(t *testing.T) {
	if out := nullBool(boolPtrFromValue(false)); !out.Valid || out.Int64 != 0 {
		t.Fatalf("unexpected nullBool false: %+v", out)
	}
}

type queryErrorDriver struct{}

type queryErrorConn struct {
	mode string
}

type scanErrorRows struct {
	returned bool
}

type rowsErrRows struct{}

var queryErrorOnce sync.Once

func registerQueryErrorDrivers() {
	queryErrorOnce.Do(func() {
		sql.Register("scan-error-driver", queryErrorDriver{})
		sql.Register("rows-error-driver", queryErrorDriver{})
	})
}

func newQueryErrorStore(t *testing.T, driverName string) *SQLiteStore {
	t.Helper()
	registerQueryErrorDrivers()
	db, err := sql.Open(driverName, driverName)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &SQLiteStore{db: db}
}

func (queryErrorDriver) Open(name string) (driver.Conn, error) {
	return &queryErrorConn{mode: name}, nil
}

func (c *queryErrorConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}

func (c *queryErrorConn) Close() error {
	return nil
}

func (c *queryErrorConn) Begin() (driver.Tx, error) {
	return nil, errors.New("tx not supported")
}

func (c *queryErrorConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

func (c *queryErrorConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	switch c.mode {
	case "scan-error-driver":
		return &scanErrorRows{}, nil
	default:
		return &rowsErrRows{}, nil
	}
}

func (r *scanErrorRows) Columns() []string {
	return []string{"col"}
}

func (r *scanErrorRows) Close() error {
	return nil
}

func (r *scanErrorRows) Next(dest []driver.Value) error {
	if r.returned {
		return io.EOF
	}
	r.returned = true
	dest[0] = "value"
	return nil
}

func (r *rowsErrRows) Columns() []string {
	return []string{"col"}
}

func (r *rowsErrRows) Close() error {
	return nil
}

func (r *rowsErrRows) Next(dest []driver.Value) error {
	return errors.New("rows failed")
}

func osWriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
