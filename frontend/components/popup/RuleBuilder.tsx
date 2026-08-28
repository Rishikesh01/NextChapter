import type { DetectedPosition } from '../../lib/url-detection';
import type { RuleDraft } from '../../lib/rule-builder';

export interface RuleBuilderProps {
  segments: string[];
  draft: RuleDraft;
  /** Applying the drafted rule to the current URL; null = invalid selection. */
  preview: DetectedPosition | null;
  busy: boolean;
  onDraftChange: (draft: RuleDraft) => void;
  onSave: () => void;
  onBack: () => void;
}

/**
 * The no-regex rule builder: the page URL's path segments as rows, two radio
 * columns (Series / Chapter), a live preview of what the drafted rule
 * detects. "Save rule & capture" IS the capture (ADR-0009 §3).
 */
export function RuleBuilder({
  segments,
  draft,
  preview,
  busy,
  onDraftChange,
  onSave,
  onBack,
}: RuleBuilderProps) {
  const invalidMessage =
    draft.slugIndex === draft.chapterIndex
      ? 'The series and chapter must be different parts.'
      : preview === null
        ? `“${segments[draft.chapterIndex] ?? ''}” has no number — pick the part that contains the chapter.`
        : undefined;

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    if (invalidMessage === undefined) onSave();
  };

  return (
    <form className="nc-card" onSubmit={submit}>
      <p className="nc-form-intro">Create a rule for this site</p>
      <p className="nc-rule-caption nc-small">
        Mark which parts of the page address are the series name and the
        chapter.
      </p>

      <div className="nc-rulegrid" role="group" aria-label="Address parts">
        <div className="nc-rulegrid-head" aria-hidden="true">
          <span className="nc-rulegrid-seg">Address part</span>
          <span className="nc-rulegrid-col">Series</span>
          <span className="nc-rulegrid-col">Chapter</span>
        </div>
        {segments.map((segment, index) => (
          <div className="nc-rulegrid-row" key={`${String(index)}-${segment}`}>
            <code className="nc-rulegrid-seg" title={segment}>
              {segment}
            </code>
            <span className="nc-rulegrid-col">
              <input
                type="radio"
                name="series-part"
                checked={draft.slugIndex === index}
                aria-label={`Series name part: ${segment}`}
                onChange={() => {
                  onDraftChange({ ...draft, slugIndex: index });
                }}
              />
            </span>
            <span className="nc-rulegrid-col">
              <input
                type="radio"
                name="chapter-part"
                checked={draft.chapterIndex === index}
                aria-label={`Chapter part: ${segment}`}
                onChange={() => {
                  onDraftChange({ ...draft, chapterIndex: index });
                }}
              />
            </span>
          </div>
        ))}
      </div>

      {invalidMessage !== undefined ? (
        <p
          className="nc-rule-preview nc-rule-preview-invalid"
          aria-live="polite"
        >
          {invalidMessage}
        </p>
      ) : (
        <p className="nc-rule-preview" aria-live="polite">
          <span>Will detect:</span>
          <code className="nc-rule-preview-slug" title={preview?.seriesSlug}>
            {preview?.seriesSlug}
          </code>
          <span>
            · ch <strong>{String(preview?.chapter ?? '')}</strong>
          </span>
        </p>
      )}

      <button
        className="nc-btn-primary nc-btn-capture"
        type="submit"
        disabled={busy || invalidMessage !== undefined}
      >
        {busy ? 'Saving…' : 'Save rule & capture'}
      </button>
      <p className="nc-rule-back">
        <button className="nc-btn-link" type="button" onClick={onBack}>
          Back to manual entry
        </button>
      </p>
    </form>
  );
}
