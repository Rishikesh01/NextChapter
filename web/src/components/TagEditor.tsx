import { useState } from 'react';

const TAG_PATTERN = /^[a-z0-9][a-z0-9-]{0,31}$/;
const MAX_TAGS = 16;

export interface TagEditorProps {
  tags: string[];
  busy: boolean;
  /** Called with the FULL replacement list — the API replaces, never diffs. */
  onChange: (tags: string[]) => void;
}

export function TagEditor({ tags, busy, onChange }: TagEditorProps) {
  const [draft, setDraft] = useState('');
  const [error, setError] = useState<string>();

  const commit = () => {
    const tag = draft.trim().toLowerCase();
    if (tag === '') return;
    if (!TAG_PATTERN.test(tag)) {
      setError('Tags are lowercase letters, digits and dashes (max 32 chars).');
      return;
    }
    if (tags.length >= MAX_TAGS) {
      setError(`At most ${String(MAX_TAGS)} tags per series.`);
      return;
    }
    setError(undefined);
    setDraft('');
    if (!tags.includes(tag)) onChange([...tags, tag]);
  };

  return (
    <>
      <div className="nc-tag-editor" role="group" aria-label="Tags">
        {tags.map((tag) => (
          <span className="nc-chip" key={tag}>
            {tag}
            <button
              className="nc-chip-remove"
              type="button"
              aria-label={`Remove tag ${tag}`}
              disabled={busy}
              onClick={() => {
                onChange(tags.filter((existing) => existing !== tag));
              }}
            >
              ×
            </button>
          </span>
        ))}
        <input
          className={`nc-tag-add${error !== undefined ? ' is-invalid' : ''}`}
          type="text"
          placeholder="add tag"
          aria-label="Add a tag"
          autoComplete="off"
          spellCheck={false}
          disabled={busy}
          value={draft}
          onChange={(event) => {
            setDraft(event.target.value);
            setError(undefined);
          }}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ',') {
              event.preventDefault();
              commit();
            }
          }}
          onBlur={commit}
        />
      </div>
      {error !== undefined && <p className="nc-field-error">{error}</p>}
    </>
  );
}
