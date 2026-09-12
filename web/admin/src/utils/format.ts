export const numberFormatter = new Intl.NumberFormat('en-US');

export function validDate(value?: string | null): Date | null {
  if (!value || value.startsWith('0001-')) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatDate(value?: string | null): string {
  return validDate(value)?.toLocaleString() ?? '—';
}

export function formatAge(value?: string | null): string {
  const date = validDate(value);
  if (!date) return '—';
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ${minutes % 60}m ago`;
  return `${Math.floor(hours / 24)}d ${hours % 24}h ago`;
}

export function shortenMiddle(value: string, maxLength = 32): string {
  if (value.length <= maxLength) return value;
  const visibleLength = maxLength - 1;
  const startLength = Math.ceil(visibleLength / 2);
  const endLength = Math.floor(visibleLength / 2);
  return `${value.slice(0, startLength)}…${value.slice(-endLength)}`;
}

export function averageRate(messagesProcessed: number, connectedAt?: string | null): number {
  const date = validDate(connectedAt);
  if (!date) return 0;
  const seconds = Math.max(1, (Date.now() - date.getTime()) / 1000);
  return messagesProcessed / seconds;
}

export function compactNumber(value: number): string {
  if (value < 1000) return numberFormatter.format(value);
  if (value < 1_000_000) return `${(value / 1000).toFixed(value < 10_000 ? 1 : 0)}k`;
  return `${(value / 1_000_000).toFixed(value < 10_000_000 ? 1 : 0)}M`;
}
