import { ChapterInput } from './ChapterInput';

export interface DetectedCaptureProps {
  seriesTitle: string;
  chapter: string;
  chapterError?: string;
  busy: boolean;
  onChapterChange: (value: string) => void;
  onCapture: () => void;
  /** The host this page is on, named in the auto-track toggle. */
  host: string;
  /** null while the permission state is still being read. */
  autoTrack: boolean | null;
  onToggleAutoTrack: (next: boolean) => void;
}

/** The "detected" state: a site rule matched the page URL. */
export function DetectedCapture({
  seriesTitle,
  chapter,
  chapterError,
  busy,
  onChapterChange,
  onCapture,
  host,
  autoTrack,
  onToggleAutoTrack,
}: DetectedCaptureProps) {
  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    onCapture();
  };

  return (
    <form className="nc-card" onSubmit={submit}>
      <h1 className="nc-series-title">{seriesTitle}</h1>
      <p className="nc-series-note nc-small">
        Detected from the page URL — edit if wrong.
      </p>
      <ChapterInput
        compact
        autoFocus
        value={chapter}
        onChange={onChapterChange}
        error={chapterError}
      />
      <button
        className="nc-btn-primary nc-btn-capture"
        type="submit"
        disabled={busy}
      >
        {busy ? 'Capturing…' : 'Capture chapter'}
      </button>

      {/* The permission prompt this opens requires a user gesture, so the
          toggle has to be a real click here — it cannot be flipped on
          from the options page or on the user's behalf. */}
      {autoTrack !== null && (
        <div className="nc-autotrack">
          <input
            id="nc-autotrack-toggle"
            type="checkbox"
            checked={autoTrack}
            disabled={busy}
            aria-describedby="nc-autotrack-hint"
            onChange={(event) => {
              onToggleAutoTrack(event.target.checked);
            }}
          />
          <div className="nc-autotrack-text">
            {/* The consequence line is a DESCRIPTION, not part of the
                accessible name: naming the control "…Chapters save
                themselves…" would make it answer to every by-label query
                looking for the chapter field. */}
            <label className="nc-small" htmlFor="nc-autotrack-toggle">
              Auto-track {host}
            </label>
            <span className="nc-autotrack-hint" id="nc-autotrack-hint">
              {autoTrack
                ? 'Chapters here save themselves after a few seconds.'
                : 'Save chapters here without clicking. Asks for access to this site.'}
            </span>
          </div>
        </div>
      )}
    </form>
  );
}
