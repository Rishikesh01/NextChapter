import { useId } from 'react';

export interface ChapterInputProps {
  value: string;
  onChange: (value: string) => void;
  error?: string;
  /** Compact right-aligned variant for the detected card's inline row. */
  compact?: boolean;
  autoFocus?: boolean;
}

/**
 * Chapter numbers can be fractional (45.5), so this is a text input with a
 * decimal keyboard hint, not type=number — the value is validated on submit.
 */
export function ChapterInput({
  value,
  onChange,
  error,
  compact,
  autoFocus,
}: ChapterInputProps) {
  const id = useId();
  const classes = [
    'nc-input',
    compact === true ? 'nc-input-chapter' : 'nc-input-chapter-full',
  ];
  if (error !== undefined) classes.push('is-invalid');

  const input = (
    <input
      className={classes.join(' ')}
      id={id}
      type="text"
      inputMode="decimal"
      autoComplete="off"
      spellCheck={false}
      autoFocus={autoFocus}
      value={value}
      onChange={(event) => {
        onChange(event.target.value);
      }}
      aria-invalid={error !== undefined}
    />
  );

  if (compact === true) {
    return (
      <>
        <div className="nc-chapter-row">
          <label htmlFor={id}>Chapter</label>
          {input}
        </div>
        {error !== undefined && <p className="nc-field-error">{error}</p>}
      </>
    );
  }
  return (
    <div className="nc-field">
      <label htmlFor={id}>Chapter</label>
      {input}
      {error !== undefined && <p className="nc-field-error">{error}</p>}
    </div>
  );
}

/** Submit-time validation shared by both capture forms. */
export function parseChapterInput(value: string): number | null {
  const trimmed = value.trim();
  if (trimmed === '' || !/^\d+(?:\.\d+)?$/.test(trimmed)) return null;
  const parsed = Number.parseFloat(trimmed);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
}
