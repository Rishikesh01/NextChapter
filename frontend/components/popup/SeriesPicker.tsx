import { useState } from 'react';
import type { SeriesSummary } from '@nextchapter/api-client';

export interface SeriesPickerProps {
  /** Slug + host shown as context under the title. */
  seriesSlug: string;
  siteHost: string;
  /** Prefill for the pinned "create new series" row. */
  suggestedTitle: string;
  series: SeriesSummary[];
  loading: boolean;
  busy: boolean;
  onPick: (series: SeriesSummary) => void;
  onCreate: (title: string) => void;
}

/**
 * Shown after a capture came back 422 needs-series: assign the new
 * (host, slug) pair to an existing series or create a new one. One click
 * completes the capture. The create row is pinned first and doubles as the
 * zero-series empty state.
 */
export function SeriesPicker({
  seriesSlug,
  siteHost,
  suggestedTitle,
  series,
  loading,
  busy,
  onPick,
  onCreate,
}: SeriesPickerProps) {
  const [filter, setFilter] = useState('');

  const query = filter.trim().toLowerCase();
  const visible =
    query === ''
      ? series
      : series.filter((item) =>
          (item.title ?? '').toLowerCase().includes(query),
        );
  const createTitle = filter.trim() !== '' ? filter.trim() : suggestedTitle;

  return (
    <div>
      <div className="nc-picker-header">
        <h1 className="nc-picker-title">Which series is this?</h1>
        <p className="nc-picker-context nc-small">
          <code>{seriesSlug}</code> on {siteHost}
        </p>
      </div>
      <div className="nc-picker-search">
        <input
          className="nc-input"
          type="search"
          placeholder="Filter your series"
          aria-label="Filter your series"
          autoComplete="off"
          spellCheck={false}
          value={filter}
          onChange={(event) => {
            setFilter(event.target.value);
          }}
        />
      </div>
      {loading ? (
        <div aria-hidden="true">
          <div className="nc-skeleton-row">
            <div className="nc-skeleton-bar" />
            <div className="nc-skeleton-bar" />
          </div>
          <div className="nc-skeleton-row">
            <div className="nc-skeleton-bar" />
            <div className="nc-skeleton-bar" />
          </div>
          <div className="nc-skeleton-row">
            <div className="nc-skeleton-bar" />
            <div className="nc-skeleton-bar" />
          </div>
        </div>
      ) : (
        <ul className="nc-series-list" aria-label="Choose a series">
          <li>
            <button
              className="nc-series-row nc-series-row-create"
              type="button"
              disabled={busy}
              onClick={() => {
                onCreate(createTitle);
              }}
            >
              <span className="nc-series-row-title">
                Create new series: “{createTitle}”
              </span>
            </button>
          </li>
          {visible.map((item) => (
            <li key={item.id}>
              <button
                className="nc-series-row"
                type="button"
                disabled={busy}
                onClick={() => {
                  onPick(item);
                }}
              >
                <span className="nc-series-row-title">{item.title}</span>
                <span className="nc-series-row-meta">
                  {item.highest_chapter !== undefined
                    ? `read till ch ${String(item.highest_chapter)} · `
                    : ''}
                  {String(item.entry_count ?? 0)}{' '}
                  {(item.entry_count ?? 0) === 1 ? 'site' : 'sites'}
                </span>
              </button>
            </li>
          ))}
          {visible.length === 0 && query !== '' && (
            <li>
              <p className="nc-list-empty nc-small">
                No series match “{filter.trim()}”
              </p>
            </li>
          )}
        </ul>
      )}
    </div>
  );
}
