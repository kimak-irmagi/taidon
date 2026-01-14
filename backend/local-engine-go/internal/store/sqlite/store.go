package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"sqlrs/engine/internal/store"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := initDB(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) ListNames(ctx context.Context, filters store.NameFilters) ([]store.NameEntry, error) {
	query := strings.Builder{}
	query.WriteString(`
SELECT n.name, n.instance_id, n.image_id, n.state_id, n.state_fingerprint, n.last_used_at, i.expires_at, i.instance_id
FROM names n
LEFT JOIN instances i ON i.instance_id = n.instance_id
WHERE 1=1`)
	args := []any{}
	addFilter(&query, &args, "n.instance_id", filters.InstanceID)
	addFilter(&query, &args, "n.state_id", filters.StateID)
	addFilter(&query, &args, "n.image_id", filters.ImageID)
	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var out []store.NameEntry
	for rows.Next() {
		var name string
		var instanceID sql.NullString
		var imageID string
		var stateID sql.NullString
		var stateFingerprint string
		var lastUsedAt sql.NullString
		var expiresAt sql.NullString
		var joinedInstanceID sql.NullString
		if err := rows.Scan(&name, &instanceID, &imageID, &stateID, &stateFingerprint, &lastUsedAt, &expiresAt, &joinedInstanceID); err != nil {
			return nil, err
		}
		entry := store.NameEntry{
			Name:             name,
			ImageID:          imageID,
			StateFingerprint: stateFingerprint,
			Status:           store.NameStatusActive,
		}
		if instanceID.Valid {
			entry.InstanceID = strPtr(instanceID.String)
		}
		if lastUsedAt.Valid {
			entry.LastUsedAt = strPtr(lastUsedAt.String)
		}
		entry.StateID = stateFingerprint
		if stateID.Valid && stateID.String != "" {
			entry.StateID = stateID.String
		}
		if instanceID.Valid && joinedInstanceID.Valid {
			if isExpired(expiresAt, now) {
				entry.Status = store.NameStatusExpired
			}
		} else {
			entry.Status = store.NameStatusMissing
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GetName(ctx context.Context, name string) (store.NameEntry, bool, error) {
	query := `
SELECT n.name, n.instance_id, n.image_id, n.state_id, n.state_fingerprint, n.last_used_at, i.expires_at, i.instance_id
FROM names n
LEFT JOIN instances i ON i.instance_id = n.instance_id
WHERE n.name = ?`
	row := s.db.QueryRowContext(ctx, query, name)
	now := time.Now().UTC()
	entry, err := scanName(row, now)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.NameEntry{}, false, nil
		}
		return store.NameEntry{}, false, err
	}
	return entry, true, nil
}

func (s *Store) ListInstances(ctx context.Context, filters store.InstanceFilters) ([]store.InstanceEntry, error) {
	query := strings.Builder{}
	query.WriteString(`
SELECT i.instance_id, i.image_id, i.state_id, i.created_at, i.expires_at,
       pn.name,
       (SELECT COUNT(1) FROM names n WHERE n.instance_id = i.instance_id) as name_count
FROM instances i
LEFT JOIN names pn ON pn.instance_id = i.instance_id AND pn.is_primary = 1
WHERE 1=1`)
	args := []any{}
	addFilter(&query, &args, "i.state_id", filters.StateID)
	addFilter(&query, &args, "i.image_id", filters.ImageID)
	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var out []store.InstanceEntry
	for rows.Next() {
		entry, err := scanInstance(rows, now)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GetInstance(ctx context.Context, instanceID string) (store.InstanceEntry, bool, error) {
	query := `
SELECT i.instance_id, i.image_id, i.state_id, i.created_at, i.expires_at,
       pn.name,
       (SELECT COUNT(1) FROM names n WHERE n.instance_id = i.instance_id) as name_count
FROM instances i
LEFT JOIN names pn ON pn.instance_id = i.instance_id AND pn.is_primary = 1
WHERE i.instance_id = ?`
	row := s.db.QueryRowContext(ctx, query, instanceID)
	entry, err := scanInstance(row, time.Now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.InstanceEntry{}, false, nil
		}
		return store.InstanceEntry{}, false, err
	}
	return entry, true, nil
}

func (s *Store) ListStates(ctx context.Context, filters store.StateFilters) ([]store.StateEntry, error) {
	query := strings.Builder{}
	query.WriteString(`
SELECT s.state_id, s.image_id, s.prepare_kind, s.prepare_args_normalized, s.created_at, s.size_bytes,
       (SELECT COUNT(1) FROM instances i WHERE i.state_id = s.state_id) as refcount
FROM states s
WHERE 1=1`)
	args := []any{}
	addFilter(&query, &args, "s.prepare_kind", filters.Kind)
	addFilter(&query, &args, "s.image_id", filters.ImageID)
	rows, err := s.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.StateEntry
	for rows.Next() {
		var entry store.StateEntry
		if err := rows.Scan(&entry.StateID, &entry.ImageID, &entry.PrepareKind, &entry.PrepareArgs, &entry.CreatedAt, &entry.SizeBytes, &entry.RefCount); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) GetState(ctx context.Context, stateID string) (store.StateEntry, bool, error) {
	query := `
SELECT s.state_id, s.image_id, s.prepare_kind, s.prepare_args_normalized, s.created_at, s.size_bytes,
       (SELECT COUNT(1) FROM instances i WHERE i.state_id = s.state_id) as refcount
FROM states s
WHERE s.state_id = ?`
	row := s.db.QueryRowContext(ctx, query, stateID)
	var entry store.StateEntry
	if err := row.Scan(&entry.StateID, &entry.ImageID, &entry.PrepareKind, &entry.PrepareArgs, &entry.CreatedAt, &entry.SizeBytes, &entry.RefCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.StateEntry{}, false, nil
		}
		return store.StateEntry{}, false, err
	}
	return entry, true, nil
}

func initDB(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	_, err := db.Exec(SchemaSQL())
	return err
}

func addFilter(query *strings.Builder, args *[]any, column, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	query.WriteString(" AND ")
	query.WriteString(column)
	query.WriteString(" = ?")
	*args = append(*args, value)
}

func scanName(scanner interface {
	Scan(dest ...any) error
}, now time.Time) (store.NameEntry, error) {
	var name string
	var instanceID sql.NullString
	var imageID string
	var stateID sql.NullString
	var stateFingerprint string
	var lastUsedAt sql.NullString
	var expiresAt sql.NullString
	var joinedInstanceID sql.NullString
	if err := scanner.Scan(&name, &instanceID, &imageID, &stateID, &stateFingerprint, &lastUsedAt, &expiresAt, &joinedInstanceID); err != nil {
		return store.NameEntry{}, err
	}
	entry := store.NameEntry{
		Name:             name,
		ImageID:          imageID,
		StateFingerprint: stateFingerprint,
		Status:           store.NameStatusActive,
		StateID:          stateFingerprint,
	}
	if instanceID.Valid {
		entry.InstanceID = strPtr(instanceID.String)
	}
	if lastUsedAt.Valid {
		entry.LastUsedAt = strPtr(lastUsedAt.String)
	}
	if stateID.Valid && stateID.String != "" {
		entry.StateID = stateID.String
	}
	if instanceID.Valid && joinedInstanceID.Valid {
		if isExpired(expiresAt, now) {
			entry.Status = store.NameStatusExpired
		}
	} else {
		entry.Status = store.NameStatusMissing
	}
	return entry, nil
}

func scanInstance(scanner interface {
	Scan(dest ...any) error
}, now time.Time) (store.InstanceEntry, error) {
	var entry store.InstanceEntry
	var expiresAt sql.NullString
	var name sql.NullString
	var nameCount int
	if err := scanner.Scan(&entry.InstanceID, &entry.ImageID, &entry.StateID, &entry.CreatedAt, &expiresAt, &name, &nameCount); err != nil {
		return store.InstanceEntry{}, err
	}
	entry.Status = store.InstanceStatusActive
	if expiresAt.Valid {
		entry.ExpiresAt = strPtr(expiresAt.String)
		if isExpired(expiresAt, now) {
			entry.Status = store.InstanceStatusExpired
		}
	}
	if name.Valid {
		entry.Name = strPtr(name.String)
	}
	if entry.Status != store.InstanceStatusExpired && nameCount == 0 {
		entry.Status = store.InstanceStatusOrphaned
	}
	return entry, nil
}

func isExpired(expiresAt sql.NullString, now time.Time) bool {
	if !expiresAt.Valid || expiresAt.String == "" {
		return false
	}
	parsed, ok := parseTime(expiresAt.String)
	if !ok {
		return false
	}
	return parsed.Before(now)
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts, true
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts, true
	}
	return time.Time{}, false
}

func strPtr(value string) *string {
	return &value
}
