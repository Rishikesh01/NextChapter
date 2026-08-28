// Fetch-and-overwrite for the site-rule cache. Rule create/delete calls this
// so detection picks the change up immediately instead of waiting out the TTL
// (ADR-0009 §3).
import { extensionApiClient } from './api';
import { setRulesCache } from './storage';
import { toRulesCache, type RulesCache } from './rules-cache';

export async function refreshRulesCache(): Promise<RulesCache> {
  const list = await extensionApiClient().getSites();
  const cache = toRulesCache(list, Date.now());
  await setRulesCache(cache);
  return cache;
}
