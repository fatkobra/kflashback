import { useState, useMemo, useRef, useEffect } from 'react';
import { Plus, Pencil, Trash2, GitCommitHorizontal, Clock, Eye, Loader2 } from 'lucide-react';
import type { ResourceRevision } from '../types';
import { formatTimestamp, formatTimestampShort } from '../utils/format';

const eventMeta: Record<string, { Icon: typeof Plus; color: string; bg: string; line: string; label: string }> = {
  CREATED: { Icon: Plus, color: 'text-green-600', bg: 'bg-green-100 border-green-300', line: 'bg-green-300', label: 'Created' },
  UPDATED: { Icon: Pencil, color: 'text-blue-600', bg: 'bg-blue-100 border-blue-300', line: 'bg-blue-300', label: 'Updated' },
  DELETED: { Icon: Trash2, color: 'text-red-600', bg: 'bg-red-100 border-red-300', line: 'bg-red-300', label: 'Deleted' },
};

function timeBetween(a: string, b: string): string {
  const ms = Math.abs(new Date(a).getTime() - new Date(b).getTime());
  if (ms < 1000) return '<1s';
  if (ms < 60000) return `${Math.round(ms / 1000)}s`;
  if (ms < 3600000) return `${Math.round(ms / 60000)}m`;
  if (ms < 86400000) return `${(ms / 3600000).toFixed(1)}h`;
  return `${(ms / 86400000).toFixed(1)}d`;
}

interface Props {
  revisions: ResourceRevision[];
  selectedRevisions: number[];
  onToggleSelect: (rev: number) => void;
  onViewSnapshot: (rev: number) => void;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
}

export default function ChangeTimeline({ revisions, selectedRevisions, onToggleSelect, onViewSnapshot, hasMore, loadingMore, onLoadMore }: Props) {
  const [hoveredRev, setHoveredRev] = useState<number | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const sentinel = sentinelRef.current;
    if (!sentinel || !onLoadMore) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !loadingMore) {
          onLoadMore();
        }
      },
      { root: scrollRef.current, threshold: 0.1 }
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, loadingMore, onLoadMore]);

  // Sort oldest-first for the timeline
  const sorted = useMemo(() => [...revisions].sort((a, b) => a.revision - b.revision), [revisions]);

  if (sorted.length === 0) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 p-8 text-center text-gray-400 text-sm">
        <Clock className="w-8 h-8 mx-auto mb-2 opacity-40" />
        <p>No changes recorded yet</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
      <div className="px-5 py-3 border-b border-gray-100 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-gray-700 flex items-center gap-2">
          <Clock className="w-4 h-4" /> Change Timeline
        </h2>
        <span className="text-xs text-gray-400">{sorted.length} events</span>
      </div>

      <div ref={scrollRef} className="px-5 py-4 max-h-[700px] overflow-y-auto">
        <div className="relative">
          {sorted.map((rev, i) => {
            const meta = eventMeta[rev.eventType] ?? { Icon: GitCommitHorizontal, color: 'text-gray-600', bg: 'bg-gray-100 border-gray-300', line: 'bg-gray-300', label: rev.eventType };
            const { Icon } = meta;
            const isSelected = selectedRevisions.includes(rev.revision);
            const isHovered = hoveredRev === rev.revision;
            const isLast = i === sorted.length - 1;
            const gap = i > 0 ? timeBetween(sorted[i - 1].timestamp, rev.timestamp) : null;

            return (
              <div key={rev.revision}>
                {/* Time gap indicator */}
                {gap && (
                  <div className="flex items-center ml-[15px] -my-0.5">
                    <div className={`w-0.5 h-6 ${meta.line} opacity-40`} />
                    <span className="text-[10px] text-gray-400 ml-3 bg-gray-50 px-1.5 py-0.5 rounded">
                      +{gap}
                    </span>
                  </div>
                )}

                {/* Timeline node */}
                <div
                  className={`flex items-start gap-3 group relative rounded-lg px-2 py-2 -mx-2 transition-colors cursor-pointer ${
                    isSelected ? 'bg-blue-50 ring-1 ring-blue-200' : isHovered ? 'bg-gray-50' : ''
                  }`}
                  onMouseEnter={() => setHoveredRev(rev.revision)}
                  onMouseLeave={() => setHoveredRev(null)}
                >
                  {/* Vertical line + node dot */}
                  <div className="flex flex-col items-center flex-shrink-0">
                    <div className={`w-[30px] h-[30px] rounded-full border-2 flex items-center justify-center ${meta.bg} ${isSelected ? 'ring-2 ring-blue-400 ring-offset-1' : ''}`}>
                      <Icon className={`w-3.5 h-3.5 ${meta.color}`} />
                    </div>
                    {!isLast && (
                      <div className={`w-0.5 flex-1 min-h-[8px] ${meta.line} opacity-30`} />
                    )}
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0 pt-0.5 pb-1">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-semibold text-gray-900">Rev {rev.revision}</span>
                        <span className={`text-[10px] font-bold uppercase tracking-wider ${meta.color}`}>
                          {meta.label}
                        </span>
                        {rev.isSnapshot && (
                          <span className="text-[10px] bg-amber-100 text-amber-700 px-1.5 py-0.5 rounded font-medium">
                            snapshot
                          </span>
                        )}
                      </div>
                      <span className="text-xs text-gray-400 whitespace-nowrap">{formatTimestampShort(rev.timestamp)}</span>
                    </div>

                    {/* Changed fields */}
                    {rev.changedFields && rev.changedFields.length > 0 && (
                      <div className="mt-1.5 flex flex-wrap gap-1">
                        {rev.changedFields.slice(0, 5).map((f) => (
                          <code key={f} className="text-[11px] bg-gray-100 text-gray-600 px-1.5 py-0.5 rounded font-mono">
                            {f}
                          </code>
                        ))}
                        {rev.changedFields.length > 5 && (
                          <span className="text-[11px] text-gray-400">+{rev.changedFields.length - 5} more</span>
                        )}
                      </div>
                    )}

                    {/* Actions row */}
                    <div className="mt-1.5 flex items-center gap-3">
                      <label className="flex items-center gap-1.5 text-xs text-gray-500 hover:text-gray-700 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => onToggleSelect(rev.revision)}
                          className="rounded border-gray-300 text-blue-600 focus:ring-blue-500 h-3.5 w-3.5"
                        />
                        Select
                      </label>
                      <button
                        onClick={() => onViewSnapshot(rev.revision)}
                        className="text-xs text-blue-600 hover:text-blue-700 font-medium flex items-center gap-1"
                      >
                        <Eye className="w-3 h-3" /> View
                      </button>
                      <span className="text-[11px] text-gray-300">{formatTimestamp(rev.timestamp)}</span>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
          {/* Infinite scroll sentinel */}
          <div ref={sentinelRef} className="h-1" />
          {loadingMore && (
            <div className="flex items-center justify-center py-4 gap-2 text-sm text-gray-400">
              <Loader2 className="w-4 h-4 animate-spin" /> Loading more...
            </div>
          )}
          {!hasMore && sorted.length > 0 && (
            <div className="text-center py-3 text-xs text-gray-300">
              All events loaded
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
