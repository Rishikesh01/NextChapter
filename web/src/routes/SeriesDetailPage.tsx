import { useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import type { Entry, SeriesPatch } from '@nextchapter/api-client';
import {
  useCreateSeries,
  useDeleteEntry,
  useDeleteSeries,
  usePatchEntry,
  usePatchSeries,
  useSeriesDetail,
  useSeriesList,
} from '../lib/queries';
import { STATUS_OPTIONS } from '../lib/format';
import { EntryRow } from '../components/EntryRow';
import { ReassignEntryDialog } from '../components/ReassignEntryDialog';
import { ConfirmInline } from '../components/ConfirmInline';
import { ErrorBanner } from '../components/ErrorBanner';
import { TagEditor } from '../components/TagEditor';

export function SeriesDetailPage() {
  const params = useParams();
  const seriesID = Number(params.id);
  const navigate = useNavigate();

  const detail = useSeriesDetail(seriesID);
  const patchSeries = usePatchSeries(seriesID);
  const deleteSeries = useDeleteSeries();
  const patchEntry = usePatchEntry();
  const deleteEntry = useDeleteEntry();
  const createSeries = useCreateSeries();

  const [notes, setNotes] = useState<string>();
  // Editors row and the notes card each get their own quiet save hint.
  const [saveHint, setSaveHint] = useState('');
  const [notesHint, setNotesHint] = useState('');
  const hintTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [moving, setMoving] = useState<Entry | null>(null);
  const [showRating, setShowRating] = useState(false);

  // The reassign dialog needs the rest of the library; fetched when opened.
  const allSeries = useSeriesList({ limit: 200 });

  useEffect(
    () => () => {
      clearTimeout(hintTimer.current);
    },
    [],
  );

  const patch = (body: SeriesPatch, setHint = setSaveHint) => {
    setHint('Saving…');
    patchSeries.mutate(body, {
      onSuccess: () => {
        setHint('Saved');
        clearTimeout(hintTimer.current);
        hintTimer.current = setTimeout(() => {
          setHint('');
        }, 2000);
      },
      onError: () => {
        setHint('');
      },
    });
  };

  if (detail.isPending) return null;
  if (detail.isError) {
    return (
      <>
        <Link className="nc-breadcrumb" to="/">
          ← Library
        </Link>
        <ErrorBanner>{detail.error.message}</ErrorBanner>
      </>
    );
  }

  const series = detail.data;
  const entries = series.entries ?? [];
  const mutationError =
    patchSeries.error ?? patchEntry.error ?? deleteEntry.error;
  const entryBusy =
    patchEntry.isPending || deleteEntry.isPending || createSeries.isPending;

  const reassign = (entry: Entry, targetSeriesID: number) => {
    if (entry.id === undefined) return;
    patchEntry.mutate(
      { entryID: entry.id, patch: { series_id: targetSeriesID } },
      {
        onSuccess: () => {
          setMoving(null);
        },
      },
    );
  };

  return (
    <>
      <Link className="nc-breadcrumb" to="/">
        ← Library
      </Link>
      <h1 className="nc-series-title">{series.title}</h1>

      {mutationError !== null && (
        <ErrorBanner>{mutationError.message}</ErrorBanner>
      )}

      <div className="nc-editors">
        <select
          className="nc-input"
          aria-label="Status"
          value={series.status ?? 'reading'}
          onChange={(event) => {
            patch({ status: event.target.value as SeriesPatch['status'] });
          }}
        >
          {STATUS_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        {(series.rating ?? 0) > 0 || showRating ? (
          <select
            className="nc-input nc-rating-select"
            aria-label="Rating"
            value={series.rating ?? ''}
            onChange={(event) => {
              patch({ rating: Number(event.target.value) });
            }}
          >
            {(series.rating ?? 0) === 0 && <option value="">–</option>}
            {Array.from({ length: 10 }, (_, index) => index + 1).map(
              (value) => (
                <option key={value} value={value}>
                  ★ {value}
                </option>
              ),
            )}
          </select>
        ) : (
          <button
            className="nc-btn-link"
            type="button"
            onClick={() => {
              setShowRating(true);
            }}
          >
            Rate
          </button>
        )}
        <span className="nc-savehint" aria-live="polite">
          {saveHint}
        </span>
      </div>

      <TagEditor
        tags={series.tags ?? []}
        busy={patchSeries.isPending}
        onChange={(tags) => {
          patch({ tags });
        }}
      />

      <div className="nc-notes">
        <label htmlFor="notes">Notes</label>
        <textarea
          className="nc-textarea"
          id="notes"
          value={notes ?? series.notes ?? ''}
          onChange={(event) => {
            setNotes(event.target.value);
          }}
        />
        <div className="nc-notes-actions">
          <span className="nc-savehint" aria-live="polite">
            {notesHint}
          </span>
          <button
            className="nc-btn-primary"
            type="button"
            disabled={patchSeries.isPending || notes === undefined}
            onClick={() => {
              if (notes !== undefined) patch({ notes }, setNotesHint);
            }}
          >
            Save notes
          </button>
        </div>
      </div>

      <section className="nc-section">
        <h2 className="nc-section-title">
          Sites
          {series.highest_chapter !== undefined
            ? ` · read till ch ${String(series.highest_chapter)}`
            : ''}
        </h2>
        {entries.length === 0 ? (
          <p className="nc-entries-empty nc-small">
            No sites yet for this series.
          </p>
        ) : (
          <table className="nc-table">
            <thead>
              <tr>
                <th scope="col">Site</th>
                <th scope="col">Page title</th>
                <th scope="col">Chapter</th>
                <th scope="col">Captured</th>
                <th scope="col" aria-label="Actions"></th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => (
                <EntryRow
                  key={entry.id}
                  entry={entry}
                  busy={entryBusy}
                  onMove={() => {
                    setMoving(entry);
                  }}
                  onSave={(entryPatch) => {
                    if (entry.id !== undefined) {
                      patchEntry.mutate({
                        entryID: entry.id,
                        patch: entryPatch,
                      });
                    }
                  }}
                  onRemove={() => {
                    if (entry.id !== undefined) deleteEntry.mutate(entry.id);
                  }}
                />
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="nc-section">
        <h2 className="nc-section-title">Danger zone</h2>
        <div className="nc-danger-row">
          <p className="nc-small">
            Delete this series and its {entries.length}{' '}
            {entries.length === 1 ? 'site entry' : 'site entries'}. This
            can&rsquo;t be undone.
          </p>
          <ConfirmInline
            label="Delete series"
            question="Delete series?"
            asButton
            busy={deleteSeries.isPending}
            onConfirm={() => {
              deleteSeries.mutate(seriesID, {
                onSuccess: () => {
                  void navigate('/');
                },
              });
            }}
          />
        </div>
      </section>

      {moving !== null && (
        <ReassignEntryDialog
          entry={moving}
          series={(allSeries.data?.items ?? []).filter(
            (item) => item.id !== seriesID,
          )}
          busy={entryBusy}
          onPick={(targetSeriesID) => {
            reassign(moving, targetSeriesID);
          }}
          onCreate={(title) => {
            // Non-atomic by API design (ADR-0010 §8): create the series, then
            // move the entry onto it. The dialog stays open on failure so the
            // user can retry the move alone.
            createSeries.mutate(
              { title },
              {
                onSuccess: (created) => {
                  if (created.id !== undefined) reassign(moving, created.id);
                },
              },
            );
          }}
          onClose={() => {
            setMoving(null);
          }}
        />
      )}
    </>
  );
}
