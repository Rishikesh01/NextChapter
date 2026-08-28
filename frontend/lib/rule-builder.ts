// Builds a site rule from a concrete chapter URL — the user picks which path
// segment is the series slug and which carries the chapter number; no regex is
// ever shown or typed (ADR-0009 §2). Pure: no I/O, no browser globals.
import type { SiteRuleNew } from '@nextchapter/api-client';
import {
  detectPosition,
  normalizeHost,
  type DetectedPosition,
} from './url-detection';

export interface RuleDraft {
  /** Index into pathSegments() of the series-slug segment. */
  slugIndex: number;
  /** Index of the segment carrying the chapter number. Must differ from slugIndex. */
  chapterIndex: number;
}

// No shared /g regex here: a global regex carries lastIndex state across
// calls (test() advances it, matchAll starts from it), which once made
// buildRule blind to single-digit segments after guessDraft had run.
const HAS_NUMBER = /\d/;
const CHAPTER_HINT = /chapter|\bch\b|episode|\bep\b/i;

function numericRuns(segment: string): RegExpMatchArray[] {
  return [...segment.matchAll(/\d+(?:\.\d+)?/g)];
}

/** Non-empty path segments of an http(s) URL, or null when it has none. */
export function pathSegments(url: string): string[] | null {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return null;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return null;
  const segments = parsed.pathname
    .split('/')
    .filter((segment) => segment !== '');
  return segments.length > 0 ? segments : null;
}

/**
 * Pre-guess the draft: the chapter segment is the last one containing a
 * number, preferring segments with a chapter-ish keyword; the slug is the
 * segment just before it (falling back to the longest other segment). Null
 * when the URL has no usable shape (no number, or only one segment).
 */
export function guessDraft(segments: string[]): RuleDraft | null {
  if (segments.length < 2) return null;

  const numeric = segments
    .map((segment, index) => ({ segment, index }))
    .filter(({ segment }) => HAS_NUMBER.test(segment));
  if (numeric.length === 0) return null;
  const hinted = numeric.filter(({ segment }) => CHAPTER_HINT.test(segment));
  const chapterIndex = (hinted.at(-1) ?? numeric.at(-1))?.index;
  if (chapterIndex === undefined) return null;

  let slugIndex = chapterIndex - 1;
  if (slugIndex < 0 || segments[slugIndex] === undefined) {
    const longest = segments
      .map((segment, index) => ({ segment, index }))
      .filter(({ index }) => index !== chapterIndex)
      .sort((a, b) => b.segment.length - a.segment.length)
      .at(0);
    if (longest === undefined) return null;
    slugIndex = longest.index;
  }
  return { slugIndex, chapterIndex };
}

function escapeRegex(literal: string): string {
  return literal.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Generate the rule in Go syntax (`(?P<name>…)` groups), mirroring the shape
 * of the shipped defaults: literal segments escaped, the chapter segment's
 * LAST numeric run generalized, optional trailing slash. Text before a
 * chapter keyword inside the chapter segment is generalized too —
 * "en-chapter-45.5" becomes `[^/]+-chapter-…` so a language prefix doesn't
 * pin the rule to one translation (the shipped comics.example.org default's shape).
 * Null for an invalid draft (same segment twice, out-of-range index, chapter
 * segment without a number).
 */
export function buildRule(url: string, draft: RuleDraft): SiteRuleNew | null {
  const segments = pathSegments(url);
  if (segments === null) return null;
  if (draft.slugIndex === draft.chapterIndex) return null;
  const chapterSegment = segments[draft.chapterIndex];
  if (segments[draft.slugIndex] === undefined || chapterSegment === undefined)
    return null;

  const lastRun = numericRuns(chapterSegment).at(-1);
  if (lastRun?.index === undefined) return null;
  const runStart = lastRun.index;
  const literalPrefix = chapterSegment.slice(0, runStart);
  // "en-chapter-" → "[^/]+-chapter-": anything before the keyword varies
  // (language, volume), the keyword and number structure don't.
  const keywordMatch = /^.+-(chapter|ch|episode|ep)-$/i.exec(literalPrefix);
  const keyword = keywordMatch?.[1];
  const prefixPattern =
    keyword !== undefined
      ? `[^/]+-${escapeRegex(keyword)}-`
      : escapeRegex(literalPrefix);
  const chapterPattern =
    prefixPattern +
    '(?P<chapter>[0-9]+(?:\\.[0-9]+)?)' +
    escapeRegex(chapterSegment.slice(runStart + lastRun[0].length));

  const pattern = segments
    .map((segment, index) => {
      if (index === draft.slugIndex) return '/(?P<slug>[^/]+)';
      if (index === draft.chapterIndex) return `/${chapterPattern}`;
      return `/${escapeRegex(segment)}`;
    })
    .join('');

  return {
    host: normalizeHost(new URL(url).hostname),
    chapter_url_regex: `^${pattern}/?$`,
    slug_capture_group: 'slug',
    chapter_capture_group: 'chapter',
  };
}

/**
 * What the drafted rule would detect on the given URL — the live preview, and
 * the pre-save validation: a null preview means the rule must not be saved.
 * Runs through the same detectPosition pipeline the popup uses.
 */
export function previewRule(
  url: string,
  rule: SiteRuleNew,
): DetectedPosition | null {
  return detectPosition([rule], url);
}

const IPV4 = /^\d{1,3}(?:\.\d{1,3}){3}$/;

/**
 * Whether the "Create a rule from this page" entry point should be offered:
 * a registrable host (the backend rejects IP literals) and a URL shape a rule
 * can be guessed from. When false, the manual form shows no entry point at
 * all (design/components/rule-builder.html).
 */
export function canOfferRuleBuilder(url: string): boolean {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return false;
  }
  const host = parsed.hostname;
  if (IPV4.test(host) || host.includes(':') || host.includes('[')) return false;
  const segments = pathSegments(url);
  return segments !== null && guessDraft(segments) !== null;
}
