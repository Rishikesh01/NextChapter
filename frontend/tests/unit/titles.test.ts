import { describe, expect, it } from 'vitest';
import { prettifySlug, suggestSeriesTitle } from '../../lib/titles';

describe('prettifySlug', () => {
  it.each([
    ['solo-leveling', 'Solo Leveling'],
    ['the-beginning-after-the-end', 'The Beginning After The End'],
    ['omniscient_readers_viewpoint', 'Omniscient Readers Viewpoint'],
    ['tbate', 'Tbate'],
    ['', ''],
  ])('%s -> %s', (input, expected) => {
    expect(prettifySlug(input)).toBe(expected);
  });
});

describe('suggestSeriesTitle', () => {
  it('cuts the site-name suffix and a trailing chapter fragment', () => {
    expect(
      suggestSeriesTitle(
        'Solo Leveling Chapter 101 - Example Reader',
        'solo-leveling',
      ),
    ).toBe('Solo Leveling');
  });

  it('handles pipe separators and fractional chapters', () => {
    expect(
      suggestSeriesTitle(
        'TBATE Ch. 45.5 | comics',
        'the-beginning-after-the-end',
      ),
    ).toBe('TBATE');
  });

  it('keeps a plain title untouched', () => {
    expect(suggestSeriesTitle('Omniscient Reader', 'orv')).toBe(
      'Omniscient Reader',
    );
  });

  it('falls back to the prettified slug when the title empties out', () => {
    expect(suggestSeriesTitle('Chapter 12 - Some Site', 'solo-leveling')).toBe(
      'Solo Leveling',
    );
    expect(suggestSeriesTitle('', 'solo-leveling')).toBe('Solo Leveling');
  });
});
