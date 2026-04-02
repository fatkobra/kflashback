import { formatDistanceToNow, format } from 'date-fns';

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

export function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

export function timeAgo(dateStr: string): string {
  try {
    return formatDistanceToNow(new Date(dateStr), { addSuffix: true });
  } catch {
    return dateStr;
  }
}

export function formatTimestamp(dateStr: string): string {
  try {
    return format(new Date(dateStr), 'MMM d, yyyy HH:mm:ss');
  } catch {
    return dateStr;
  }
}

export function formatTimestampShort(dateStr: string): string {
  try {
    return format(new Date(dateStr), 'MMM d, HH:mm');
  } catch {
    return dateStr;
  }
}
