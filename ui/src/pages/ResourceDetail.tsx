import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  GitCommitHorizontal,
  Plus,
  Pencil,
  Trash2,
  Eye,
  ArrowLeftRight,
  ChevronDown,
  ChevronUp,
  Search,
  Filter,
  X,
  Loader2,
} from 'lucide-react';
import { getResource, getHistory, diffRevisions, reconstructAtRevision } from '../api/client';
import type { TrackedResource, ResourceRevision, DiffResult } from '../types';
import { timeAgo, formatTimestamp } from '../utils/format';
import DiffViewer from '../components/DiffViewer';
import JsonViewer from '../components/JsonViewer';
import ChangeTimeline from '../components/ChangeTimeline';

const PAGE_SIZE = 30;

type PanelView =
  | { kind: 'empty' }
  | { kind: 'snapshot'; revision: number; data: Record<string, unknown> }
  | { kind: 'diff'; from: number; to: number; data: DiffResult };

interface HistoryFilters {
  since: string;
  until: string;
  eventType: string;
}

const emptyFilters: HistoryFilters = { since: '', until: '', eventType: '' };

export default function ResourceDetail() {
  const { uid } = useParams<{ uid: string }>();
  const [resource, setResource] = useState<TrackedResource | null>(null);
  const [revisions, setRevisions] = useState<ResourceRevision[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [selectedRevisions, setSelectedRevisions] = useState<number[]>([]);
  const [panel, setPanel] = useState<PanelView>({ kind: 'empty' });
  const [detailLoading, setDetailLoading] = useState(false);
  const [leftTab, setLeftTab] = useState<'list' | 'timeline'>('list');

  // Pagination & filters
  const [filters, setFilters] = useState<HistoryFilters>(emptyFilters);
  const [appliedFilters, setAppliedFilters] = useState<HistoryFilters>(emptyFilters);
  const [showFilters, setShowFilters] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);

  // Build query params from applied filters
  const buildParams = useCallback((offset: number, f: HistoryFilters) => {
    const params: { limit: number; offset: number; since?: string; until?: string; eventType?: string } = {
      limit: PAGE_SIZE,
      offset,
    };
    if (f.since) params.since = new Date(f.since).toISOString();
    if (f.until) params.until = new Date(f.until).toISOString();
    if (f.eventType) params.eventType = f.eventType;
    return params;
  }, []);

  // Initial load + reload on filter change
  const loadInitial = useCallback(async (f: HistoryFilters) => {
    if (!uid) return;
    setLoading(true);
    setRevisions([]);
    try {
      const [res, hist] = await Promise.all([
        getResource(uid),
        getHistory(uid, buildParams(0, f)),
      ]);
      setResource(res);
      setRevisions(hist.data);
      setTotal(hist.total);
      setHasMore(hist.data.length < hist.total);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [uid, buildParams]);

  useEffect(() => {
    loadInitial(appliedFilters);
  }, [loadInitial, appliedFilters]);

  // Load more (infinite scroll)
  const loadMore = useCallback(async () => {
    if (!uid || loadingMore || !hasMore) return;
    setLoadingMore(true);
    try {
      const hist = await getHistory(uid, buildParams(revisions.length, appliedFilters));
      setRevisions((prev) => [...prev, ...hist.data]);
      setTotal(hist.total);
      setHasMore(revisions.length + hist.data.length < hist.total);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load more');
    } finally {
      setLoadingMore(false);
    }
  }, [uid, loadingMore, hasMore, revisions.length, buildParams, appliedFilters]);

  // IntersectionObserver for infinite scroll
  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !loadingMore) {
          loadMore();
        }
      },
      { root: scrollRef.current, threshold: 0.1 }
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, loadingMore, loadMore]);

  const applyFilters = () => {
    setAppliedFilters({ ...filters });
  };

  const clearFilters = () => {
    setFilters(emptyFilters);
    setAppliedFilters(emptyFilters);
  };

  const hasActiveFilters = appliedFilters.since !== '' || appliedFilters.until !== '' || appliedFilters.eventType !== '';

  const handleViewSnapshot = useCallback(
    async (revision: number) => {
      if (!uid) return;
      setDetailLoading(true);
      setSelectedRevisions([revision]);
      try {
        const data = await reconstructAtRevision(uid, revision);
        setPanel({ kind: 'snapshot', revision, data });
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to reconstruct');
      } finally {
        setDetailLoading(false);
      }
    },
    [uid]
  );

  const loadDiff = useCallback(
    async (from: number, to: number) => {
      if (!uid) return;
      setDetailLoading(true);
      try {
        const result = await diffRevisions(uid, from, to);
        setPanel({ kind: 'diff', from, to, data: result });
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to diff');
      } finally {
        setDetailLoading(false);
      }
    },
    [uid]
  );

  const handleToggleSelect = useCallback(
    (revision: number) => {
      setSelectedRevisions((prev) => {
        if (prev.includes(revision)) {
          return prev.filter((r) => r !== revision);
        }
        const next = [...prev, revision].sort((a, b) => a - b);
        if (next.length > 2) return [next[0], next[next.length - 1]];
        return next;
      });
    },
    []
  );

  // Auto-trigger diff when exactly 2 revisions are selected
  useEffect(() => {
    if (selectedRevisions.length === 2) {
      loadDiff(selectedRevisions[0], selectedRevisions[1]);
    }
  }, [selectedRevisions, loadDiff]);

  if (loading && revisions.length === 0) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  if (error && !resource) {
    return (
      <div className="space-y-4">
        <Link to="/resources" className="flex items-center gap-2 text-sm text-gray-500 hover:text-blue-600">
          <ArrowLeft className="w-4 h-4" /> Back to Resources
        </Link>
        <div className="bg-red-50 border border-red-200 text-red-700 rounded-xl p-6">
          <h3 className="font-semibold">Error</h3>
          <p className="text-sm mt-1">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Breadcrumb */}
      <Link to="/resources" className="flex items-center gap-1.5 text-xs text-gray-500 hover:text-blue-600">
        <ArrowLeft className="w-3.5 h-3.5" /> Back to Resources
      </Link>

      {/* Resource header */}
      {resource && (
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <div className="flex items-start justify-between">
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-lg font-bold text-gray-900">{resource.name}</h1>
                {resource.isDeleted ? (
                  <span className="badge badge-red">Deleted</span>
                ) : (
                  <span className="badge badge-green">Active</span>
                )}
              </div>
              <div className="flex items-center gap-3 mt-1.5 text-xs text-gray-500">
                <span className="badge badge-blue">{resource.kind}</span>
                {resource.namespace && (
                  <span>
                    Namespace: <span className="font-medium text-gray-700">{resource.namespace}</span>
                  </span>
                )}
                <span>
                  API: <span className="font-medium text-gray-700">{resource.apiVersion}</span>
                </span>
                <span>
                  Policy: <span className="font-medium text-gray-700">{resource.policyName}</span>
                </span>
              </div>
            </div>
            <div className="text-right text-xs text-gray-500">
              <p>
                First seen: <span className="font-medium">{timeAgo(resource.firstSeen)}</span>
              </p>
              <p>
                Last seen: <span className="font-medium">{timeAgo(resource.lastSeen)}</span>
              </p>
              <p className="text-base font-bold text-gray-900 mt-0.5">
                {resource.currentRevision} revisions
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Selection info bar */}
      {selectedRevisions.length > 0 && (
        <div className="flex items-center justify-between bg-blue-50 border border-blue-200 rounded-md px-3 py-1.5">
          <div className="flex items-center gap-1.5 text-xs text-blue-700">
            <ArrowLeftRight className="w-3.5 h-3.5" />
            {selectedRevisions.length === 1 ? (
              <span>Viewing Rev {selectedRevisions[0]} — check another revision to compare</span>
            ) : (
              <span>Comparing Rev {selectedRevisions[0]} vs Rev {selectedRevisions[1]}</span>
            )}
          </div>
          <button
            onClick={() => { setSelectedRevisions([]); setPanel({ kind: 'empty' }); }}
            className="text-xs text-blue-600 hover:text-blue-800 font-medium"
          >
            Clear selection
          </button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
        {/* Left panel */}
        <div className="lg:col-span-2 space-y-2">
          {/* Tab toggle + filter button */}
          <div className="flex items-center gap-2">
            <div className="flex gap-1 flex-1 bg-gray-100 rounded-lg p-1">
              <button
                onClick={() => setLeftTab('list')}
                className={`flex-1 px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                  leftTab === 'list'
                    ? 'bg-white text-gray-900 shadow-sm'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                List View
              </button>
              <button
                onClick={() => setLeftTab('timeline')}
                className={`flex-1 px-3 py-1.5 rounded-md text-xs font-medium transition-colors ${
                  leftTab === 'timeline'
                    ? 'bg-white text-gray-900 shadow-sm'
                    : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                Timeline
              </button>
            </div>
            <button
              onClick={() => setShowFilters(!showFilters)}
              className={`p-2 rounded-lg border transition-colors ${
                hasActiveFilters
                  ? 'bg-blue-50 border-blue-300 text-blue-600'
                  : 'bg-white border-gray-200 text-gray-500 hover:text-gray-700'
              }`}
              title="Filter revisions"
            >
              <Filter className="w-4 h-4" />
            </button>
          </div>

          {/* Filter panel */}
          {showFilters && (
            <div className="bg-white rounded-xl border border-gray-200 p-4 space-y-3">
              <div className="flex items-center justify-between">
                <h3 className="text-xs font-semibold text-gray-700 uppercase tracking-wider flex items-center gap-1.5">
                  <Search className="w-3.5 h-3.5" /> Search & Filter
                </h3>
                {hasActiveFilters && (
                  <button onClick={clearFilters} className="text-xs text-red-500 hover:text-red-700 flex items-center gap-1">
                    <X className="w-3 h-3" /> Clear
                  </button>
                )}
              </div>

              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-[11px] font-medium text-gray-500 mb-1">From</label>
                  <input
                    type="datetime-local"
                    value={filters.since}
                    onChange={(e) => setFilters((f) => ({ ...f, since: e.target.value }))}
                    className="w-full text-xs border border-gray-200 rounded-lg px-2.5 py-1.5 focus:ring-1 focus:ring-blue-500 focus:border-blue-500"
                  />
                </div>
                <div>
                  <label className="block text-[11px] font-medium text-gray-500 mb-1">To</label>
                  <input
                    type="datetime-local"
                    value={filters.until}
                    onChange={(e) => setFilters((f) => ({ ...f, until: e.target.value }))}
                    className="w-full text-xs border border-gray-200 rounded-lg px-2.5 py-1.5 focus:ring-1 focus:ring-blue-500 focus:border-blue-500"
                  />
                </div>
              </div>

              <div>
                <label className="block text-[11px] font-medium text-gray-500 mb-1">Event Type</label>
                <select
                  value={filters.eventType}
                  onChange={(e) => setFilters((f) => ({ ...f, eventType: e.target.value }))}
                  className="w-full text-xs border border-gray-200 rounded-lg px-2.5 py-1.5 focus:ring-1 focus:ring-blue-500 focus:border-blue-500"
                >
                  <option value="">All events</option>
                  <option value="CREATED">Created</option>
                  <option value="UPDATED">Updated</option>
                  <option value="DELETED">Deleted</option>
                </select>
              </div>

              <button
                onClick={applyFilters}
                className="w-full px-3 py-1.5 bg-blue-600 text-white text-xs font-medium rounded-lg hover:bg-blue-700 transition-colors"
              >
                Apply Filters
              </button>
            </div>
          )}

          {/* Active filter badges */}
          {hasActiveFilters && !showFilters && (
            <div className="flex flex-wrap items-center gap-1.5">
              {appliedFilters.since && (
                <span className="inline-flex items-center gap-1 text-[11px] bg-blue-50 text-blue-700 border border-blue-200 rounded-full px-2 py-0.5">
                  From: {new Date(appliedFilters.since).toLocaleDateString()}
                  <button onClick={() => { const f = { ...appliedFilters, since: '' }; setFilters(f); setAppliedFilters(f); }}>
                    <X className="w-3 h-3" />
                  </button>
                </span>
              )}
              {appliedFilters.until && (
                <span className="inline-flex items-center gap-1 text-[11px] bg-blue-50 text-blue-700 border border-blue-200 rounded-full px-2 py-0.5">
                  To: {new Date(appliedFilters.until).toLocaleDateString()}
                  <button onClick={() => { const f = { ...appliedFilters, until: '' }; setFilters(f); setAppliedFilters(f); }}>
                    <X className="w-3 h-3" />
                  </button>
                </span>
              )}
              {appliedFilters.eventType && (
                <span className="inline-flex items-center gap-1 text-[11px] bg-blue-50 text-blue-700 border border-blue-200 rounded-full px-2 py-0.5">
                  {appliedFilters.eventType}
                  <button onClick={() => { const f = { ...appliedFilters, eventType: '' }; setFilters(f); setAppliedFilters(f); }}>
                    <X className="w-3 h-3" />
                  </button>
                </span>
              )}
            </div>
          )}

          {leftTab === 'timeline' ? (
            <ChangeTimeline
              revisions={revisions}
              selectedRevisions={selectedRevisions}
              onToggleSelect={handleToggleSelect}
              onViewSnapshot={handleViewSnapshot}
              hasMore={hasMore}
              loadingMore={loadingMore}
              onLoadMore={loadMore}
            />
          ) : (
            <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
              <div className="px-4 py-2 border-b border-gray-100 flex items-center justify-between">
                <h2 className="text-sm font-semibold text-gray-700">
                  Revision History
                </h2>
                <span className="text-xs text-gray-400">
                  {revisions.length} of {total}
                </span>
              </div>
              <div ref={scrollRef} className="divide-y divide-gray-50 max-h-[700px] overflow-y-auto">
                {revisions.map((rev) => (
                  <RevisionItem
                    key={rev.revision}
                    revision={rev}
                    isSelected={selectedRevisions.includes(rev.revision)}
                    onToggleSelect={() => handleToggleSelect(rev.revision)}
                    onViewSnapshot={() => handleViewSnapshot(rev.revision)}
                  />
                ))}
                {/* Infinite scroll sentinel */}
                <div ref={sentinelRef} className="h-1" />
                {loadingMore && (
                  <div className="flex items-center justify-center py-4 gap-2 text-sm text-gray-400">
                    <Loader2 className="w-4 h-4 animate-spin" /> Loading more...
                  </div>
                )}
                {!hasMore && revisions.length > 0 && (
                  <div className="text-center py-3 text-xs text-gray-300">
                    All {total} revisions loaded
                  </div>
                )}
                {revisions.length === 0 && !loading && (
                  <div className="text-center py-12 text-gray-400 text-sm">
                    {hasActiveFilters ? 'No revisions match your filters' : 'No revisions found'}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>

        {/* Detail panel (right) */}
        <div className="lg:col-span-3">
          {detailLoading ? (
            <div className="bg-white rounded-lg border border-gray-200 flex items-center justify-center h-48">
              <div className="animate-spin rounded-full h-7 w-7 border-b-2 border-blue-600" />
            </div>
          ) : panel.kind === 'diff' ? (
            <DiffViewer diff={panel.data} />
          ) : panel.kind === 'snapshot' ? (
            <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
              <div className="px-4 py-2 border-b border-gray-100 flex items-center justify-between">
                <h2 className="text-sm font-semibold text-gray-700">
                  Snapshot at Revision {panel.revision}
                </h2>
              </div>
              <div className="p-3 max-h-[700px] overflow-auto">
                <JsonViewer data={panel.data} />
              </div>
            </div>
          ) : (
            <div className="bg-white rounded-lg border border-gray-200 flex items-center justify-center h-48 text-gray-400 text-sm">
              <div className="text-center">
                <Eye className="w-6 h-6 mx-auto mb-1.5 opacity-40" />
                <p>Click <b>View</b> on a revision to see its full state</p>
                <p className="text-xs mt-1">or check two revision boxes to compare them</p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

const eventIcons = {
  CREATED: Plus,
  UPDATED: Pencil,
  DELETED: Trash2,
};

const eventColors = {
  CREATED: 'text-green-600 bg-green-50 border-green-200',
  UPDATED: 'text-blue-600 bg-blue-50 border-blue-200',
  DELETED: 'text-red-600 bg-red-50 border-red-200',
};

function RevisionItem({
  revision,
  isSelected,
  onToggleSelect,
  onViewSnapshot,
}: {
  revision: ResourceRevision;
  isSelected: boolean;
  onToggleSelect: () => void;
  onViewSnapshot: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const Icon = eventIcons[revision.eventType] ?? GitCommitHorizontal;
  const colorClass = eventColors[revision.eventType] ?? 'text-gray-600 bg-gray-50 border-gray-200';

  return (
    <div className={`${isSelected ? 'bg-blue-50/60 ring-1 ring-blue-200' : ''} transition-colors`}>
      <div className="px-3 py-2 flex items-start gap-2">
        {/* Selection checkbox */}
        <label className="flex items-center mt-0.5">
          <input
            type="checkbox"
            checked={isSelected}
            onChange={onToggleSelect}
            className="rounded border-gray-300 text-blue-600 focus:ring-blue-500 h-3.5 w-3.5"
          />
        </label>

        {/* Event icon */}
        <div className={`p-1 rounded-md border ${colorClass}`}>
          <Icon className="w-3 h-3" />
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="text-sm font-semibold text-gray-900">Rev {revision.revision}</span>
              <span className={`badge ${
                revision.eventType === 'CREATED' ? 'badge-green' :
                revision.eventType === 'DELETED' ? 'badge-red' : 'badge-blue'
              }`}>
                {revision.eventType}
              </span>
              {revision.isSnapshot && (
                <span className="badge badge-yellow">snapshot</span>
              )}
            </div>
            <span className="text-xs text-gray-400">{timeAgo(revision.timestamp)}</span>
          </div>

          {/* Changed fields */}
          {revision.changedFields && revision.changedFields.length > 0 && (
            <div className="mt-1">
              <button
                onClick={() => setExpanded(!expanded)}
                className="text-xs text-gray-500 flex items-center gap-1 hover:text-gray-700"
              >
                {expanded ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                {revision.changedFields.length} field{revision.changedFields.length > 1 ? 's' : ''} changed
              </button>
              {expanded && (
                <div className="mt-1 flex flex-wrap gap-1">
                  {revision.changedFields.map((f) => (
                    <code key={f} className="text-xs bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded">
                      {f}
                    </code>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Actions */}
          <div className="mt-1 flex items-center gap-2">
            <button
              onClick={onViewSnapshot}
              className="text-xs text-blue-600 hover:text-blue-700 font-medium flex items-center gap-1"
            >
              <Eye className="w-3 h-3" /> View
            </button>
            <span className="text-xs text-gray-300">|</span>
            <span className="text-xs text-gray-400">
              {formatTimestamp(revision.timestamp)}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
