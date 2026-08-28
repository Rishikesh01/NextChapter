import { useState } from 'react';
import type { Entry } from '@nextchapter/api-client';
import { relativeTime } from '../lib/format';
import { ConfirmInline } from './ConfirmInline';

export interface EntryRowProps {
  entry: Entry;
  busy: boolean;
  onMove: () => void;
  onSave: (patch: { last_chapter?: number; last_url?: string }) => void;
  onRemove: () => void;
}

/**
 * One per-site entry: Continue reading (the row's primary action, opens
 * last_url in a new tab), then quiet Move / Edit / Remove. Edit opens an
 * inline correction row (chapter accepts decimals); Remove is the two-step
 * confirm.
 */
export function EntryRow({
  entry,
  busy,
  onMove,
  onSave,
  onRemove,
}: EntryRowProps) {
  const [editing, setEditing] = useState(false);
  const [chapter, setChapter] = useState('');
  const [url, setUrl] = useState('');
  const [chapterError, setChapterError] = useState<string>();

  const openEdit = () => {
    setChapter(String(entry.last_chapter ?? ''));
    setUrl(entry.last_url ?? '');
    setChapterError(undefined);
    setEditing(true);
  };

  const save = () => {
    const trimmed = chapter.trim();
    if (!/^\d+(?:\.\d+)?$/.test(trimmed)) {
      setChapterError('Enter a chapter number like 101 or 45.5');
      return;
    }
    setEditing(false);
    onSave({ last_chapter: Number.parseFloat(trimmed), last_url: url.trim() });
  };

  return (
    <>
      <tr>
        <td className="nc-td-host">{entry.site_host}</td>
        <td className="nc-td-title" title={entry.site_title}>
          {entry.site_title}
        </td>
        <td className="nc-td-ch">at ch {entry.last_chapter}</td>
        <td className="nc-td-when">{relativeTime(entry.last_captured_at)}</td>
        <td className="nc-td-actions">
          <a
            className="nc-btn-continue"
            href={entry.last_url}
            target="_blank"
            rel="noopener noreferrer"
          >
            Continue reading ↗
          </a>
          <button
            className="nc-row-action"
            type="button"
            disabled={busy}
            onClick={onMove}
          >
            Move
          </button>
          <button
            className="nc-row-action"
            type="button"
            disabled={busy}
            onClick={openEdit}
          >
            Edit
          </button>
          <ConfirmInline
            label="Remove"
            question="Remove entry?"
            busy={busy}
            onConfirm={onRemove}
          />
        </td>
      </tr>
      {editing && (
        <tr className="nc-entry-editrow">
          <td colSpan={5}>
            <div className="nc-entry-editform">
              <div className="nc-fieldlet">
                <label htmlFor={`chapter-${String(entry.id ?? 0)}`}>
                  Chapter
                </label>
                <input
                  className={`nc-input nc-input-chapter${chapterError !== undefined ? ' is-invalid' : ''}`}
                  id={`chapter-${String(entry.id ?? 0)}`}
                  type="text"
                  inputMode="decimal"
                  autoComplete="off"
                  spellCheck={false}
                  value={chapter}
                  aria-invalid={chapterError !== undefined}
                  onChange={(event) => {
                    setChapter(event.target.value);
                  }}
                />
              </div>
              <div className="nc-fieldlet nc-fieldlet-url">
                <label htmlFor={`url-${String(entry.id ?? 0)}`}>Last URL</label>
                <input
                  className="nc-input nc-input-url"
                  id={`url-${String(entry.id ?? 0)}`}
                  type="url"
                  autoComplete="off"
                  spellCheck={false}
                  value={url}
                  onChange={(event) => {
                    setUrl(event.target.value);
                  }}
                />
              </div>
              <button
                className="nc-btn-primary"
                type="button"
                disabled={busy}
                onClick={save}
              >
                Save
              </button>
              <button
                className="nc-btn-secondary"
                type="button"
                onClick={() => {
                  setEditing(false);
                }}
              >
                Cancel
              </button>
            </div>
            {chapterError !== undefined && (
              <p className="nc-field-error">{chapterError}</p>
            )}
          </td>
        </tr>
      )}
    </>
  );
}
