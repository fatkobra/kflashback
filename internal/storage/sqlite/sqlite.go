// Copyright 2024 The kflashback Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/kflashback/kflashback/internal/storage"
)

func init() {
	storage.RegisterBackend("sqlite", func(dsn string) (storage.Store, error) {
		return New(dsn)
	})
}

// Store implements storage.Store using SQLite.
type Store struct {
	db   *sql.DB
	path string
}

// New creates a new SQLite store at the given path.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=-20000")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	return &Store{db: db, path: path}, nil
}

// Initialize creates tables and indexes.
func (s *Store) Initialize(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tracked_resources (
			uid TEXT PRIMARY KEY,
			api_version TEXT NOT NULL,
			kind TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			current_revision INTEGER DEFAULT 0,
			first_seen DATETIME NOT NULL,
			last_seen DATETIME NOT NULL,
			is_deleted BOOLEAN DEFAULT FALSE,
			policy_name TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS resource_revisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_uid TEXT NOT NULL,
			api_version TEXT NOT NULL,
			kind TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			revision INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			snapshot BLOB,
			patch BLOB,
			is_snapshot BOOLEAN DEFAULT FALSE,
			resource_version TEXT NOT NULL DEFAULT '',
			changed_fields TEXT DEFAULT '',
			timestamp DATETIME NOT NULL,
			policy_name TEXT NOT NULL,
			size_bytes INTEGER DEFAULT 0,
			UNIQUE(resource_uid, revision)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_uid ON resource_revisions(resource_uid)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_uid_revision ON resource_revisions(resource_uid, revision)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_kind_ns ON resource_revisions(kind, namespace)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_timestamp ON resource_revisions(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_policy ON resource_revisions(policy_name)`,
		`CREATE INDEX IF NOT EXISTS idx_revisions_snapshot ON resource_revisions(resource_uid, is_snapshot, revision)`,
		`CREATE INDEX IF NOT EXISTS idx_tracked_kind ON tracked_resources(kind)`,
		`CREATE INDEX IF NOT EXISTS idx_tracked_ns ON tracked_resources(namespace)`,
		`CREATE INDEX IF NOT EXISTS idx_tracked_policy ON tracked_resources(policy_name)`,
	}

	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("executing schema query: %w", err)
		}
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// StoreRevision stores a new resource revision.
func (s *Store) StoreRevision(ctx context.Context, rev *storage.ResourceRevision) error {
	changedFields := strings.Join(rev.ChangedFields, ",")
	sizeBytes := int64(len(rev.Snapshot) + len(rev.Patch))

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO resource_revisions 
		(resource_uid, api_version, kind, namespace, name, revision, event_type, 
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rev.ResourceUID, rev.APIVersion, rev.Kind, rev.Namespace, rev.Name,
		rev.Revision, string(rev.EventType), rev.Snapshot, rev.Patch, rev.IsSnapshot,
		rev.ResourceVersion, changedFields, rev.Timestamp.UTC(), rev.PolicyName, sizeBytes,
	)
	if err != nil {
		return fmt.Errorf("storing revision: %w", err)
	}
	return nil
}

// GetRevision retrieves a specific revision.
func (s *Store) GetRevision(ctx context.Context, resourceUID string, revision int64) (*storage.ResourceRevision, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, resource_uid, api_version, kind, namespace, name, revision, event_type,
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes
		 FROM resource_revisions WHERE resource_uid = ? AND revision = ?`,
		resourceUID, revision,
	)
	return scanRevision(row)
}

// GetLatestRevision retrieves the latest revision for a resource.
func (s *Store) GetLatestRevision(ctx context.Context, resourceUID string) (*storage.ResourceRevision, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, resource_uid, api_version, kind, namespace, name, revision, event_type,
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes
		 FROM resource_revisions WHERE resource_uid = ? ORDER BY revision DESC LIMIT 1`,
		resourceUID,
	)
	return scanRevision(row)
}

// GetHistory returns paginated history for a resource.
func (s *Store) GetHistory(ctx context.Context, query storage.ResourceHistoryQuery) ([]storage.ResourceRevision, int64, error) {
	where, args := buildHistoryWhere(query)

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM resource_revisions WHERE %s", where)
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting revisions: %w", err)
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	selectQuery := fmt.Sprintf(
		`SELECT id, resource_uid, api_version, kind, namespace, name, revision, event_type,
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes
		 FROM resource_revisions WHERE %s ORDER BY revision DESC LIMIT ? OFFSET ?`,
		where,
	)
	args = append(args, limit, query.Offset)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying revisions: %w", err)
	}
	defer rows.Close()

	var revisions []storage.ResourceRevision
	for rows.Next() {
		rev, err := scanRevisionRows(rows)
		if err != nil {
			return nil, 0, err
		}
		revisions = append(revisions, *rev)
	}
	return revisions, total, rows.Err()
}

// GetNearestSnapshot finds the closest full snapshot at or before the given revision.
func (s *Store) GetNearestSnapshot(ctx context.Context, resourceUID string, revision int64) (*storage.ResourceRevision, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, resource_uid, api_version, kind, namespace, name, revision, event_type,
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes
		 FROM resource_revisions 
		 WHERE resource_uid = ? AND is_snapshot = TRUE AND revision <= ?
		 ORDER BY revision DESC LIMIT 1`,
		resourceUID, revision,
	)
	return scanRevision(row)
}

