import { createServer, type IncomingMessage, type Server } from 'node:http';
import { afterAll, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { ApiError, createApiClient, type ApiClient } from '../src/index';

interface SeenRequest {
  method: string;
  url: string;
  headers: IncomingMessage['headers'];
  body: string;
  /** Raw bytes, for the binary (cover upload) path. */
  raw: Buffer;
}

// A real HTTP server standing in for the backend: it records every request
// and replays canned responses. Nothing about fetch is mocked.
let server: Server;
let baseUrl: string;
let seen: SeenRequest[];
let nextResponse: { status: number; body?: unknown };

function makeClient(token?: string): ApiClient {
  return createApiClient(() => Promise.resolve({ baseUrl, token }));
}

beforeAll(async () => {
  server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      const raw = Buffer.concat(chunks);
      seen.push({
        method: req.method ?? '',
        url: req.url ?? '',
        headers: req.headers,
        body: raw.toString(),
        raw,
      });
      res.writeHead(nextResponse.status, {
        'Content-Type': 'application/json',
      });
      res.end(
        nextResponse.body !== undefined
          ? JSON.stringify(nextResponse.body)
          : '',
      );
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (address === null || typeof address === 'string')
    throw new Error('no server address');
  baseUrl = `http://127.0.0.1:${String(address.port)}`;
});

afterAll(async () => {
  await new Promise((resolve) => server.close(resolve));
});

beforeEach(() => {
  seen = [];
  nextResponse = { status: 200, body: {} };
});

describe('bearer channel', () => {
  it('sends the Authorization header and parses the response', async () => {
    nextResponse = { status: 200, body: { id: 1, username: 'rishi' } };
    const user = await makeClient('nca_test').me();

    expect(user.username).toBe('rishi');
    expect(seen[0]?.headers.authorization).toBe('Bearer nca_test');
    expect(seen[0]?.headers.cookie).toBeUndefined();
  });

  it('rejects without a network call when no token is configured', async () => {
    await expect(makeClient(undefined).me()).rejects.toMatchObject({
      status: 401,
      message: 'no API token configured',
    });
    expect(seen).toHaveLength(0);
  });

  it('builds the series list query string', async () => {
    nextResponse = { status: 200, body: { items: [], total: 0 } };
    await makeClient('nca_test').listSeries({
      status: 'reading',
      tag: ['action', 'isekai'],
      limit: 200,
      offset: 50,
    });

    expect(seen[0]?.url).toBe(
      '/series?status=reading&tag=action&tag=isekai&limit=200&offset=50',
    );
  });

  it('normalizes a trailing slash in the base URL', async () => {
    nextResponse = { status: 200, body: { status: 'ok' } };
    const client = createApiClient(() =>
      Promise.resolve({ baseUrl: `${baseUrl}/` }),
    );
    await client.health();

    expect(seen[0]?.url).toBe('/healthz');
  });
});

describe('capture', () => {
  const captureBody = {
    site_host: 'reader.example.com',
    series_slug: 'solo-leveling',
    site_title: 'Solo Leveling Ch 101',
    chapter: 101,
    url: 'https://reader.example.com/series/solo-leveling/chapter-101',
  };

  it('reports created=true on 201', async () => {
    nextResponse = { status: 201, body: { id: 7, last_chapter: 101 } };
    const result = await makeClient('nca_test').capture(captureBody);

    expect(result.created).toBe(true);
    expect(result.entry.id).toBe(7);
    expect(JSON.parse(seen[0]?.body ?? '')).toEqual(captureBody);
  });

  it('reports created=false on 200 (advanced an existing entry)', async () => {
    nextResponse = { status: 200, body: { id: 7, last_chapter: 101 } };
    const result = await makeClient('nca_test').capture(captureBody);

    expect(result.created).toBe(false);
  });

  it('maps the 422 series-binding envelope onto ApiError', async () => {
    nextResponse = {
      status: 422,
      body: {
        error: {
          code: 'validation',
          message: 'validation failed',
          fields: { series_id: 'required (or new_series_title)' },
        },
      },
    };

    const err = await makeClient('nca_test')
      .capture(captureBody)
      .then(
        () => null,
        (e: unknown) => e,
      );

    expect(err).toBeInstanceOf(ApiError);
    const apiErr = err as ApiError;
    expect(apiErr.status).toBe(422);
    expect(apiErr.code).toBe('validation');
    expect(apiErr.needsSeriesBinding).toBe(true);
    expect(apiErr.unauthorized).toBe(false);
  });
});

describe('cookie channel', () => {
  it('never sends an Authorization header on auth calls', async () => {
    nextResponse = { status: 200, body: { id: 1, username: 'rishi' } };
    const client = makeClient('nca_should-not-be-sent');
    await client.auth.login({ username: 'rishi', password: 'hunter22' });

    expect(seen[0]?.headers.authorization).toBeUndefined();
    expect(seen[0]?.method).toBe('POST');
    expect(seen[0]?.url).toBe('/auth/login');
  });

  it('mints a token and surfaces the one-time plaintext', async () => {
    nextResponse = {
      status: 201,
      body: { id: 3, label: 'extension', token: 'nca_minted' },
    };
    const minted = await makeClient(undefined).auth.mintToken({
      label: 'extension',
    });

    expect(minted.token).toBe('nca_minted');
  });
});

describe('errors', () => {
  it('maps 401 onto unauthorized', async () => {
    nextResponse = {
      status: 401,
      body: { error: { code: 'unauthorized', message: 'invalid credentials' } },
    };

    const err = await makeClient('nca_bad')
      .me()
      .then(
        () => null,
        (e: unknown) => e,
      );

    const apiErr = err as ApiError;
    expect(apiErr.unauthorized).toBe(true);
    expect(apiErr.message).toBe('invalid credentials');
  });

  it('falls back to HTTP status text when the body is not the envelope', async () => {
    nextResponse = { status: 500 };

    const err = await makeClient('nca_test')
      .me()
      .then(
        () => null,
        (e: unknown) => e,
      );

    const apiErr = err as ApiError;
    expect(apiErr.status).toBe(500);
    expect(apiErr.code).toBe('unknown');
    expect(apiErr.message).toBe('HTTP 500');
  });

  it('wraps network failure as status 0', async () => {
    const client = createApiClient(() =>
      Promise.resolve({ baseUrl: 'http://127.0.0.1:1', token: 'nca_test' }),
    );

    const err = await client.me().then(
      () => null,
      (e: unknown) => e,
    );

    expect((err as ApiError).status).toBe(0);
  });
});

describe('site rules and token revocation', () => {
  it('creates a rule over the Bearer channel', async () => {
    nextResponse = { status: 201, body: { id: 9, host: 'manhua.example.net' } };
    const rule = await makeClient('nca_test').createSiteRule({
      host: 'manhua.example.net',
      chapter_url_regex:
        '^/manga/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+(?:\\.[0-9]+)?)/?$',
      slug_capture_group: 'slug',
      chapter_capture_group: 'chapter',
    });

    expect(rule.id).toBe(9);
    expect(seen[0]?.method).toBe('POST');
    expect(seen[0]?.url).toBe('/sites/rules');
    expect(seen[0]?.headers.authorization).toBe('Bearer nca_test');
  });

  it('deletes a rule by id', async () => {
    nextResponse = { status: 204 };
    await makeClient('nca_test').deleteSiteRule(9);

    expect(seen[0]?.method).toBe('DELETE');
    expect(seen[0]?.url).toBe('/sites/rules/9');
  });

  it('revokes a token by id', async () => {
    nextResponse = { status: 204 };
    await makeClient('nca_test').revokeToken(6);

    expect(seen[0]?.method).toBe('DELETE');
    expect(seen[0]?.url).toBe('/auth/tokens/6');
    expect(seen[0]?.headers.authorization).toBe('Bearer nca_test');
  });

  it('surfaces the duplicate-host validation envelope', async () => {
    nextResponse = {
      status: 422,
      body: {
        error: {
          code: 'validation',
          message: 'validation failed',
          fields: { host: 'already has a rule' },
        },
      },
    };

    const err = await makeClient('nca_test')
      .createSiteRule({
        host: 'comics.example.org',
        chapter_url_regex: '^/x/(?P<slug>[^/]+)/(?P<chapter>[0-9]+)$',
        slug_capture_group: 'slug',
        chapter_capture_group: 'chapter',
      })
      .then(
        () => null,
        (e: unknown) => e,
      );

    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).fields?.host).toBe('already has a rule');
  });
});

