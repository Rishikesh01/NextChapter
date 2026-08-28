import { browser } from 'wxt/browser';
import { defineBackground } from 'wxt/utils/define-background';
import { ApiError, type SiteRule } from '@nextchapter/api-client';
import { extensionApiClient } from '../lib/api';
import { getRulesCache, getSettings } from '../lib/storage';
import { detectPosition, normalizeHost } from '../lib/url-detection';
import {
  AUTO_TRACK_DWELL_MS,
  isAutoTrackActive,
  VisitTracker,
} from '../lib/autotrack';

/**
 * Auto-tracking worker (ADR-0012).
 *
 * This is the extension's only code that runs without a user gesture, so
 * it is deliberately narrow: it reacts to tab URL changes on hosts the
 * user explicitly switched on, waits out a dwell, and advances an entry
 * that already exists. It never creates a series, never injects into a
 * page, and does nothing at all on a host without a granted permission.
 */

const client = extensionApiClient();
const visits = new VisitTracker();
/** Pending dwell timers, keyed by tab so navigation can cancel them. */
const timers = new Map<number, ReturnType<typeof setTimeout>>();

function cancel(tabId: number): void {
  const timer = timers.get(tabId);
  if (timer !== undefined) {
    clearTimeout(timer);
    timers.delete(tabId);
  }
}

/**
 * Badge feedback. Auto-capture is meant to be forgotten about, so success
 * is a brief tick rather than a notification; an auth failure persists,
 * because a tracker that has silently stopped working must not look like
 * one that is working.
 */
async function flashBadge(text: string, color: string): Promise<void> {
  await browser.action.setBadgeBackgroundColor({ color });
  await browser.action.setBadgeText({ text });
  if (text === '✓') {
    setTimeout(() => {
      void browser.action.setBadgeText({ text: '' });
    }, 3_000);
  }
}

/**
 * Runs once a dwell has elapsed: re-checks that the page still matches,
 * then advances the entry.
 *
 * Advance-only by construction — no series_id and no new_series_title are
 * sent, so an unknown (host, slug) comes back 422 and is dropped. That is
 * the designed outcome, not an error: adopting a new series is a decision
 * only the user can make, through the popup.
 */
async function captureVisit(tabId: number, url: string): Promise<void> {
  const settings = await getSettings();
  if (settings === undefined) return;

  const host = normalizeHost(new URL(url).hostname);
  if (!(await isAutoTrackActive(host))) return;

  const cache = await getRulesCache();
  const rules: SiteRule[] = cache?.rules ?? [];
  const detected = detectPosition(rules, url);
  if (detected === null) return;

  // The tab may have moved on while the dwell ran.
  let current: string | undefined;
  try {
    current = (await browser.tabs.get(tabId)).url;
  } catch {
    return; // tab closed
  }
  if (current !== url) return;

  try {
    await client.capture({
      site_host: host,
      series_slug: detected.seriesSlug,
      site_title: (await browser.tabs.get(tabId)).title ?? '',
      chapter: detected.chapter,
      url,
    });
    visits.markCaptured(tabId);
    await flashBadge('✓', '#16a34a');
  } catch (err) {
    if (err instanceof ApiError && err.unauthorized) {
      await flashBadge('!', '#dc2626');
      return;
    }
    // A 422 here is the advance-only boundary: this (host, slug) is not
    // tracked yet, so there is nothing to advance and nothing to report.
    // Network failures are equally silent — the next chapter retries.
  }
}

/** Schedules a capture for a matching URL, replacing any pending one. */
function scheduleCapture(tabId: number, url: string, now: number): void {
  if (visits.wasCaptured(tabId, url)) return;
  const visit = visits.note(tabId, url, now);
  cancel(tabId);
  const remaining = Math.max(0, visit.since + AUTO_TRACK_DWELL_MS - now);
  timers.set(
    tabId,
    setTimeout(() => {
      timers.delete(tabId);
      void captureVisit(tabId, url);
    }, remaining),
  );
}

export default defineBackground(() => {
  browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
    // `url` is only populated for tabs we hold host permission for, so a
    // site the user has not switched on is invisible here — the privacy
    // property is enforced by the browser, not by this check.
    const url =
      changeInfo.url ??
      (changeInfo.status === 'complete' ? tab.url : undefined);
    if (url === undefined) return;
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      cancel(tabId);
      visits.forget(tabId);
      return;
    }
    scheduleCapture(tabId, url, Date.now());
  });

  browser.tabs.onRemoved.addListener((tabId) => {
    cancel(tabId);
    visits.forget(tabId);
  });
});
