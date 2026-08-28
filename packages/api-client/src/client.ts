import { ApiError } from './errors';
import type {
  APIToken,
  Credentials,
  Entry,
  EntryCapture,
  ErrorBody,
  Health,
  NewToken,
  Registration,
  Series,
  SeriesList,
  SeriesNew,
  SeriesStatus,
  SiteList,
  SiteRule,
  SiteRuleNew,
  User,
} from './types';

export interface ApiConfig {
  /** Server origin, e.g. "https://nextchapter.example.org". */
  baseUrl: string;
  /** API token (nca_…). Absent until onboarding completes. */
  token?: string;
}

/**
 * Supplies the current server configuration for every request. The extension
 * backs this with `browser.storage`; tests and the future web SPA supply
 * their own. The client itself never touches storage and never hardcodes a
 * server URL.
 */
export type ConfigProvider = () => Promise<ApiConfig>;

export interface CaptureResult {
  entry: Entry;
  /** True on HTTP 201 (first capture for this site+slug), false on 200 (advanced). */
  created: boolean;
}

export interface ListSeriesQuery {
  status?: SeriesStatus;
  tag?: string[];
  limit?: number;
  offset?: number;
}

export interface ApiClient {
  /** Unauthenticated. */
  health(): Promise<Health>;

  // Bearer channel: Authorization header, credentials omitted so a stray
  // nc_session cookie can never shadow the token (the server reads the
  // cookie first — ADR-0008 §6).
  me(): Promise<User>;
  listSeries(query?: ListSeriesQuery): Promise<SeriesList>;
  createSeries(body: SeriesNew): Promise<Series>;
  capture(body: EntryCapture): Promise<CaptureResult>;
  getSites(): Promise<SiteList>;
  createSiteRule(body: SiteRuleNew): Promise<SiteRule>;
  deleteSiteRule(ruleID: number): Promise<void>;
  /** Revoke an API token by id — used by Disconnect on its own token (ADR-0009). */
  revokeToken(tokenID: number): Promise<void>;

  /** Cookie channel — onboarding only. credentials:"include", no Bearer header. */
  auth: {
    /** Sets the nc_session cookie on success. */
    register(body: Registration): Promise<User>;
    /** Sets the nc_session cookie on success. */
    login(body: Credentials): Promise<User>;
    /** Cookie-authenticated; the plaintext token is in `token`, returned exactly once. */
    mintToken(body: NewToken): Promise<APIToken>;
    /** Best-effort session cleanup after minting. */
    logout(): Promise<void>;
  };
}

type Channel = 'none' | 'bearer' | 'cookie';

interface RequestOptions {
  method: string;
  path: string;
  channel: Channel;
  body?: unknown;
  query?: URLSearchParams;
}

export function createApiClient(getConfig: ConfigProvider): ApiClient {
  async function request(opts: RequestOptions): Promise<Response> {
    const config = await getConfig();
    const base = config.baseUrl.replace(/\/+$/, '');
    if (base === '') {
      throw new ApiError(0, undefined, 'no server URL configured');
    }

    const headers = new Headers();
    if (opts.body !== undefined) {
      headers.set('Content-Type', 'application/json');
    }
    if (opts.channel === 'bearer') {
      if (config.token === undefined || config.token === '') {
        throw new ApiError(401, undefined, 'no API token configured');
      }
      headers.set('Authorization', `Bearer ${config.token}`);
    }

    const search = opts.query?.size ? `?${opts.query.toString()}` : '';
    let response: Response;
    try {
      response = await fetch(`${base}${opts.path}${search}`, {
        method: opts.method,
        headers,
        credentials: opts.channel === 'cookie' ? 'include' : 'omit',
        body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
      });
    } catch (cause) {
      throw new ApiError(
        0,
        undefined,
        `cannot reach ${base}: ${String(cause)}`,
      );
    }

    if (!response.ok) {
      let errorBody: ErrorBody | undefined;
      try {
        errorBody = (await response.json()) as ErrorBody;
      } catch {
        errorBody = undefined;
      }
      throw new ApiError(
        response.status,
        errorBody,
        `HTTP ${String(response.status)}`,
      );
    }
    return response;
  }

  async function json<T>(opts: RequestOptions): Promise<T> {
    const response = await request(opts);
    return (await response.json()) as T;
  }

  return {
    health: () =>
      json<Health>({ method: 'GET', path: '/healthz', channel: 'none' }),

    me: () =>
      json<User>({ method: 'GET', path: '/auth/me', channel: 'bearer' }),

    listSeries: (query) => {
      const params = new URLSearchParams();
      if (query?.status !== undefined) params.set('status', query.status);
      for (const tag of query?.tag ?? []) params.append('tag', tag);
      if (query?.limit !== undefined) params.set('limit', String(query.limit));
      if (query?.offset !== undefined)
        params.set('offset', String(query.offset));
      return json<SeriesList>({
        method: 'GET',
        path: '/series',
        channel: 'bearer',
        query: params,
      });
    },

    createSeries: (body) =>
      json<Series>({
        method: 'POST',
        path: '/series',
        channel: 'bearer',
        body,
      }),

    capture: async (body) => {
      const response = await request({
        method: 'POST',
        path: '/entries/capture',
        channel: 'bearer',
        body,
      });
      const entry = (await response.json()) as Entry;
      return { entry, created: response.status === 201 };
    },

    getSites: () =>
      json<SiteList>({ method: 'GET', path: '/sites', channel: 'bearer' }),

    createSiteRule: (body) =>
      json<SiteRule>({
        method: 'POST',
        path: '/sites/rules',
        channel: 'bearer',
        body,
      }),

    deleteSiteRule: async (ruleID) => {
      await request({
        method: 'DELETE',
        path: `/sites/rules/${String(ruleID)}`,
        channel: 'bearer',
      });
    },

    revokeToken: async (tokenID) => {
      await request({
        method: 'DELETE',
        path: `/auth/tokens/${String(tokenID)}`,
        channel: 'bearer',
      });
    },

    auth: {
      register: (body) =>
        json<User>({
          method: 'POST',
          path: '/auth/register',
          channel: 'cookie',
          body,
        }),
      login: (body) =>
        json<User>({
          method: 'POST',
          path: '/auth/login',
          channel: 'cookie',
          body,
        }),
      mintToken: (body) =>
        json<APIToken>({
          method: 'POST',
          path: '/auth/tokens',
          channel: 'cookie',
          body,
        }),
      logout: async () => {
        await request({
          method: 'POST',
          path: '/auth/logout',
          channel: 'cookie',
        });
      },
    },
  };
}
