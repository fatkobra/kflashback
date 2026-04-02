import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { Search, Filter, ChevronLeft, ChevronRight, ExternalLink } from 'lucide-react';
import { listResources, getKindStats } from '../api/client';
import type { TrackedResource, KindStats } from '../types';
import { timeAgo } from '../utils/format';

export default function ResourcesPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [resources, setResources] = useState<TrackedResource[]>([]);
  const [kindStats, setKindStats] = useState<KindStats[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');

  const kind = searchParams.get('kind') ?? '';
  const namespace = searchParams.get('namespace') ?? '';
  const page = parseInt(searchParams.get('page') ?? '1', 10);
  const limit = 25;

  useEffect(() => {
    getKindStats().then(setKindStats).catch(() => {});
  }, []);

  useEffect(() => {
    setLoading(true);
    setError(null);
    listResources({
      kind: kind || undefined,
      namespace: namespace || undefined,
      limit,
      offset: (page - 1) * limit,
    })
      .then(({ data, total }) => {
        setResources(data);
        setTotal(total);
      })
      .catch((e) => setError(e instanceof Error ? e.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, [kind, namespace, page]);

  const totalPages = Math.ceil(total / limit);

  const filteredResources = searchQuery
    ? resources.filter(
        (r) =>
          r.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          r.namespace.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : resources;

  function setFilter(key: string, value: string) {
    const params = new URLSearchParams(searchParams);
    if (value) {
      params.set(key, value);
    } else {
      params.delete(key);
    }
    params.delete('page');
    setSearchParams(params);
  }

  function goToPage(p: number) {
    const params = new URLSearchParams(searchParams);
    params.set('page', String(p));
    setSearchParams(params);
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-gray-900">Resources</h1>
        <p className="text-xs text-gray-500 mt-0.5">
          Browse all tracked Kubernetes resources and view their history
        </p>
      </div>

      {/* Filters row */}
      <div className="flex flex-wrap items-center gap-3">
        {/* Search */}
        <div className="relative flex-1 min-w-[240px] max-w-md">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search by name or namespace..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-9 pr-3 py-1.5 text-sm border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
          />
        </div>

        {/* Kind filter */}
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-gray-400" />
          <select
            value={kind}
            onChange={(e) => setFilter('kind', e.target.value)}
            className="text-sm border border-gray-300 rounded-md px-2.5 py-1.5 bg-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
          >
            <option value="">All Kinds</option>
            {kindStats.map((ks) => (
              <option key={ks.kind} value={ks.kind}>
                {ks.kind} ({ks.resources})
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="bg-red-50 border border-red-200 text-red-700 rounded-xl p-4 text-sm">
          {error}
        </div>
      )}

      {/* Table */}
      <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center h-48">
            <div className="animate-spin rounded-full h-7 w-7 border-b-2 border-blue-600" />
          </div>
        ) : filteredResources.length === 0 ? (
          <div className="text-center py-16 text-gray-400 text-sm">
            No resources found matching your filters
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="bg-gray-50 border-b border-gray-100">
                <th className="text-left text-xs font-semibold text-gray-500 uppercase tracking-wider px-4 py-2">
                  Resource
                </th>
                <th className="text-left text-xs font-semibold text-gray-500 uppercase tracking-wider px-4 py-2">
                  Kind
                </th>
                <th className="text-left text-xs font-semibold text-gray-500 uppercase tracking-wider px-4 py-2">
                  Namespace
                </th>
                <th className="text-right text-xs font-semibold text-gray-500 uppercase tracking-wider px-4 py-2">
                  Revisions
                </th>
                <th className="text-left text-xs font-semibold text-gray-500 uppercase tracking-wider px-4 py-2">
                  Last Seen
                </th>
                <th className="text-left text-xs font-semibold text-gray-500 uppercase tracking-wider px-4 py-2">
                  Status
                </th>
                <th className="px-4 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {filteredResources.map((r) => (
                <tr key={r.uid} className="hover:bg-blue-50/40 transition-colors">
                  <td className="px-4 py-2">
                    <Link
                      to={`/resources/${r.uid}`}
                      className="text-sm font-medium text-gray-900 hover:text-blue-600"
                    >
                      {r.name}
                    </Link>
                  </td>
                  <td className="px-4 py-2">
                    <KindPill kind={r.kind} />
                  </td>
                  <td className="px-4 py-2 text-sm text-gray-600">
                    {r.namespace || <span className="text-gray-300">cluster</span>}
                  </td>
                  <td className="px-4 py-2 text-sm text-gray-700 text-right font-mono">
                    {r.currentRevision}
                  </td>
                  <td className="px-4 py-2 text-sm text-gray-500">{timeAgo(r.lastSeen)}</td>
                  <td className="px-4 py-2">
                    {r.isDeleted ? (
                      <span className="badge badge-red">Deleted</span>
                    ) : (
                      <span className="badge badge-green">Active</span>
                    )}
                  </td>
                  <td className="px-4 py-2 text-right">
                    <Link
                      to={`/resources/${r.uid}`}
                      className="text-gray-400 hover:text-blue-600"
                    >
                      <ExternalLink className="w-4 h-4" />
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-gray-500">
            Showing {(page - 1) * limit + 1}-{Math.min(page * limit, total)} of {total}
          </p>
          <div className="flex items-center gap-2">
            <button
              onClick={() => goToPage(page - 1)}
              disabled={page <= 1}
              className="p-2 rounded-lg border border-gray-300 text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            {Array.from({ length: Math.min(totalPages, 5) }, (_, i) => {
              const p = i + 1;
              return (
                <button
                  key={p}
                  onClick={() => goToPage(p)}
                  className={`px-3 py-1.5 rounded-lg text-sm font-medium ${
                    p === page
                      ? 'bg-blue-600 text-white'
                      : 'text-gray-600 hover:bg-gray-100'
                  }`}
                >
                  {p}
                </button>
              );
            })}
            <button
              onClick={() => goToPage(page + 1)}
              disabled={page >= totalPages}
              className="p-2 rounded-lg border border-gray-300 text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

const kindColors: Record<string, string> = {
  Deployment: 'bg-blue-100 text-blue-700',
  StatefulSet: 'bg-purple-100 text-purple-700',
  DaemonSet: 'bg-indigo-100 text-indigo-700',
  Service: 'bg-emerald-100 text-emerald-700',
  Pod: 'bg-orange-100 text-orange-700',
  Job: 'bg-amber-100 text-amber-700',
  CronJob: 'bg-yellow-100 text-yellow-700',
};

function KindPill({ kind }: { kind: string }) {
  const color = kindColors[kind] ?? 'bg-gray-100 text-gray-700';
  return <span className={`badge ${color}`}>{kind}</span>;
}
