// Copyright 2024 The kflashback Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/kflashback/kflashback/internal/storage"
)

func init() {
	storage.RegisterBackend("postgres", func(dsn string) (storage.Store, error) {
		return New(dsn)
	})
}

// Store implements storage.Store using PostgreSQL.
type Store struct {
	db  *sql.DB
	dsn string
}

// New creates a new PostgreSQL store with the given DSN.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging postgres database: %w", err)
	}

	return &Store{db: db, dsn: dsn}, nil
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
			current_revision BIGINT DEFAULT 0,
			first_seen TIMESTAMPTZ NOT NULL,
			last_seen TIMESTAMPTZ NOT NULL,
			is_deleted BOOLEAN DEFAULT FALSE,
			policy_name TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS resource_revisions (
			id BIGSERIAL PRIMARY KEY,
			resource_uid TEXT NOT NULL,
			api_version TEXT NOT NULL,
			kind TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			revision BIGINT NOT NULL,
			event_type TEXT NOT NULL,
			snapshot BYTEA,
			patch BYTEA,
			is_snapshot BOOLEAN DEFAULT FALSE,
			resource_version TEXT NOT NULL DEFAULT '',
			changed_fields TEXT DEFAULT '',
			timestamp TIMESTAMPTZ NOT NULL,
			policy_name TEXT NOT NULL,
			size_bytes BIGINT DEFAULT 0,
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

// Close closes the database connection pool.
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
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
		 FROM resource_revisions WHERE resource_uid = $1 AND revision = $2`,
		resourceUID, revision,
	)
	return scanRevision(row)
}

// GetLatestRevision retrieves the latest revision for a resource.
func (s *Store) GetLatestRevision(ctx context.Context, resourceUID string) (*storage.ResourceRevision, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, resource_uid, api_version, kind, namespace, name, revision, event_type,
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes
		 FROM resource_revisions WHERE resource_uid = $1 ORDER BY revision DESC LIMIT 1`,
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

	nextParam := len(args) + 1
	selectQuery := fmt.Sprintf(
		`SELECT id, resource_uid, api_version, kind, namespace, name, revision, event_type,
		 snapshot, patch, is_snapshot, resource_version, changed_fields, timestamp, policy_name, size_bytes
		 FROM resource_revisions WHERE %s ORDER BY revision DESC LIMIT $%d OFFSET $%d`,
		where, nextParam, nextParam+1,
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
		 WHERE resource_uid = $1 AND is_snapshot = TRUE AND revision <= $2
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
		 WHERE resource_uid = $1 AND revision > $2 AND revision <= $3
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
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT(uid) DO UPDATE SET
		   current_revision = EXCLUDED.current_revision,
		   last_seen = EXCLUDED.last_seen,
		   is_deleted = EXCLUDED.is_deleted`,
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
		 FROM tracked_resources WHERE uid = $1`, uid,
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

	nextParam := len(args) + 1
	selectQuery := fmt.Sprintf(
		`SELECT uid, api_version, kind, namespace, name, current_revision, first_seen, last_seen, is_deleted, policy_name
		 FROM tracked_resources WHERE %s ORDER BY kind, namespace, name LIMIT $%d OFFSET $%d`,
		where, nextParam, nextParam+1,
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
		`UPDATE tracked_resources SET is_deleted = TRUE, last_seen = $1 WHERE uid = $2`,
		deletedAt.UTC(), uid,
	)
	return err
}

// PurgeOldRevisions removes revisions older than the given time.
func (s *Store) PurgeOldRevisions(ctx context.Context, olderThan time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM resource_revisions WHERE timestamp < $1`, olderThan.UTC(),
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
		 WHERE resource_uid = $1 AND revision NOT IN (
		   SELECT revision FROM resource_revisions 
		   WHERE resource_uid = $2 ORDER BY revision DESC LIMIT $3
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

	// Estimate storage from data size (pg_total_relation_size requires superuser in some setups)
	err = s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(size_bytes), 0) FROM resource_revisions").Scan(&stats.StorageBytes)
	if err != nil {
		stats.StorageBytes = 0
	}

	// Try to get actual table sizes if we have permission
	var tableSize int64
	err = s.db.QueryRowContext(ctx,
		`SELECT pg_total_relation_size('resource_revisions') + pg_total_relation_size('tracked_resources')`).Scan(&tableSize)
	if err == nil && tableSize > 0 {
		stats.StorageBytes = tableSize
	}

	var oldest, newest sql.NullTime
	s.db.QueryRowContext(ctx, "SELECT MIN(timestamp) FROM resource_revisions").Scan(&oldest)
	s.db.QueryRowContext(ctx, "SELECT MAX(timestamp) FROM resource_revisions").Scan(&newest)
	if oldest.Valid {
		stats.OldestRevision = &oldest.Time
	}
	if newest.Valid {
		stats.NewestRevision = &newest.Time
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
		var ks storage.KindStats
		if err := rows.Scan(&ks.APIVersion, &ks.Kind, &ks.Resources, &ks.Revisions); err != nil {
			return nil, err
		}
		stats = append(stats, ks)
	}
	return stats, rows.Err()
}

// --- Helper functions ---

func buildHistoryWhere(q storage.ResourceHistoryQuery) (string, []interface{}) {
	conditions := []string{"1=1"}
	var args []interface{}
	paramIdx := 1

	if q.UID != "" {
		conditions = append(conditions, fmt.Sprintf("resource_uid = $%d", paramIdx))
		args = append(args, q.UID)
		paramIdx++
	}
	if q.APIVersion != "" {
		conditions = append(conditions, fmt.Sprintf("api_version = $%d", paramIdx))
		args = append(args, q.APIVersion)
		paramIdx++
	}
	if q.Kind != "" {
		conditions = append(conditions, fmt.Sprintf("kind = $%d", paramIdx))
		args = append(args, q.Kind)
		paramIdx++
	}
	if q.Namespace != "" {
		conditions = append(conditions, fmt.Sprintf("namespace = $%d", paramIdx))
		args = append(args, q.Namespace)
		paramIdx++
	}
	if q.Name != "" {
		conditions = append(conditions, fmt.Sprintf("name = $%d", paramIdx))
		args = append(args, q.Name)
		paramIdx++
	}
	if q.PolicyName != "" {
		conditions = append(conditions, fmt.Sprintf("policy_name = $%d", paramIdx))
		args = append(args, q.PolicyName)
		paramIdx++
	}
	if q.EventType != "" {
		conditions = append(conditions, fmt.Sprintf("event_type = $%d", paramIdx))
		args = append(args, q.EventType)
		paramIdx++
	}
	if q.Since != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", paramIdx))
		args = append(args, q.Since.UTC())
		paramIdx++
	}
	if q.Until != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", paramIdx))
		args = append(args, q.Until.UTC())
	}

	return strings.Join(conditions, " AND "), args
}

func buildResourceListWhere(q storage.ResourceListQuery) (string, []interface{}) {
	conditions := []string{"1=1"}
	var args []interface{}
	paramIdx := 1

	if q.APIVersion != "" {
		conditions = append(conditions, fmt.Sprintf("api_version = $%d", paramIdx))
		args = append(args, q.APIVersion)
		paramIdx++
	}
	if q.Kind != "" {
		conditions = append(conditions, fmt.Sprintf("kind = $%d", paramIdx))
		args = append(args, q.Kind)
		paramIdx++
	}
	if q.Namespace != "" {
		conditions = append(conditions, fmt.Sprintf("namespace = $%d", paramIdx))
		args = append(args, q.Namespace)
		paramIdx++
	}
	if q.PolicyName != "" {
		conditions = append(conditions, fmt.Sprintf("policy_name = $%d", paramIdx))
		args = append(args, q.PolicyName)
		paramIdx++
	}
	if q.IsDeleted != nil {
		conditions = append(conditions, fmt.Sprintf("is_deleted = $%d", paramIdx))
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
