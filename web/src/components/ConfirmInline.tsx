import { useState } from 'react';

export interface ConfirmInlineProps {
  /** The quiet trigger label, e.g. "Remove" / "Delete series". */
  label: string;
  question: string;
  busy?: boolean;
  danger?: boolean;
  /** Renders the trigger as a secondary button instead of a text action. */
  asButton?: boolean;
  onConfirm: () => void;
}

/**
 * The two-step inline confirm idiom: Cancel takes the destructive position
 * (rightmost) and receives focus so a double-click or Enter–Enter can never
 * destroy data; Escape cancels.
 */
export function ConfirmInline({
  label,
  question,
  busy,
  danger = true,
  asButton,
  onConfirm,
}: ConfirmInlineProps) {
  const [armed, setArmed] = useState(false);

  if (!armed) {
    return (
      <button
        className={
          asButton === true
            ? `nc-btn-secondary${danger ? ' nc-btn-danger-quiet' : ''}`
            : `nc-row-action${danger ? ' nc-row-action-danger' : ''}`
        }
        type="button"
        onClick={() => {
          setArmed(true);
        }}
      >
        {label}
      </button>
    );
  }
  return (
    <span
      className="nc-confirm"
      role="group"
      aria-label={question}
      onKeyDown={(event) => {
        if (event.key === 'Escape') setArmed(false);
      }}
    >
      <span className="nc-confirm-q">{question}</span>
      <button
        className="nc-confirm-yes"
        type="button"
        disabled={busy}
        onClick={() => {
          setArmed(false);
          onConfirm();
        }}
      >
        Confirm
      </button>
      <button
        className="nc-confirm-no"
        type="button"
        autoFocus
        onClick={() => {
          setArmed(false);
        }}
      >
        Cancel
      </button>
    </span>
  );
}