// GetPatchesBetween retrieves patches between two revisions.
func (s *Store) GetPatchesBetween(ctx context.Context, resourceUID string, fromRevision, toRevision int64) ([]storage.ResourceRevision, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, resource_uid, api_version, kind, namespace, name, revision, event_type,
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes
		 FROM resource_revisions
		 WHERE resource_uid = ? AND revision > ? AND revision <= ?
		 ORDER BY revision ASC`,
		resourceUID, fromRevision, toRevision,
	)
	if err != nil {
		return nil, fmt.Errorf("querying patches: %w", err)
	}
	defer rows.Close()

	var revisions []storage.ResourceRevision
	for rows.Next() {
		rev, err := scanRevisionRows(rows)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, *rev)
	}
	return revisions, rows.Err()
}

// UpsertTrackedResource creates or updates a tracked resource record.
func (s *Store) UpsertTrackedResource(ctx context.Context, resource *storage.TrackedResourceRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tracked_resources (uid, api_version, kind, namespace, name, current_revision, first_seen, last_seen, is_deleted, policy_name)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(uid) DO UPDATE SET
		   current_revision = excluded.current_revision,
		   last_seen = excluded.last_seen,
		   is_deleted = excluded.is_deleted`,
		resource.UID, resource.APIVersion, resource.Kind, resource.Namespace, resource.Name,
		resource.CurrentRevision, resource.FirstSeen.UTC(), resource.LastSeen.UTC(),
		resource.IsDeleted, resource.PolicyName,
	)
	if err != nil {
		return fmt.Errorf("upserting tracked resource: %w", err)
	}
	return nil
}

// GetTrackedResource retrieves a tracked resource by UID.
func (s *Store) GetTrackedResource(ctx context.Context, uid string) (*storage.TrackedResourceRecord, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT uid, api_version, kind, namespace, name, current_revision, first_seen, last_seen, is_deleted, policy_name
		 FROM tracked_resources WHERE uid = ?`, uid,
	)

	var r storage.TrackedResourceRecord
	err := row.Scan(&r.UID, &r.APIVersion, &r.Kind, &r.Namespace, &r.Name,
		&r.CurrentRevision, &r.FirstSeen, &r.LastSeen, &r.IsDeleted, &r.PolicyName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning tracked resource: %w", err)
	}
	return &r, nil
}

// ListTrackedResources lists tracked resources with filters.
func (s *Store) ListTrackedResources(ctx context.Context, query storage.ResourceListQuery) ([]storage.TrackedResourceRecord, int64, error) {
	where, args := buildResourceListWhere(query)

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tracked_resources WHERE %s", where)
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting resources: %w", err)
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	selectQuery := fmt.Sprintf(
		`SELECT uid, api_version, kind, namespace, name, current_revision, first_seen, last_seen, is_deleted, policy_name
		 FROM tracked_resources WHERE %s ORDER BY kind, namespace, name LIMIT ? OFFSET ?`,
		where,
	)
	args = append(args, limit, query.Offset)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying resources: %w", err)
	}
	defer rows.Close()

	var resources []storage.TrackedResourceRecord
	for rows.Next() {
		var r storage.TrackedResourceRecord
		if err := rows.Scan(&r.UID, &r.APIVersion, &r.Kind, &r.Namespace, &r.Name,
			&r.CurrentRevision, &r.FirstSeen, &r.LastSeen, &r.IsDeleted, &r.PolicyName); err != nil {
			return nil, 0, fmt.Errorf("scanning resource: %w", err)
		}
		resources = append(resources, r)
	}
	return resources, total, rows.Err()
}

// MarkDeleted marks a resource as deleted.
func (s *Store) MarkDeleted(ctx context.Context, uid string, deletedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tracked_resources SET is_deleted = TRUE, last_seen = ? WHERE uid = ?`,
		deletedAt.UTC(), uid,
	)
	return err
}

