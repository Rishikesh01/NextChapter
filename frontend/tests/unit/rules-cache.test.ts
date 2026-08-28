import { describe, expect, it } from 'vitest';
import { isFresh, RULES_TTL_MS, toRulesCache } from '../../lib/rules-cache';

describe('toRulesCache', () => {
  it('defaults missing fields to empty arrays', () => {
    expect(toRulesCache({}, 1000)).toEqual({
      rules: [],
      trackedHosts: [],
      fetchedAt: 1000,
    });
  });

  it('carries rules and tracked hosts through', () => {
    const cache = toRulesCache(
      { rules: [{ host: 'comics.example.org' }], tracked_hosts: ['comics.example.org'] },
      42,
    );
    expect(cache.rules).toHaveLength(1);
    expect(cache.trackedHosts).toEqual(['comics.example.org']);
  });
});

describe('isFresh', () => {
  const cache = toRulesCache({}, 100_000);

  it('is false for a missing cache', () => {
    expect(isFresh(undefined, 0)).toBe(false);
  });

  it('is true inside the TTL', () => {
    expect(isFresh(cache, 100_000 + RULES_TTL_MS - 1)).toBe(true);
  });

  it('is false at and past the TTL', () => {
    expect(isFresh(cache, 100_000 + RULES_TTL_MS)).toBe(false);
  });

  it('is false for a future-stamped cache (clock skew)', () => {
    expect(isFresh(cache, 99_999)).toBe(false);
  });
});
