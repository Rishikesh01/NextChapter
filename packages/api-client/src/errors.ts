import type { ErrorBody } from './types';

/** A non-2xx response from the NextChapter API, carrying the server's error envelope. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields?: Record<string, string>;

  constructor(
    status: number,
    body: ErrorBody | undefined,
    fallbackMessage: string,
  ) {
    const detail = body?.error;
    super(detail?.message ?? fallbackMessage);
    this.name = 'ApiError';
    this.status = status;
    this.code = detail?.code ?? 'unknown';
    this.fields = detail?.fields;
  }

  /**
   * True when POST /entries/capture rejected because no entry exists yet for
   * the (site_host, series_slug) key and no series binding was supplied — the
   * signal to show the series picker and re-capture with series_id or
   * new_series_title.
   */
  get needsSeriesBinding(): boolean {
    return this.status === 422 && this.fields?.series_id !== undefined;
  }

  get unauthorized(): boolean {
    return this.status === 401;
  }
}