// PurgeOldRevisions removes revisions older than the given time.
func (s *Store) PurgeOldRevisions(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM resource_revisions WHERE timestamp < ?`, olderThan.UTC(),
	)
	if err != nil {
		return 0, fmt.Errorf("purging old revisions: %w", err)
	}
	return result.RowsAffected()
}

// PurgeExcessRevisions removes revisions beyond maxRevisions for a resource.
func (s *Store) PurgeExcessRevisions(ctx context.Context, resourceUID string, maxRevisions int64) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM resource_revisions 
		 WHERE resource_uid = ? AND revision NOT IN (
		   SELECT revision FROM resource_revisions 
		   WHERE resource_uid = ? ORDER BY revision DESC LIMIT ?
		 )`,
		resourceUID, resourceUID, maxRevisions,
	)
	if err != nil {
		return 0, fmt.Errorf("purging excess revisions: %w", err)
	}
	return result.RowsAffected()
}

// GetStats returns overall storage statistics.
func (s *Store) GetStats(ctx context.Context) (*storage.StorageStats, error) {
	stats := &storage.StorageStats{}

	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tracked_resources").Scan(&stats.TotalResources)
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM resource_revisions").Scan(&stats.TotalRevisions)
	if err != nil {
		return nil, err
	}

	// Get database file size via page_count * page_size
	err = s.db.QueryRowContext(ctx,
		"SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&stats.StorageBytes)
	if err != nil {
		// Fallback: estimate from data
		_ = s.db.QueryRowContext(ctx,
			"SELECT COALESCE(SUM(size_bytes), 0) FROM resource_revisions").Scan(&stats.StorageBytes)
	}

	var oldestStr, newestStr sql.NullString
	_ = s.db.QueryRowContext(ctx, "SELECT MIN(timestamp) FROM resource_revisions").Scan(&oldestStr)
	_ = s.db.QueryRowContext(ctx, "SELECT MAX(timestamp) FROM resource_revisions").Scan(&newestStr)
	if oldestStr.Valid && oldestStr.String != "" {
		if t := parseSQLiteTime(oldestStr.String); t != nil {
			stats.OldestRevision = t
		}
	}
	if newestStr.Valid && newestStr.String != "" {
		if t := parseSQLiteTime(newestStr.String); t != nil {
			stats.NewestRevision = t
		}
	}

	return stats, nil
}

// GetKindStats returns per-kind statistics.
func (s *Store) GetKindStats(ctx context.Context) ([]storage.KindStats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tr.api_version, tr.kind, COUNT(DISTINCT tr.uid) as resources,
		 COALESCE((SELECT COUNT(*) FROM resource_revisions rr WHERE rr.kind = tr.kind AND rr.api_version = tr.api_version), 0) as revisions
		 FROM tracked_resources tr
		 GROUP BY tr.api_version, tr.kind
		 ORDER BY tr.kind`,
	)
	if err != nil {
		return nil, fmt.Errorf("querying kind stats: %w", err)
	}
	defer rows.Close()

	var stats []storage.KindStats
	for rows.Next() {
		var s storage.KindStats
		if err := rows.Scan(&s.APIVersion, &s.Kind, &s.Resources, &s.Revisions); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// --- Helper functions ---

func buildHistoryWhere(q storage.ResourceHistoryQuery) (string, []interface{}) {
	conditions := []string{"1=1"}
	var args []interface{}

	if q.UID != "" {
		conditions = append(conditions, "resource_uid = ?")
		args = append(args, q.UID)
	}
	if q.APIVersion != "" {
		conditions = append(conditions, "api_version = ?")
		args = append(args, q.APIVersion)
	}
	if q.Kind != "" {
		conditions = append(conditions, "kind = ?")
		args = append(args, q.Kind)
	}
	if q.Namespace != "" {
		conditions = append(conditions, "namespace = ?")
		args = append(args, q.Namespace)
	}
	if q.Name != "" {
		conditions = append(conditions, "name = ?")
		args = append(args, q.Name)
	}
	if q.PolicyName != "" {
		conditions = append(conditions, "policy_name = ?")
		args = append(args, q.PolicyName)
	}
	if q.EventType != "" {
		conditions = append(conditions, "event_type = ?")
		args = append(args, q.EventType)
	}
	if q.Since != nil {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, q.Since.UTC())
	}
	if q.Until != nil {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, q.Until.UTC())
	}

	return strings.Join(conditions, " AND "), args
}

func buildResourceListWhere(q storage.ResourceListQuery) (string, []interface{}) {
	conditions := []string{"1=1"}
	var args []interface{}

	if q.APIVersion != "" {
		conditions = append(conditions, "api_version = ?")
		args = append(args, q.APIVersion)
	}
	if q.Kind != "" {
		conditions = append(conditions, "kind = ?")
		args = append(args, q.Kind)
	}
	if q.Namespace != "" {
		conditions = append(conditions, "namespace = ?")
		args = append(args, q.Namespace)
	}
	if q.PolicyName != "" {
		conditions = append(conditions, "policy_name = ?")
		args = append(args, q.PolicyName)
	}
	if q.IsDeleted != nil {
		conditions = append(conditions, "is_deleted = ?")
		args = append(args, *q.IsDeleted)
	}

	return strings.Join(conditions, " AND "), args
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRevision(row scannable) (*storage.ResourceRevision, error) {
	var rev storage.ResourceRevision
	var changedFields string
	var snapshot, patch []byte

	err := row.Scan(
		&rev.ID, &rev.ResourceUID, &rev.APIVersion, &rev.Kind, &rev.Namespace, &rev.Name,
		&rev.Revision, &rev.EventType, &snapshot, &patch, &rev.IsSnapshot,
		&rev.ResourceVersion, &changedFields, &rev.Timestamp, &rev.PolicyName, &rev.SizeBytes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning revision: %w", err)
	}

	rev.Snapshot = snapshot
	rev.Patch = patch
	if changedFields != "" {
		rev.ChangedFields = strings.Split(changedFields, ",")
	}
	return &rev, nil
}

var sqliteTimeFormats = []string{
	"2006-01-02 15:04:05.999999999 +0000 UTC",
	"2006-01-02 15:04:05.999999999+00:00",
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05Z",
}

func parseSQLiteTime(s string) *time.Time {
	for _, fmt := range sqliteTimeFormats {
		if t, err := time.Parse(fmt, s); err == nil {
			return &t
		}
	}
	return nil
}

func scanRevisionRows(rows *sql.Rows) (*storage.ResourceRevision, error) {
	var rev storage.ResourceRevision
	var changedFields string
	var snapshot, patch []byte

	err := rows.Scan(
		&rev.ID, &rev.ResourceUID, &rev.APIVersion, &rev.Kind, &rev.Namespace, &rev.Name,
		&rev.Revision, &rev.EventType, &snapshot, &patch, &rev.IsSnapshot,
		&rev.ResourceVersion, &changedFields, &rev.Timestamp, &rev.PolicyName, &rev.SizeBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning revision row: %w", err)
	}

	rev.Snapshot = snapshot
	rev.Patch = patch
	if changedFields != "" {
		rev.ChangedFields = strings.Split(changedFields, ",")
	}
	return &rev, nil
}
