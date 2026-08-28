// Thin typed glue over browser.storage.local. Versioned keys; pure logic
// (TTL, defaults) lives in rules-cache.ts so it stays unit-testable.
import { browser } from 'wxt/browser';
import type { RulesCache } from './rules-cache';

export interface Settings {
  /** Server origin, e.g. "https://nextchapter.example.org". */
  serverUrl: string;
  /** API token (nca_…). storage.local only — never storage.sync (ADR-0008 §6). */
  apiToken: string;
  /**
   * Id of the minted token, so Disconnect can revoke it (ADR-0009 §5).
   * Absent for pasted tokens — those can't be revoked from here.
   */
  apiTokenId?: number;
  /** For display on the options page. */
  username: string;
}

const SETTINGS_KEY = 'settings:v1';
const RULES_KEY = 'siteRules:v1';

async function read<T>(key: string): Promise<T | undefined> {
  const record = await browser.storage.local.get(key);
  return record[key] as T | undefined;
}

export function getSettings(): Promise<Settings | undefined> {
  return read<Settings>(SETTINGS_KEY);
}

export async function setSettings(settings: Settings): Promise<void> {
  await browser.storage.local.set({ [SETTINGS_KEY]: settings });
}

export async function clearSettings(): Promise<void> {
  await browser.storage.local.remove(SETTINGS_KEY);
}

export function getRulesCache(): Promise<RulesCache | undefined> {
  return read<RulesCache>(RULES_KEY);
}

export async function setRulesCache(cache: RulesCache): Promise<void> {
  await browser.storage.local.set({ [RULES_KEY]: cache });
}

/** Rules are per-account — Disconnect must not leak them into the next login. */
export async function clearRulesCache(): Promise<void> {
  await browser.storage.local.remove(RULES_KEY);
}
