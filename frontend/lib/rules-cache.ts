// Pure staleness logic for the cached site rules — no storage, no clock.
import type { SiteList, SiteRule } from '@nextchapter/api-client';

export interface RulesCache {
  rules: SiteRule[];
  trackedHosts: string[];
  /** Epoch milliseconds of the fetch that produced this cache. */
  fetchedAt: number;
}

/** Stale-while-revalidate window (ADR-0008 §8). */
export const RULES_TTL_MS = 15 * 60 * 1000;

export function toRulesCache(list: SiteList, now: number): RulesCache {
  return {
    rules: list.rules ?? [],
    trackedHosts: list.tracked_hosts ?? [],
    fetchedAt: now,
  };
}

/** False for a missing cache, an expired one, or one stamped in the future (clock skew). */
export function isFresh(cache: RulesCache | undefined, now: number): boolean {
  if (cache === undefined) return false;
  const age = now - cache.fetchedAt;
  return age >= 0 && age < RULES_TTL_MS;
}
