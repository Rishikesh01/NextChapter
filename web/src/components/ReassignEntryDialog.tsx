import { useState } from 'react';
import type { Entry, SeriesSummary } from '@nextchapter/api-client';

export interface ReassignEntryDialogProps {
  entry: Entry;
  /** Every series except the entry's current one. */
  series: SeriesSummary[];
  busy: boolean;
  onPick: (seriesID: number) => void;
  onCreate: (title: string) => void;
  onClose: () => void;
}

/**
 * "Move this entry to…" — the same picker idiom as the extension's series
 * picker: filter input, rows with rollups, a pinned create-new row prefilled
 * from the filter text (or the entry's site title).
 */
export function ReassignEntryDialog({
  entry,
  series,
  busy,
  onPick,
  onCreate,
  onClose,
}: ReassignEntryDialogProps) {
  const [filter, setFilter] = useState('');

  const query = filter.trim().toLowerCase();
  const visible =
    query === ''
      ? series
      : series.filter((item) =>
          (item.title ?? '').toLowerCase().includes(query),
        );
  const createTitle =
    filter.trim() !== '' ? filter.trim() : (entry.site_title ?? '');

  return (
    <div
      className="nc-overlay"
      role="presentation"
      onKeyDown={(event) => {
        if (event.key === 'Escape') onClose();
      }}
    >
      <div
        className="nc-dialog"
        role="dialog"
        aria-label="Move this entry"
        aria-modal="true"
      >
        <h2 className="nc-dialog-title">Move this entry to…</h2>
        <p className="nc-dialog-context nc-small">
          <code>{entry.series_slug}</code> on {entry.site_host}
        </p>
        <input
          className="nc-input nc-dialog-search"
          type="search"
          placeholder="Filter your series"
          aria-label="Filter your series"
          autoComplete="off"
          spellCheck={false}
          autoFocus
          value={filter}
          onChange={(event) => {
            setFilter(event.target.value);
          }}
        />
        <ul className="nc-picker-list" aria-label="Choose a series">
          {createTitle !== '' && (
            <li>
              <button
                className="nc-picker-row nc-picker-row-create"
                type="button"
                disabled={busy}
                onClick={() => {
                  onCreate(createTitle);
                }}
              >
                <span className="nc-picker-row-title">
                  Create new series: “{createTitle}”
                </span>
              </button>
            </li>
          )}
          {visible.map((item) => (
            <li key={item.id}>
              <button
                className="nc-picker-row"
                type="button"
                disabled={busy}
                onClick={() => {
                  if (item.id !== undefined) onPick(item.id);
                }}
              >
                <span className="nc-picker-row-title">{item.title}</span>
                <span className="nc-picker-row-meta">
                  {item.highest_chapter !== undefined
                    ? `read till ch ${String(item.highest_chapter)} · `
                    : ''}
                  {String(item.entry_count ?? 0)}{' '}
                  {(item.entry_count ?? 0) === 1 ? 'site' : 'sites'}
                </span>
              </button>
            </li>
          ))}
        </ul>
        <p className="nc-dialog-note nc-small">
          The chapter rollups of both series update.
        </p>
        <div className="nc-dialog-actions">
          <button className="nc-btn-secondary" type="button" onClick={onClose}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
