import { describe, expect, it } from 'vitest';
import type { SiteRuleNew } from '@nextchapter/api-client';
import {
  buildRule,
  canOfferRuleBuilder,
  guessDraft,
  pathSegments,
  previewRule,
  type RuleDraft,
} from '../../lib/rule-builder';

const manhua =
  'https://manhua.example.net/manga/the-mad-dog-of-the-dukes-estate/chapter-54/';

function mustBuild(url: string, draft: RuleDraft): SiteRuleNew {
  const rule = buildRule(url, draft);
  if (rule === null) throw new Error('buildRule unexpectedly returned null');
  return rule;
}

describe('pathSegments', () => {
  it('splits and drops empty segments (trailing slash)', () => {
    expect(pathSegments(manhua)).toEqual([
      'manga',
      'the-mad-dog-of-the-dukes-estate',
      'chapter-54',
    ]);
  });

  it('is null for non-http URLs, bad URLs, and bare hosts', () => {
    expect(pathSegments('chrome://extensions')).toBeNull();
    expect(pathSegments('not a url')).toBeNull();
    expect(pathSegments('https://example.org/')).toBeNull();
  });
});

describe('guessDraft', () => {
  it('picks the chapter-hinted segment and the one before it as slug', () => {
    expect(
      guessDraft(['manga', 'the-mad-dog-of-the-dukes-estate', 'chapter-54']),
    ).toEqual({
      slugIndex: 1,
      chapterIndex: 2,
    });
  });

  it('prefers a chapter-ish keyword over a later numeric segment', () => {
    expect(
      guessDraft(['manga', 'solo-leveling', 'chapter-3', 'page-2']),
    ).toEqual({
      slugIndex: 1,
      chapterIndex: 2,
    });
  });

  it('falls back to the last numeric segment without a keyword', () => {
    expect(guessDraft(['read', 'some-novel', '12'])).toEqual({
      slugIndex: 1,
      chapterIndex: 2,
    });
  });

  it('handles digits inside the slug', () => {
    expect(guessDraft(['manga', '86-eighty-six', 'chapter-3'])).toEqual({
      slugIndex: 1,
      chapterIndex: 2,
    });
  });

  it('is null with no numeric segment or with a single segment', () => {
    expect(guessDraft(['manga', 'solo-leveling'])).toBeNull();
    expect(guessDraft(['chapter-54'])).toBeNull();
  });
});

describe('canOfferRuleBuilder', () => {
  it('offers on a registrable host with a guessable shape', () => {
    expect(canOfferRuleBuilder(manhua)).toBe(true);
    expect(
      canOfferRuleBuilder('http://localhost:18081/read/fresh-novel/7'),
    ).toBe(true);
  });

  it('never offers on IP-literal hosts (the backend rejects them as rule hosts)', () => {
    expect(canOfferRuleBuilder('http://192.168.1.5/read/some-novel/7')).toBe(
      false,
    );
    expect(canOfferRuleBuilder('http://[::1]/read/some-novel/7')).toBe(false);
  });

  it('never offers when no rule could be guessed from the URL', () => {
    expect(canOfferRuleBuilder('https://example.org/about')).toBe(false);
    expect(canOfferRuleBuilder('https://example.org/manga/solo-leveling')).toBe(
      false,
    );
    expect(canOfferRuleBuilder('not a url')).toBe(false);
  });
});

