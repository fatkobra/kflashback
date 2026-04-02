// Copyright 2024 The kflashback Authors
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"time"
)

// Store defines the interface for resource history storage backends.
type Store interface {
	// Initialize sets up the storage backend (creates tables, indexes, etc.).
	Initialize(ctx context.Context) error

	// Close cleanly shuts down the storage backend.
	Close() error

	// StoreRevision stores a new revision for a resource.
	StoreRevision(ctx context.Context, rev *ResourceRevision) error

	// GetRevision retrieves a specific revision by resource UID and revision number.
	GetRevision(ctx context.Context, resourceUID string, revision int64) (*ResourceRevision, error)

	// GetLatestRevision retrieves the latest revision for a resource.
	GetLatestRevision(ctx context.Context, resourceUID string) (*ResourceRevision, error)

	// GetHistory retrieves the revision history for a resource.
	GetHistory(ctx context.Context, query ResourceHistoryQuery) ([]ResourceRevision, int64, error)

	// GetNearestSnapshot finds the closest full snapshot at or before the given revision.
	GetNearestSnapshot(ctx context.Context, resourceUID string, revision int64) (*ResourceRevision, error)

	// GetPatchesBetween retrieves all patches between two revisions (exclusive start, inclusive end).
	GetPatchesBetween(ctx context.Context, resourceUID string, fromRevision, toRevision int64) ([]ResourceRevision, error)

	// UpsertTrackedResource creates or updates a tracked resource record.
	UpsertTrackedResource(ctx context.Context, resource *TrackedResourceRecord) error

	// GetTrackedResource retrieves a tracked resource by UID.
	GetTrackedResource(ctx context.Context, uid string) (*TrackedResourceRecord, error)

	// ListTrackedResources lists tracked resources with optional filters.
	ListTrackedResources(ctx context.Context, query ResourceListQuery) ([]TrackedResourceRecord, int64, error)

	// MarkDeleted marks a resource as deleted.
	MarkDeleted(ctx context.Context, uid string, deletedAt time.Time) error

	// PurgeOldRevisions removes revisions older than the given time.
	PurgeOldRevisions(ctx context.Context, olderThan time.Time) (int64, error)

	// PurgeExcessRevisions removes revisions beyond maxRevisions per resource.
	PurgeExcessRevisions(ctx context.Context, resourceUID string, maxRevisions int64) (int64, error)

	// GetStats returns overall storage statistics.
	GetStats(ctx context.Context) (*StorageStats, error)

	// GetKindStats returns per-kind statistics.
	GetKindStats(ctx context.Context) ([]KindStats, error)
}
