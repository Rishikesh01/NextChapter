// Pure URL → reading-position detection. No I/O, no browser globals — unit
// tested without a browser. Site rules are authored in Go/RE2 syntax on the
// server (see backend/internal/sites/defaults.go); this module owns the
// translation to JS RegExp (ADR-0008 §7).
import type { SiteRule } from '@nextchapter/api-client';

export interface DetectedPosition {
  /** Normalized host, matches what the server stores (lowercase, no leading www.). */
  siteHost: string;
  seriesSlug: string;
  /** Chapter number ready for EntryCapture.chapter; may be fractional. */
  chapter: number;
  /** The rule's host as configured, for display. */
  ruleHost: string;
}

/** Mirrors the server's normalization: lowercase, strip one leading "www.". */
export function normalizeHost(hostname: string): string {
  const lower = hostname.toLowerCase();
  return lower.startsWith('www.') ? lower.slice(4) : lower;
}

/**
 * Go named groups are `(?P<name>…)`; JS RegExp only accepts `(?<name>…)`.
 * Everything else in the server's RE2 subset is ECMAScript-compatible.
 */
export function goRegexToJs(pattern: string): string {
  return pattern.replaceAll('(?P<', '(?<');
}

/**
 * A rule's chapter group may capture a whole path segment (the shipped
 * wuxiaworld.com default captures "de-book-2-chapter-15"), so take the LAST
 * numeric run rather than parseFloat-ing the front — that also picks the
 * chapter rather than the book when both are present.
 */
export function parseChapterNumber(captured: string): number | null {
  const runs = captured.match(/\d+(?:\.\d+)?/g);
  const last = runs?.at(-1);
  if (last === undefined) return null;
  const value = Number.parseFloat(last);
  return Number.isFinite(value) && value >= 0 ? value : null;
}

/**
 * First rule whose host matches and whose regex matches the URL's pathname
 * wins. Rules that are incomplete or whose translated regex fails to compile
 * are skipped, never thrown.
 */
export function detectPosition(
  rules: readonly SiteRule[],
  url: string,
): DetectedPosition | null {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return null;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;

  const host = normalizeHost(parsed.hostname);
  for (const rule of rules) {
    if (
      rule.host === undefined ||
      rule.chapter_url_regex === undefined ||
      rule.slug_capture_group === undefined ||
      rule.chapter_capture_group === undefined
    ) {
      continue;
    }
    if (normalizeHost(rule.host) !== host) continue;

    let regex: RegExp;
    try {
      regex = new RegExp(goRegexToJs(rule.chapter_url_regex));
    } catch {
      continue;
    }

    const groups = regex.exec(parsed.pathname)?.groups;
    if (groups === undefined) continue;
    const slug = groups[rule.slug_capture_group];
    const chapterRaw = groups[rule.chapter_capture_group];
    if (slug === undefined || slug === '' || chapterRaw === undefined) continue;
    const chapter = parseChapterNumber(chapterRaw);
    if (chapter === null) continue;

    return { siteHost: host, seriesSlug: slug, chapter, ruleHost: rule.host };
  }
  return null;
}
