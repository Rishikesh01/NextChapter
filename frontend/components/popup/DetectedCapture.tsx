import { ChapterInput } from './ChapterInput';

export interface DetectedCaptureProps {
  seriesTitle: string;
  chapter: string;
  chapterError?: string;
  busy: boolean;
  onChapterChange: (value: string) => void;
  onCapture: () => void;
}

/** The "detected" state: a site rule matched the page URL. */
export function DetectedCapture({
  seriesTitle,
  chapter,
  chapterError,
  busy,
  onChapterChange,
  onCapture,
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
    </form>
  );
}
