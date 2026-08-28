import { describe, expect, it } from 'vitest';
import { relativeTime, statusLabel } from '../../src/lib/format';

describe('statusLabel', () => {
  it.each([
    ['reading', 'Reading'],
    ['on_hold', 'On hold'],
    ['plan_to_read', 'Plan to read'],
    [undefined, 'Reading'],
  ])('%s -> %s', (input, expected) => {
    expect(statusLabel(input)).toBe(expected);
  });
});

describe('relativeTime', () => {
  const now = Date.parse('2026-08-28T12:00:00Z');

  it.each([
    ['2026-08-28T11:59:40Z', 'just now'],
    ['2026-08-28T11:45:00Z', '15 min ago'],
    ['2026-08-28T10:00:00Z', '2 h ago'],
    ['2026-08-27T09:00:00Z', 'yesterday'],
    ['2026-08-23T12:00:00Z', '5 d ago'],
    ['2026-08-07T12:00:00Z', '3 wk ago'],
    ['2026-04-28T12:00:00Z', '4 mo ago'],
    ['2024-05-28T12:00:00Z', '2 y ago'],
  ])('%s -> %s', (iso, expected) => {
    expect(relativeTime(iso, now)).toBe(expected);
  });

  it('is empty for missing or invalid input', () => {
    expect(relativeTime(undefined, now)).toBe('');
    expect(relativeTime('not a date', now)).toBe('');
  });
});
