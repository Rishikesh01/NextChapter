import { useState } from 'react';
import type { SeriesStatus } from '@nextchapter/api-client';
import { STATUS_OPTIONS } from '../lib/format';

export interface SeriesFiltersProps {
  status: SeriesStatus | '';
  tags: string[];
  shown: number;
  total: number;
  onStatusChange: (status: SeriesStatus | '') => void;
  onAddTag: (tag: string) => void;
  onRemoveTag: (tag: string) => void;
}

export function SeriesFilters({
  status,
  tags,
  shown,
  total,
  onStatusChange,
  onAddTag,
  onRemoveTag,
}: SeriesFiltersProps) {
  const [draft, setDraft] = useState('');

  const commit = () => {
    const tag = draft.trim().toLowerCase();
    if (tag !== '' && !tags.includes(tag)) onAddTag(tag);
    setDraft('');
  };

  return (
    <div className="nc-filters">
      <select
        className="nc-input"
        aria-label="Filter by status"
        value={status}
        onChange={(event) => {
          onStatusChange(event.target.value as SeriesStatus | '');
        }}
      >
        <option value="">All statuses</option>
        {STATUS_OPTIONS.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      <input
        className="nc-input nc-filter-tag-input"
        type="text"
        placeholder="Filter by tag"
        aria-label="Filter by tag"
        autoComplete="off"
        spellCheck={false}
        value={draft}
        onChange={(event) => {
          setDraft(event.target.value);
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault();
            commit();
          }
        }}
      />
      {tags.map((tag) => (
        <span className="nc-chip" key={tag}>
          {tag}
          <button
            className="nc-chip-remove"
            type="button"
            aria-label={`Remove tag filter ${tag}`}
            onClick={() => {
              onRemoveTag(tag);
            }}
          >
            ×
          </button>
        </span>
      ))}
      <span className="nc-filter-count nc-small">
        {shown} of {total} series
      </span>
    </div>
  );
}
