import { describe, expect, it } from 'vitest';
import {
  AUTO_TRACK_DWELL_MS,
  dwellElapsed,
  originPatternFor,
  VisitTracker,
} from '../../lib/autotrack';

describe('originPatternFor', () => {
  it('covers both schemes for exactly one host', () => {
    // Not `*://*.host/*`: a rule for reader.example.com must not hand the
    // extension access to every subdomain of it.
    expect(originPatternFor('reader.example.com')).toBe('*://reader.example.com/*');
  });
});

describe('dwellElapsed', () => {
  const visit = { tabId: 1, url: 'https://x.test/c/1', since: 1_000 };

  it('is false before the dwell and true at or after it', () => {
    expect(dwellElapsed(visit, 1_000 + AUTO_TRACK_DWELL_MS - 1)).toBe(false);
    expect(dwellElapsed(visit, 1_000 + AUTO_TRACK_DWELL_MS)).toBe(true);
    expect(dwellElapsed(visit, 1_000 + AUTO_TRACK_DWELL_MS + 5_000)).toBe(true);
  });

  it('accepts an override so the rule can be tuned without a clock', () => {
    // since=1_000, so at now=2_500 exactly 1_500ms has elapsed.
    expect(dwellElapsed(visit, 2_500, 1_000)).toBe(true);
    expect(dwellElapsed(visit, 2_500, 2_000)).toBe(false);
  });
});

describe('VisitTracker', () => {
  it('starts the clock when a tab first shows a URL', () => {
    const tracker = new VisitTracker();
    const visit = tracker.note(1, 'https://x.test/c/1', 5_000);
    expect(visit).toEqual({
      tabId: 1,
      url: 'https://x.test/c/1',
      since: 5_000,
    });
  });

  it('does NOT restart the clock when the same URL is re-seen', () => {
    // A single page load fires onUpdated repeatedly (loading → complete,
    // plus history events). Restarting each time would keep pushing the
    // deadline out and the capture would never fire.
    const tracker = new VisitTracker();
    tracker.note(1, 'https://x.test/c/1', 5_000);
    const again = tracker.note(1, 'https://x.test/c/1', 7_500);
    expect(again.since).toBe(5_000);
  });

  it('restarts the clock when the tab navigates to a different URL', () => {
    const tracker = new VisitTracker();
    tracker.note(1, 'https://x.test/c/1', 5_000);
    const next = tracker.note(1, 'https://x.test/c/2', 9_000);
    expect(next.since).toBe(9_000);
  });

  it('keeps a separate dwell per tab', () => {
    // Readers routinely have several chapter tabs open; one tab's
    // navigation must not reset another's progress toward capture.
    const tracker = new VisitTracker();
    tracker.note(1, 'https://x.test/c/1', 1_000);
    tracker.note(2, 'https://x.test/c/9', 4_000);
    expect(tracker.get(1)?.since).toBe(1_000);
    expect(tracker.get(2)?.since).toBe(4_000);
  });

  it('forgets a tab that closed', () => {
    const tracker = new VisitTracker();
    tracker.note(1, 'https://x.test/c/1', 1_000);
    tracker.forget(1);
    expect(tracker.get(1)).toBeUndefined();
  });

  it('will not capture the same tab+URL twice', () => {
    // A reload, or a late onUpdated, must not re-POST a capture that
    // already landed.
    const tracker = new VisitTracker();
    tracker.note(1, 'https://x.test/c/1', 1_000);
    expect(tracker.wasCaptured(1, 'https://x.test/c/1')).toBe(false);
    tracker.markCaptured(1);
    expect(tracker.wasCaptured(1, 'https://x.test/c/1')).toBe(true);
  });

  it('scopes the captured mark to that exact tab and URL', () => {
    const tracker = new VisitTracker();
    tracker.note(1, 'https://x.test/c/1', 1_000);
    tracker.markCaptured(1);
    // The next chapter on the same tab is a fresh capture...
    expect(tracker.wasCaptured(1, 'https://x.test/c/2')).toBe(false);
    // ...and so is the same chapter opened in another tab.
    expect(tracker.wasCaptured(2, 'https://x.test/c/1')).toBe(false);
  });
});
