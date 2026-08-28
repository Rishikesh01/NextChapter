import { describe, expect, it } from 'vitest';
import {
  dataUrlToBlob,
  rankCandidates,
  type CoverCandidate,
} from '../../lib/covers';

function candidate(over: Partial<CoverCandidate> = {}): CoverCandidate {
  return {
    url: 'https://cdn.test/a.jpg',
    width: 400,
    height: 600,
    declared: false,
    ...over,
  };
}

describe('rankCandidates', () => {
  it('puts the page’s declared artwork first', () => {
    const ranked = rankCandidates([
      candidate({ url: 'https://cdn.test/big.jpg', width: 800, height: 1200 }),
      candidate({ url: 'https://cdn.test/og.jpg', declared: true }),
    ]);
    expect(ranked[0]?.url).toBe('https://cdn.test/og.jpg');
  });

  it('prefers portrait over landscape among undeclared images', () => {
    const ranked = rankCandidates([
      // A wide banner, larger in raw area than the cover next to it.
      candidate({
        url: 'https://cdn.test/banner.jpg',
        width: 1200,
        height: 400,
      }),
      candidate({ url: 'https://cdn.test/cover.jpg', width: 300, height: 450 }),
    ]);
    expect(ranked[0]?.url).toBe('https://cdn.test/cover.jpg');
  });

  it('sorts larger before smaller once shape is equal', () => {
    const ranked = rankCandidates([
      candidate({ url: 'https://cdn.test/small.jpg', width: 200, height: 300 }),
      candidate({ url: 'https://cdn.test/large.jpg', width: 600, height: 900 }),
    ]);
    expect(ranked.map((c) => c.url)).toEqual([
      'https://cdn.test/large.jpg',
      'https://cdn.test/small.jpg',
    ]);
  });

  it('drops icons, sprites and tracking pixels', () => {
    const ranked = rankCandidates([
      candidate({ url: 'https://cdn.test/pixel.gif', width: 1, height: 1 }),
      candidate({ url: 'https://cdn.test/icon.png', width: 32, height: 32 }),
      candidate({ url: 'https://cdn.test/cover.jpg', width: 300, height: 450 }),
    ]);
    expect(ranked.map((c) => c.url)).toEqual(['https://cdn.test/cover.jpg']);
  });

  it('keeps unmeasured images, which cannot be judged on shape', () => {
    // A meta tag has no dimensions until it is loaded; discarding it would
    // throw away the most likely cover on most sites.
    const ranked = rankCandidates([
      candidate({
        url: 'https://cdn.test/og.jpg',
        width: 0,
        height: 0,
        declared: true,
      }),
    ]);
    expect(ranked).toHaveLength(1);
  });

  it('caps the grid so a gallery page cannot flood the popup', () => {
    const many = Array.from({ length: 60 }, (_, i) =>
      candidate({ url: `https://cdn.test/${String(i)}.jpg` }),
    );
    expect(rankCandidates(many)).toHaveLength(24);
  });

  it('returns a new array rather than sorting the caller’s', () => {
    const input = [
      candidate({ url: 'https://cdn.test/b.jpg', width: 200, height: 300 }),
      candidate({ url: 'https://cdn.test/a.jpg', width: 600, height: 900 }),
    ];
    rankCandidates(input);
    expect(input[0]?.url).toBe('https://cdn.test/b.jpg');
  });
});

describe('dataUrlToBlob', () => {
  it('decodes base64 payloads back to the original bytes', async () => {
    // 0x89 'P' 'N' 'G' — the PNG magic, and not valid UTF-8, so a
    // string round trip would corrupt it.
    const blob = dataUrlToBlob('data:image/png;base64,iVBORw==');
    expect(blob).not.toBeNull();
    if (blob === null) return; // narrows for the reads below
    expect(blob.type).toBe('image/png');
    const bytes = new Uint8Array(await blob.arrayBuffer());
    expect([...bytes]).toEqual([0x89, 0x50, 0x4e, 0x47]);
  });

  it('decodes percent-encoded (non-base64) payloads', async () => {
    const blob = dataUrlToBlob('data:image/svg+xml,%3Csvg%2F%3E');
    expect(blob).not.toBeNull();
    if (blob === null) return;
    expect(await blob.text()).toBe('<svg/>');
  });

  it('returns null for malformed input rather than throwing', () => {
    expect(dataUrlToBlob('not-a-data-url')).toBeNull();
    expect(dataUrlToBlob('data:image/png;base64,!!!not-base64!!!')).toBeNull();
  });
});
