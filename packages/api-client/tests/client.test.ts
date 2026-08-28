import { createServer, type IncomingMessage, type Server } from 'node:http';
import { afterAll, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { ApiError, createApiClient, type ApiClient } from '../src/index';

interface SeenRequest {
  method: string;
  url: string;
  headers: IncomingMessage['headers'];
  body: string;
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
    let body = '';
    req.on('data', (chunk: Buffer) => (body += chunk.toString()));
    req.on('end', () => {
      seen.push({
        method: req.method ?? '',
        url: req.url ?? '',
        headers: req.headers,
        body,
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
});
