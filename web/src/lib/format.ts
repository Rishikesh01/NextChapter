// Pure display helpers.
import type { SeriesStatus } from '@nextchapter/api-client';

export const STATUS_OPTIONS: { value: SeriesStatus; label: string }[] = [
  { value: 'reading', label: 'Reading' },
  { value: 'completed', label: 'Completed' },
  { value: 'on_hold', label: 'On hold' },
  { value: 'dropped', label: 'Dropped' },
  { value: 'plan_to_read', label: 'Plan to read' },
];

export function statusLabel(status: string | undefined): string {
  return (
    STATUS_OPTIONS.find((option) => option.value === status)?.label ?? 'Reading'
  );
}

/** "2 h ago" / "yesterday" / "3 wk ago" — compact, like the design mocks. */
export function relativeTime(
  iso: string | undefined,
  now = Date.now(),
): string {
  if (iso === undefined) return '';
  const then = Date.parse(iso);
  if (Number.isNaN(then)) return '';
  const seconds = Math.max(0, Math.floor((now - then) / 1000));
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${String(minutes)} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${String(hours)} h ago`;
  const days = Math.floor(hours / 24);
  if (days === 1) return 'yesterday';
  if (days < 7) return `${String(days)} d ago`;
  const weeks = Math.floor(days / 7);
  if (weeks < 5) return `${String(weeks)} wk ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${String(months)} mo ago`;
  return `${String(Math.floor(days / 365))} y ago`;
}
