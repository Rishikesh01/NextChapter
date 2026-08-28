import { useState } from 'react';
import type { SiteRule } from '@nextchapter/api-client';

export interface RulesSectionProps {
  rules: SiteRule[];
  /** Failure of the last delete, shown as the standard red status line. */
  deleteError?: string;
  busy: boolean;
  onDelete: (ruleID: number) => void;
}

/**
 * View + delete only — creation happens in the popup's rule builder and
 * regex-level editing belongs to web/ (ADR-0009 §4). Delete is a two-step
 * inline confirm: Cancel takes Delete's former position and receives focus so
 * a double-click or Enter–Enter cancels rather than destroys.
 */
export function RulesSection({
  rules,
  deleteError,
  busy,
  onDelete,
}: RulesSectionProps) {
  const [confirmId, setConfirmId] = useState<number | null>(null);

  return (
    <section className="nc-section">
      <h2 className="nc-section-title">Site rules</h2>
      {rules.length === 0 ? (
        <p className="nc-rules-empty nc-small">
          No rules yet — create one from the popup while on a chapter page.
        </p>
      ) : (
        <>
          <p className="nc-section-caption nc-small">
            Chapter pages on these sites are detected automatically. Create
            rules from the popup while on a chapter page.
          </p>
          <ul className="nc-rules-list">
            {rules.map((rule) => (
              <li
                className="nc-rule-row"
                key={rule.id}
                onKeyDown={(event) => {
                  if (event.key === 'Escape' && confirmId === rule.id)
                    setConfirmId(null);
                }}
              >
                <div className="nc-rule-info">
                  <p className="nc-rule-host" title={rule.host}>
                    {rule.host}
                  </p>
                  <p className="nc-rule-pattern" title={rule.chapter_url_regex}>
                    {rule.chapter_url_regex}
                  </p>
                </div>
                {confirmId === rule.id ? (
                  <span
                    className="nc-rule-confirm"
                    role="group"
                    aria-label={`Confirm deleting the rule for ${rule.host ?? ''}`}
                  >
                    <span className="nc-rule-confirm-q nc-small">
                      Delete rule?
                    </span>
                    <button
                      className="nc-rule-confirm-yes"
                      type="button"
                      disabled={busy}
                      onClick={() => {
                        setConfirmId(null);
                        if (rule.id !== undefined) onDelete(rule.id);
                      }}
                    >
                      Confirm
                    </button>
                    <button
                      className="nc-rule-confirm-no"
                      type="button"
                      autoFocus
                      onClick={() => {
                        setConfirmId(null);
                      }}
                    >
                      Cancel
                    </button>
                  </span>
                ) : (
                  <button
                    className="nc-rule-delete"
                    type="button"
                    aria-label={`Delete rule for ${rule.host ?? ''}`}
                    onClick={() => {
                      setConfirmId(rule.id ?? null);
                    }}
                  >
                    Delete
                  </button>
                )}
              </li>
            ))}
          </ul>
        </>
      )}
      {deleteError !== undefined && (
        <p className="nc-status nc-status-bad" role="status">
          <span className="nc-status-dot" />
          {deleteError}
        </p>
      )}
    </section>
  );
}