describe('cookie auth mode (the web SPA — ADR-0010)', () => {
  function cookieClient(): ApiClient {
    return createApiClient(() =>
      Promise.resolve({ baseUrl, authMode: 'cookie' as const }),
    );
  }

  it('sends no Authorization header and does not require a token', async () => {
    nextResponse = { status: 200, body: { id: 1, username: 'rishi' } };
    const user = await cookieClient().me();

    expect(user.username).toBe('rishi');
    expect(seen[0]?.headers.authorization).toBeUndefined();
  });

  it('covers data mutations without a token', async () => {
    nextResponse = { status: 200, body: { id: 4, title: 'ORV' } };
    const series = await cookieClient().patchSeries(4, { status: 'completed' });

    expect(series.id).toBe(4);
    expect(seen[0]?.method).toBe('PATCH');
    expect(seen[0]?.url).toBe('/series/4');
    expect(seen[0]?.headers.authorization).toBeUndefined();
  });

  it('bearer mode still requires the token (extension behavior pinned)', async () => {
    await expect(makeClient(undefined).me()).rejects.toMatchObject({
      status: 401,
    });
    expect(seen).toHaveLength(0);
  });
});

describe('web data methods', () => {
  it('gets series detail', async () => {
    nextResponse = { status: 200, body: { id: 7, title: 'ORV', entries: [] } };
    const detail = await makeClient('nca_test').getSeries(7);

    expect(detail.title).toBe('ORV');
    expect(seen[0]?.method).toBe('GET');
    expect(seen[0]?.url).toBe('/series/7');
  });

  it('patches a series', async () => {
    nextResponse = { status: 200, body: { id: 7, rating: 9 } };
    await makeClient('nca_test').patchSeries(7, {
      rating: 9,
      tags: ['action'],
    });

    expect(seen[0]?.method).toBe('PATCH');
    expect(JSON.parse(seen[0]?.body ?? '')).toEqual({
      rating: 9,
      tags: ['action'],
    });
  });

  it('deletes a series', async () => {
    nextResponse = { status: 204 };
    await makeClient('nca_test').deleteSeries(7);
    expect(seen[0]?.method).toBe('DELETE');
    expect(seen[0]?.url).toBe('/series/7');
  });

  it('lists entries with the series filter', async () => {
    nextResponse = { status: 200, body: { items: [], total: 0 } };
    await makeClient('nca_test').listEntries({
      seriesId: 7,
      limit: 50,
      offset: 10,
    });
    expect(seen[0]?.url).toBe('/entries?series_id=7&limit=50&offset=10');
  });

  it('patches an entry (reassignment shape)', async () => {
    nextResponse = { status: 200, body: { id: 3, series_id: 9 } };
    const entry = await makeClient('nca_test').patchEntry(3, { series_id: 9 });

    expect(entry.series_id).toBe(9);
    expect(seen[0]?.method).toBe('PATCH');
    expect(seen[0]?.url).toBe('/entries/3');
    expect(JSON.parse(seen[0]?.body ?? '')).toEqual({ series_id: 9 });
  });

  it('deletes an entry', async () => {
    nextResponse = { status: 204 };
    await makeClient('nca_test').deleteEntry(3);
    expect(seen[0]?.method).toBe('DELETE');
    expect(seen[0]?.url).toBe('/entries/3');
  });

  it('patches a site rule', async () => {
    nextResponse = { status: 200, body: { id: 5, host: 'manhua.example.net' } };
    await makeClient('nca_test').patchSiteRule(5, {
      chapter_url_regex: '^/x/(?P<slug>[^/]+)/(?P<chapter>[0-9]+)$',
    });

    expect(seen[0]?.method).toBe('PATCH');
    expect(seen[0]?.url).toBe('/sites/rules/5');
  });

  it('uploads a cover as raw bytes, not JSON', async () => {
    // Bytes that are not valid UTF-8 — they must survive the round trip
    // untouched, which a JSON.stringify path would mangle.
    const bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x00, 0xff, 0xfe]);
    nextResponse = {
      status: 200,
      body: { series_id: 7, mime: 'image/png', width: 300, height: 450 },
    };

    const meta = await makeClient('nca_test').setSeriesCover(
      7,
      new Blob([bytes], { type: 'image/png' }),
      'https://example.test/series/x',
    );

    expect(meta.width).toBe(300);
    expect(seen[0]?.method).toBe('PUT');
    expect(seen[0]?.url).toBe('/series/7/cover');
    expect(seen[0]?.headers['content-type']).toBe('image/png');
    expect(seen[0]?.headers['x-cover-source-url']).toBe(
      'https://example.test/series/x',
    );
    expect(Uint8Array.from(seen[0]?.raw ?? Buffer.alloc(0))).toEqual(bytes);
  });

  it('omits the source header when no source url is given', async () => {
    nextResponse = { status: 200, body: { series_id: 7 } };
    await makeClient('nca_test').setSeriesCover(
      7,
      new Blob([new Uint8Array([1, 2, 3])]),
    );

    expect(seen[0]?.headers['x-cover-source-url']).toBeUndefined();
    // A typeless Blob still declares something explicit rather than
    // letting fetch guess.
    expect(seen[0]?.headers['content-type']).toBe('application/octet-stream');
  });

  it('deletes a cover', async () => {
    nextResponse = { status: 204 };
    await makeClient('nca_test').deleteSeriesCover(7);
    expect(seen[0]?.method).toBe('DELETE');
    expect(seen[0]?.url).toBe('/series/7/cover');
  });

  it('builds a cover URL with the cache-buster, and without one', async () => {
    const client = makeClient('nca_test');
    expect(await client.seriesCoverUrl(7, '2026-08-28T10:00:00Z')).toBe(
      `${baseUrl}/series/7/cover?v=2026-08-28T10%3A00%3A00Z`,
    );
    // A series with no cover has a null cover_updated_at; that must not
    // produce a "?v=null" URL.
    expect(await client.seriesCoverUrl(7, null)).toBe(
      `${baseUrl}/series/7/cover`,
    );
    expect(await client.seriesCoverUrl(7)).toBe(`${baseUrl}/series/7/cover`);
  });
});
