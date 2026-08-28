import { useState } from 'react';
import type {
  SiteRule,
  SiteRuleNew,
  SiteRulePatch,
} from '@nextchapter/api-client';

export interface RuleFormProps {
  /** Prefill for edit mode; absent = create. */
  rule?: SiteRule;
  /** Host prefill for the "Add one" hint rows (create mode only). */
  initialHost?: string;
  /** Server-side 422 field errors, keyed like the API envelope. */
  fieldErrors?: Record<string, string>;
  busy: boolean;
  /** Create mode emits the full body; edit mode emits only changed fields. */
  onSubmit: (body: SiteRuleNew | SiteRulePatch) => void;
  onCancel: () => void;
}

/**
 * The rule create/edit form — regex-level editing is deliberate here (the
 * web half of ADR-0009; the extension keeps its no-regex builder). Group
 * names default to the seeded-rule convention.
 */
export function RuleForm({
  rule,
  initialHost,
  fieldErrors,
  busy,
  onSubmit,
  onCancel,
}: RuleFormProps) {
  const editing = rule !== undefined;
  const [host, setHost] = useState(rule?.host ?? initialHost ?? '');
  const [regex, setRegex] = useState(rule?.chapter_url_regex ?? '');
  const [slugGroup, setSlugGroup] = useState(
    rule?.slug_capture_group ?? 'slug',
  );
  const [chapterGroup, setChapterGroup] = useState(
    rule?.chapter_capture_group ?? 'chapter',
  );

  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    if (editing) {
      const patch: SiteRulePatch = {};
      if (host.trim() !== rule.host) patch.host = host.trim();
      if (regex.trim() !== rule.chapter_url_regex)
        patch.chapter_url_regex = regex.trim();
      if (slugGroup.trim() !== rule.slug_capture_group)
        patch.slug_capture_group = slugGroup.trim();
      if (chapterGroup.trim() !== rule.chapter_capture_group) {
        patch.chapter_capture_group = chapterGroup.trim();
      }
      // Nothing changed — closing beats a pointless round-trip.
      if (Object.keys(patch).length === 0) {
        onCancel();
        return;
      }
      onSubmit(patch);
      return;
    }
    onSubmit({
      host: host.trim(),
      chapter_url_regex: regex.trim(),
      slug_capture_group: slugGroup.trim(),
      chapter_capture_group: chapterGroup.trim(),
    });
  };

  const fieldError = (name: string) => fieldErrors?.[name];

  return (
    <form onSubmit={submit}>
      <div className="nc-field">
        <label htmlFor="rule-host">Host</label>
        <input
          className={`nc-input${fieldError('host') !== undefined ? ' is-invalid' : ''}`}
          id="rule-host"
          type="text"
          placeholder="e.g. scans.example.org"
          autoComplete="off"
          spellCheck={false}
          value={host}
          aria-invalid={fieldError('host') !== undefined}
          onChange={(event) => {
            setHost(event.target.value);
          }}
        />
        {fieldError('host') !== undefined ? (
          <p className="nc-field-error">{fieldError('host')}</p>
        ) : (
          <p className="nc-field-help nc-small">
            Domain name only — no https://, no path, no IP addresses.
          </p>
        )}
      </div>
      <div className="nc-field">
        <label htmlFor="rule-regex">Chapter URL pattern</label>
        <input
          className={`nc-input nc-input-mono${fieldError('chapter_url_regex') !== undefined ? ' is-invalid' : ''}`}
          id="rule-regex"
          type="text"
          placeholder="^/series/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9.]+)$"
          autoComplete="off"
          spellCheck={false}
          value={regex}
          aria-invalid={fieldError('chapter_url_regex') !== undefined}
          onChange={(event) => {
            setRegex(event.target.value);
          }}
        />
        {fieldError('chapter_url_regex') !== undefined ? (
          <p className="nc-field-error">{fieldError('chapter_url_regex')}</p>
        ) : (
          <p className="nc-field-help nc-small">
            A Go regular expression matched against the URL path. Mark the parts
            you need with named groups, <code>{'(?P<name>…)'}</code> — e.g.{' '}
            <code>
              {'^/series/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9.]+)$'}
            </code>{' '}
            matches <code>/series/solo-leveling/chapter-45.5</code>.
          </p>
        )}
      </div>
      <div className="nc-field-pair">
        <div className="nc-field">
          <label htmlFor="rule-slug-group">Slug group</label>
          <input
            className={`nc-input nc-input-mono${fieldError('slug_capture_group') !== undefined ? ' is-invalid' : ''}`}
            id="rule-slug-group"
            type="text"
            autoComplete="off"
            spellCheck={false}
            value={slugGroup}
            onChange={(event) => {
              setSlugGroup(event.target.value);
            }}
          />
          {fieldError('slug_capture_group') !== undefined && (
            <p className="nc-field-error">{fieldError('slug_capture_group')}</p>
          )}
        </div>
        <div className="nc-field">
          <label htmlFor="rule-ch-group">Chapter group</label>
          <input
            className={`nc-input nc-input-mono${fieldError('chapter_capture_group') !== undefined ? ' is-invalid' : ''}`}
            id="rule-ch-group"
            type="text"
            autoComplete="off"
            spellCheck={false}
            value={chapterGroup}
            onChange={(event) => {
              setChapterGroup(event.target.value);
            }}
          />
          {fieldError('chapter_capture_group') !== undefined && (
            <p className="nc-field-error">
              {fieldError('chapter_capture_group')}
            </p>
          )}
        </div>
      </div>
      <p className="nc-fieldset-help nc-small">
        Which named groups in the pattern hold the series slug and the chapter
        number. Leave as <code>slug</code> / <code>chapter</code> unless your
        pattern names them differently.
      </p>
      <div className="nc-form-actions">
        <button className="nc-btn-secondary" type="button" onClick={onCancel}>
          Cancel
        </button>
        <button className="nc-btn-primary" type="submit" disabled={busy}>
          {busy ? 'Saving…' : 'Save rule'}
        </button>
      </div>
    </form>
  );
}
