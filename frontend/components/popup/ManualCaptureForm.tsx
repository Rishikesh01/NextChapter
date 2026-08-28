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
  /** Opens the inline rule builder; absent when a rule can't be built here (ADR-0009). */
  onCreateRule?: () => void;
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
  onCreateRule,
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
      {onCreateRule !== undefined && (
        <>
          <p className="nc-form-hint nc-small">
            A rule lets NextChapter detect chapters here automatically.
          </p>
          <p className="nc-rule-entry">
            <button
              className="nc-btn-link"
              type="button"
              aria-expanded="false"
              onClick={onCreateRule}
            >
              Create a rule from this page
            </button>
          </p>
        </>
      )}
    </form>
  );
}
