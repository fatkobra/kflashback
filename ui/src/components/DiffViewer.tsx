import { useState } from 'react';
import { ArrowLeftRight, List, Code2 } from 'lucide-react';
import type { DiffResult } from '../types';

type DiffViewMode = 'side-by-side' | 'unified' | 'patch';

export default function DiffViewer({ diff }: { diff: DiffResult }) {
  const [mode, setMode] = useState<DiffViewMode>('side-by-side');

  return (
    <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
      {/* Header */}
      <div className="px-5 py-3 border-b border-gray-100 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <ArrowLeftRight className="w-4 h-4 text-gray-400" />
          <h2 className="text-sm font-semibold text-gray-700">
            Comparing Rev {diff.fromRevision} → Rev {diff.toRevision}
          </h2>
        </div>
        <div className="flex items-center gap-1 bg-gray-100 rounded-lg p-0.5">
          <button
            onClick={() => setMode('side-by-side')}
            className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
              mode === 'side-by-side' ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            Side by Side
          </button>
          <button
            onClick={() => setMode('unified')}
            className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
              mode === 'unified' ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            <List className="w-3 h-3 inline mr-1" />
            Unified
          </button>
          <button
            onClick={() => setMode('patch')}
            className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
              mode === 'patch' ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            <Code2 className="w-3 h-3 inline mr-1" />
            Patch
          </button>
        </div>
      </div>

      {/* Changed paths summary */}
      {diff.changedPaths && diff.changedPaths.length > 0 && (
        <div className="px-5 py-2 border-b border-gray-100 bg-amber-50/50">
          <p className="text-xs font-medium text-amber-700 mb-1">
            {diff.changedPaths.length} path{diff.changedPaths.length > 1 ? 's' : ''} changed
          </p>
          <div className="flex flex-wrap gap-1">
            {diff.changedPaths.map((p) => (
              <code
                key={p}
                className="text-xs bg-amber-100 text-amber-800 px-1.5 py-0.5 rounded"
              >
                {p}
              </code>
            ))}
          </div>
        </div>
      )}

      {/* Content */}
      <div className="max-h-[650px] overflow-auto">
        {mode === 'side-by-side' && (
          <SideBySideView from={diff.fromSnapshot} to={diff.toSnapshot} />
        )}
        {mode === 'unified' && (
          <UnifiedView from={diff.fromSnapshot} to={diff.toSnapshot} />
        )}
        {mode === 'patch' && <PatchView patch={diff.patch} />}
      </div>
    </div>
  );
}

function SideBySideView({
  from,
  to,
}: {
  from: Record<string, unknown>;
  to: Record<string, unknown>;
}) {
  const fromLines = JSON.stringify(from, null, 2).split('\n');
  const toLines = JSON.stringify(to, null, 2).split('\n');
  const maxLines = Math.max(fromLines.length, toLines.length);

  return (
    <div className="grid grid-cols-2 divide-x divide-gray-200">
      <div>
        <div className="px-3 py-1.5 bg-red-50 border-b border-gray-200 text-xs font-medium text-red-700">
          Rev (from)
        </div>
        <pre className="text-xs leading-5 p-3 overflow-x-auto">
          {fromLines.map((line, i) => {
            const toLine = toLines[i];
            const isDiff = line !== toLine;
            return (
              <div
                key={i}
                className={`px-1 -mx-1 ${isDiff ? 'bg-red-50 text-red-800' : 'text-gray-600'}`}
              >
                <span className="text-gray-300 select-none w-8 inline-block text-right mr-3">
                  {i + 1}
                </span>
                {line}
              </div>
            );
          })}
        </pre>
      </div>
      <div>
        <div className="px-3 py-1.5 bg-green-50 border-b border-gray-200 text-xs font-medium text-green-700">
          Rev (to)
        </div>
        <pre className="text-xs leading-5 p-3 overflow-x-auto">
          {Array.from({ length: maxLines }, (_, i) => {
            const fromLine = fromLines[i];
            const toLine = toLines[i] ?? '';
            const isDiff = fromLine !== toLine;
            return (
              <div
                key={i}
                className={`px-1 -mx-1 ${isDiff ? 'bg-green-50 text-green-800' : 'text-gray-600'}`}
              >
                <span className="text-gray-300 select-none w-8 inline-block text-right mr-3">
                  {i + 1}
                </span>
                {toLine}
              </div>
            );
          })}
        </pre>
      </div>
    </div>
  );
}

function UnifiedView({
  from,
  to,
}: {
  from: Record<string, unknown>;
  to: Record<string, unknown>;
}) {
  const fromLines = JSON.stringify(from, null, 2).split('\n');
  const toLines = JSON.stringify(to, null, 2).split('\n');

  const unified: { type: 'same' | 'add' | 'remove'; line: string }[] = [];
  const max = Math.max(fromLines.length, toLines.length);

  for (let i = 0; i < max; i++) {
    const fl = fromLines[i];
    const tl = toLines[i];
    if (fl === tl) {
      unified.push({ type: 'same', line: fl ?? '' });
    } else {
      if (fl !== undefined) unified.push({ type: 'remove', line: fl });
      if (tl !== undefined) unified.push({ type: 'add', line: tl });
    }
  }

  return (
    <pre className="text-xs leading-5 p-3 overflow-x-auto">
      {unified.map((u, i) => {
        const bgColor =
          u.type === 'add'
            ? 'bg-green-50 text-green-800'
            : u.type === 'remove'
            ? 'bg-red-50 text-red-800'
            : 'text-gray-600';
        const prefix = u.type === 'add' ? '+' : u.type === 'remove' ? '-' : ' ';

        return (
          <div key={i} className={`px-1 -mx-1 ${bgColor}`}>
            <span className="text-gray-300 select-none w-5 inline-block">{prefix}</span>
            {u.line}
          </div>
        );
      })}
    </pre>
  );
}

function PatchView({ patch }: { patch: Record<string, unknown> }) {
  const formatted = JSON.stringify(patch, null, 2);
  return (
    <pre className="text-xs leading-5 p-4 bg-gray-900 text-green-400 overflow-x-auto font-mono">
      {formatted}
    </pre>
  );
}
