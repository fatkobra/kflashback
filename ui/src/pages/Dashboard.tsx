import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Database, Box, GitCommitHorizontal, HardDrive, ArrowRight } from 'lucide-react';
import { getStats, getKindStats } from '../api/client';
import type { StorageStats, KindStats } from '../types';
import { formatBytes, formatNumber, timeAgo } from '../utils/format';

export default function Dashboard() {
  const [stats, setStats] = useState<StorageStats | null>(null);
  const [kindStats, setKindStats] = useState<KindStats[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const [s, ks] = await Promise.all([getStats(), getKindStats()]);
        setStats(s);
        setKindStats(ks);
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load stats');
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 text-red-700 rounded-xl p-6">
        <h3 className="font-semibold">Error loading dashboard</h3>
        <p className="text-sm mt-1">{error}</p>
      </div>
    );
  }

  const statCards = [
    {
      label: 'Tracked Resources',
      value: formatNumber(stats?.totalResources ?? 0),
      icon: Box,
      color: 'bg-blue-500',
    },
    {
      label: 'Total Revisions',
      value: formatNumber(stats?.totalRevisions ?? 0),
      icon: GitCommitHorizontal,
      color: 'bg-emerald-500',
    },
    {
      label: 'Storage Used',
      value: formatBytes(stats?.storageBytes ?? 0),
      icon: HardDrive,
      color: 'bg-violet-500',
    },
    {
      label: 'Resource Types',
      value: String(kindStats.length),
      icon: Database,
      color: 'bg-amber-500',
    },
  ];

  return (
    <div className="space-y-5">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-xs text-gray-500 mt-0.5">
          Overview of tracked Kubernetes resource history
        </p>
      </div>

      {/* Stats grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        {statCards.map((card) => {
          const Icon = card.icon;
          return (
            <div
              key={card.label}
              className="bg-white rounded-lg border border-gray-200 p-4 hover:shadow-md transition-shadow"
            >
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-gray-500">{card.label}</p>
                  <p className="text-xl font-bold text-gray-900 mt-0.5">{card.value}</p>
                </div>
                <div className={`${card.color} p-2 rounded-md`}>
                  <Icon className="w-4 h-4 text-white" />
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Time range info */}
      {stats?.oldestRevision && (
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <h2 className="text-sm font-semibold text-gray-700 mb-2">History Range</h2>
          <div className="flex items-center gap-5 text-sm text-gray-600">
            <div>
              <span className="text-gray-400">Oldest: </span>
              <span className="font-medium">{timeAgo(stats.oldestRevision)}</span>
            </div>
            <div>
              <span className="text-gray-400">Newest: </span>
              <span className="font-medium">
                {stats.newestRevision ? timeAgo(stats.newestRevision) : 'N/A'}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* Resource types breakdown */}
      {kindStats.length > 0 && (
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <div className="px-4 py-2.5 border-b border-gray-100 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-gray-700">Tracked Resource Types</h2>
            <Link
              to="/resources"
              className="text-sm text-blue-600 hover:text-blue-700 flex items-center gap-1"
            >
              View all <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </div>
          <div className="divide-y divide-gray-50">
            {kindStats.map((ks) => (
              <div
                key={`${ks.apiVersion}/${ks.kind}`}
                className="px-4 py-2 flex items-center justify-between hover:bg-gray-50 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <KindBadge kind={ks.kind} />
                  <div>
                    <p className="text-sm font-medium text-gray-900">{ks.kind}</p>
                    <p className="text-xs text-gray-400">{ks.apiVersion}</p>
                  </div>
                </div>
                <div className="flex items-center gap-6 text-sm">
                  <div className="text-right">
                    <p className="font-semibold text-gray-700">{formatNumber(ks.resources)}</p>
                    <p className="text-xs text-gray-400">resources</p>
                  </div>
                  <div className="text-right">
                    <p className="font-semibold text-gray-700">{formatNumber(ks.revisions)}</p>
                    <p className="text-xs text-gray-400">revisions</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

const kindColorMap: Record<string, string> = {
  Deployment: 'bg-blue-100 text-blue-700',
  StatefulSet: 'bg-purple-100 text-purple-700',
  DaemonSet: 'bg-indigo-100 text-indigo-700',
  Service: 'bg-emerald-100 text-emerald-700',
  Pod: 'bg-orange-100 text-orange-700',
  Job: 'bg-amber-100 text-amber-700',
  CronJob: 'bg-yellow-100 text-yellow-700',
  ConfigMap: 'bg-teal-100 text-teal-700',
  Secret: 'bg-red-100 text-red-700',
  Ingress: 'bg-cyan-100 text-cyan-700',
};

function KindBadge({ kind }: { kind: string }) {
  const color = kindColorMap[kind] ?? 'bg-gray-100 text-gray-700';
  return (
    <span className={`${color} w-7 h-7 rounded-md flex items-center justify-center text-xs font-bold`}>
      {kind.slice(0, 2).toUpperCase()}
    </span>
  );
}
