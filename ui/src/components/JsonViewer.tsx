import { useState } from 'react';
import { ChevronRight, ChevronDown, Copy, Check } from 'lucide-react';

interface JsonViewerProps {
  data: unknown;
  initialExpanded?: number;
}

export default function JsonViewer({ data, initialExpanded = 2 }: JsonViewerProps) {
  const [copied, setCopied] = useState(false);
  const jsonStr = JSON.stringify(data, null, 2);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(jsonStr);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="relative">
      <button
        onClick={handleCopy}
        className="absolute top-2 right-2 p-1.5 rounded-md bg-gray-100 hover:bg-gray-200 text-gray-500 transition-colors z-10"
        title="Copy JSON"
      >
        {copied ? <Check className="w-3.5 h-3.5 text-green-600" /> : <Copy className="w-3.5 h-3.5" />}
      </button>
      <div className="font-mono text-xs leading-5">
        <JsonNode value={data} depth={0} maxExpandDepth={initialExpanded} />
      </div>
    </div>
  );
}

function JsonNode({
  value,
  depth,
  maxExpandDepth,
  keyName,
}: {
  value: unknown;
  depth: number;
  maxExpandDepth: number;
  keyName?: string;
}) {
  const [expanded, setExpanded] = useState(depth < maxExpandDepth);
  const indent = depth * 16;

  if (value === null) {
    return (
      <div style={{ paddingLeft: indent }}>
        {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
        {keyName !== undefined && <span className="text-gray-500">: </span>}
        <span className="text-gray-400 italic">null</span>
      </div>
    );
  }

  if (typeof value === 'boolean') {
    return (
      <div style={{ paddingLeft: indent }}>
        {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
        {keyName !== undefined && <span className="text-gray-500">: </span>}
        <span className="text-orange-600">{String(value)}</span>
      </div>
    );
  }

  if (typeof value === 'number') {
    return (
      <div style={{ paddingLeft: indent }}>
        {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
        {keyName !== undefined && <span className="text-gray-500">: </span>}
        <span className="text-blue-600">{value}</span>
      </div>
    );
  }

  if (typeof value === 'string') {
    const isLong = value.length > 80;
    return (
      <div style={{ paddingLeft: indent }}>
        {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
        {keyName !== undefined && <span className="text-gray-500">: </span>}
        <span className="text-green-700" title={isLong ? value : undefined}>
          "{isLong ? value.slice(0, 77) + '...' : value}"
        </span>
      </div>
    );
  }

  if (Array.isArray(value)) {
    if (value.length === 0) {
      return (
        <div style={{ paddingLeft: indent }}>
          {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
          {keyName !== undefined && <span className="text-gray-500">: </span>}
          <span className="text-gray-500">[]</span>
        </div>
      );
    }

    return (
      <div>
        <div
          style={{ paddingLeft: indent }}
          className="cursor-pointer hover:bg-gray-50 rounded select-none"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? (
            <ChevronDown className="w-3 h-3 inline text-gray-400 mr-0.5" />
          ) : (
            <ChevronRight className="w-3 h-3 inline text-gray-400 mr-0.5" />
          )}
          {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
          {keyName !== undefined && <span className="text-gray-500">: </span>}
          <span className="text-gray-500">[</span>
          {!expanded && (
            <span className="text-gray-400"> {value.length} items ]</span>
          )}
        </div>
        {expanded && (
          <>
            {value.map((item, i) => (
              <JsonNode
                key={i}
                value={item}
                depth={depth + 1}
                maxExpandDepth={maxExpandDepth}
              />
            ))}
            <div style={{ paddingLeft: indent }}>
              <span className="text-gray-500">]</span>
            </div>
          </>
        )}
      </div>
    );
  }

  if (typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0) {
      return (
        <div style={{ paddingLeft: indent }}>
          {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
          {keyName !== undefined && <span className="text-gray-500">: </span>}
          <span className="text-gray-500">{'{}'}</span>
        </div>
      );
    }

    return (
      <div>
        <div
          style={{ paddingLeft: indent }}
          className="cursor-pointer hover:bg-gray-50 rounded select-none"
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? (
            <ChevronDown className="w-3 h-3 inline text-gray-400 mr-0.5" />
          ) : (
            <ChevronRight className="w-3 h-3 inline text-gray-400 mr-0.5" />
          )}
          {keyName !== undefined && <span className="text-purple-700">"{keyName}"</span>}
          {keyName !== undefined && <span className="text-gray-500">: </span>}
          <span className="text-gray-500">{'{'}</span>
          {!expanded && (
            <span className="text-gray-400"> {entries.length} keys {'}'}</span>
          )}
        </div>
        {expanded && (
          <>
            {entries.map(([k, v]) => (
              <JsonNode
                key={k}
                keyName={k}
                value={v}
                depth={depth + 1}
                maxExpandDepth={maxExpandDepth}
              />
            ))}
            <div style={{ paddingLeft: indent }}>
              <span className="text-gray-500">{'}'}</span>
            </div>
          </>
        )}
      </div>
    );
  }

  return (
    <div style={{ paddingLeft: indent }}>
      <span className="text-gray-500">{String(value)}</span>
    </div>
  );
}
