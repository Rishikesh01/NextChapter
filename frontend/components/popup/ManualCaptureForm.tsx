import { useId } from 'react';
import { ChapterInput } from './ChapterInput';

export interface ManualCaptureFormProps {
  slug: string;
  chapter: string;
  slugError?: string;
  chapterError?: string;
  busy: boolean;
  onSlugChange: (value: string) => void;
  onChapterChange: (value: string) => void;
  onCapture: () => void;
}

/** The "manual" state: no site rule matched, the user fills in slug + chapter. */
export function ManualCaptureForm({
  slug,
  chapter,
  slugError,
  chapterError,
  busy,
  onSlugChange,
  onChapterChange,
  onCapture,
}: ManualCaptureFormProps) {
  const slugId = useId();
  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    onCapture();
  };

  return (
    <form className="nc-card" onSubmit={submit}>
      <p className="nc-form-intro">
        No rule for this site — fill in the details
      </p>

      <div className="nc-field">
        <label htmlFor={slugId}>Series slug</label>
        <input
          className={`nc-input nc-input-slug${slugError !== undefined ? ' is-invalid' : ''}`}
          id={slugId}
          type="text"
          placeholder="solo-leveling"
          autoComplete="off"
          spellCheck={false}
          autoCapitalize="off"
          value={slug}
          onChange={(event) => {
            onSlugChange(event.target.value);
          }}
          aria-invalid={slugError !== undefined}
        />
        {slugError !== undefined && (
          <p className="nc-field-error">{slugError}</p>
        )}
      </div>

      <ChapterInput
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
      <p className="nc-form-hint nc-small">
        Add a URL rule for this site in your web library to auto-detect next
        time.
      </p>
    </form>
  );
}
