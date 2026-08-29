import { describe, expect, it } from 'vitest';
import type { SiteRule } from '@nextchapter/api-client';
import {
  detectPosition,
  goRegexToJs,
  normalizeHost,
  parseChapterNumber,
} from '../../lib/url-detection';
import fixture from './fixtures/url-detection.json';

interface FixtureCase {
  name: string;
  url: string;
  expect: { siteHost: string; seriesSlug: string; chapter: number } | null;
}

const defaultRules = fixture.defaultRules as SiteRule[];
const cases = fixture.cases as FixtureCase[];

describe('detectPosition against the shipped default rules', () => {
  it.each(cases)('$name', ({ url, expect: expected }) => {
    const result = detectPosition(defaultRules, url);
    if (expected === null) {
      expect(result).toBeNull();
    } else {
      expect(result).toMatchObject(expected);
    }
  });

  it('skips a rule whose translated regex does not compile and keeps trying', () => {
    const rules: SiteRule[] = [
      {
        host: 'reader.example.com',
        // RE2 inline flag — invalid in JS RegExp; the rule must be skipped, not thrown.
        chapter_url_regex:
          '(?i)^/series/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+)$',
        slug_capture_group: 'slug',
        chapter_capture_group: 'chapter',
      },
      ...defaultRules,
    ];
    const result = detectPosition(
      rules,
      'https://reader.example.com/series/tbate/chapter-9',
    );
    expect(result).toMatchObject({ seriesSlug: 'tbate', chapter: 9 });
  });

  it('skips a rule whose named groups are missing from the pattern', () => {
    const rules: SiteRule[] = [
      {
        host: 'example.org',
        chapter_url_regex: '^/read/(?P<slug>[^/]+)$',
        slug_capture_group: 'slug',
        chapter_capture_group: 'chapter',
      },
    ];
    expect(
      detectPosition(rules, 'https://example.org/read/solo-leveling'),
    ).toBeNull();
  });
});

describe('goRegexToJs', () => {
  it('translates every Go named group', () => {
    expect(goRegexToJs('^/a/(?P<x>\\d+)/(?P<y>\\w+)$')).toBe(
      '^/a/(?<x>\\d+)/(?<y>\\w+)$',
    );
  });

  it('leaves JS-style groups and everything else untouched', () => {
    expect(goRegexToJs('^/a/(?<x>\\d+)(?:/b)?$')).toBe(
      '^/a/(?<x>\\d+)(?:/b)?$',
    );
  });
});

describe('parseChapterNumber', () => {
  it.each([
    ['101', 101],
    ['45.5', 45.5],
    ['en-chapter-45.5', 45.5],
    ['vol-2-chapter-7', 7],
    ['0', 0],
  ])('extracts %s -> %d', (input, expected) => {
    expect(parseChapterNumber(input)).toBe(expected);
  });

  it('returns null when there is no numeric run', () => {
    expect(parseChapterNumber('prologue')).toBeNull();
  });
});

describe('normalizeHost', () => {
  it.each([
    ['WWW.Reader.Example.com', 'reader.example.com'],
    ['comics.example.org', 'comics.example.org'],
    ['www.www.example.org', 'www.example.org'],
  ])('%s -> %s', (input, expected) => {
    expect(normalizeHost(input)).toBe(expected);
  });
});