describe('buildRule', () => {
  it('generates the defaults-shaped Go-syntax rule from a real URL', () => {
    const rule = buildRule(manhua, { slugIndex: 1, chapterIndex: 2 });
    expect(rule).toEqual({
      host: 'manhua.example.net',
      chapter_url_regex:
        '^/manga/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+(?:\\.[0-9]+)?)/?$',
      slug_capture_group: 'slug',
      chapter_capture_group: 'chapter',
    });
  });

  it('round-trips through detectPosition on the source URL (trailing slash kept)', () => {
    const rule = mustBuild(manhua, { slugIndex: 1, chapterIndex: 2 });
    expect(previewRule(manhua, rule)).toMatchObject({
      siteHost: 'manhua.example.net',
      seriesSlug: 'the-mad-dog-of-the-dukes-estate',
      chapter: 54,
    });
  });

  it('matches sibling chapters, including fractional ones, but not other slugs paths', () => {
    const rule = mustBuild(manhua, { slugIndex: 1, chapterIndex: 2 });
    expect(
      previewRule(
        'https://manhua.example.net/manga/solo-leveling/chapter-45.5',
        rule,
      ),
    ).toMatchObject({ seriesSlug: 'solo-leveling', chapter: 45.5 });
    expect(
      previewRule('https://manhua.example.net/novel/solo-leveling/chapter-2', rule),
    ).toBeNull();
    expect(
      previewRule('https://other.site/manga/solo-leveling/chapter-2', rule),
    ).toBeNull();
  });

  it('keeps literal text around the chapter number', () => {
    const url = 'https://comics.example.org/comic/orv/en-chapter-45.5';
    const rule = mustBuild(url, { slugIndex: 1, chapterIndex: 2 });
    expect(rule.chapter_url_regex).toBe(
      '^/comic/(?P<slug>[^/]+)/en-chapter-(?P<chapter>[0-9]+(?:\\.[0-9]+)?)/?$',
    );
    expect(previewRule(url, rule)).toMatchObject({
      seriesSlug: 'orv',
      chapter: 45.5,
    });
  });

  it('handles a bare-number chapter segment', () => {
    const url = 'https://example.org/read/some-novel/12';
    const rule = mustBuild(url, { slugIndex: 1, chapterIndex: 2 });
    expect(rule.chapter_url_regex).toBe(
      '^/read/(?P<slug>[^/]+)/(?P<chapter>[0-9]+(?:\\.[0-9]+)?)/?$',
    );
  });

  it('escapes regex specials in literal segments', () => {
    const url = 'https://example.org/m.a+n(ga)/some-novel/ch-7';
    const rule = mustBuild(url, { slugIndex: 1, chapterIndex: 2 });
    expect(rule.chapter_url_regex).toBe(
      '^/m\\.a\\+n\\(ga\\)/(?P<slug>[^/]+)/ch-(?P<chapter>[0-9]+(?:\\.[0-9]+)?)/?$',
    );
    expect(previewRule(url, rule)).toMatchObject({
      seriesSlug: 'some-novel',
      chapter: 7,
    });
  });

  it('normalizes the host (www., case)', () => {
    const rule = mustBuild('https://WWW.manhua.example.net/manga/x-y/chapter-1', {
      slugIndex: 1,
      chapterIndex: 2,
    });
    expect(rule.host).toBe('manhua.example.net');
  });

  it('survives the popup call sequence with single-digit segments (regression: shared /g regex lastIndex)', () => {
    // canOfferRuleBuilder → guessDraft → buildRule, exactly as the popup runs
    // them; a stateful global regex once made buildRule return null here.
    const url = 'http://localhost:18081/read/fresh-novel/7';
    const segments = pathSegments(url);
    if (segments === null) throw new Error('expected path segments');
    const draft = guessDraft(segments);
    if (draft === null) throw new Error('expected a guessed draft');
    const rule = mustBuild(url, draft);
    expect(previewRule(url, rule)).toMatchObject({
      seriesSlug: 'fresh-novel',
      chapter: 7,
    });
    // and again — no state may leak between invocations
    expect(buildRule(url, draft)).not.toBeNull();
  });

  it('rejects invalid drafts', () => {
    expect(buildRule(manhua, { slugIndex: 2, chapterIndex: 2 })).toBeNull();
    expect(buildRule(manhua, { slugIndex: 0, chapterIndex: 9 })).toBeNull();
    // chapter segment without a number
    expect(
      buildRule('https://example.org/manga/slug/extras', {
        slugIndex: 1,
        chapterIndex: 2,
      }),
    ).toBeNull();
  });
});
