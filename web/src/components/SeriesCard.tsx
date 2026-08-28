import { Link } from 'react-router';
import type { SeriesSummary } from '@nextchapter/api-client';
import { coverUrl, relativeTime, statusLabel } from '../lib/format';
import { CoverThumb } from './CoverThumb';

export function SeriesCard({ series }: { series: SeriesSummary }) {
  const sites = series.entry_count ?? 0;
  return (
    <Link
      className="nc-card"
      to={`/library/${String(series.id ?? 0)}`}
      title={series.title}
    >
      <CoverThumb
        src={coverUrl(series.id, series.cover_updated_at)}
        title={series.title ?? ''}
      />
      <div className="nc-card-body">
        <h2 className="nc-card-title">{series.title}</h2>
        <p className="nc-card-badges">
          <span className="nc-pill">{statusLabel(series.status)}</span>
          {series.rating !== undefined && (
            <span className="nc-card-rating">★ {series.rating}</span>
          )}
        </p>
        <p className="nc-card-tags">
          {(series.tags ?? []).map((tag) => (
            <span className="nc-chip" key={tag}>
              {tag}
            </span>
          ))}
        </p>
        <p className="nc-card-foot">
          {series.highest_chapter !== undefined ? (
            <span className="nc-card-progress">
              Read till ch <strong>{series.highest_chapter}</strong> ·{' '}
              {sites === 1 ? '1 site' : `${String(sites)} sites`}
            </span>
          ) : (
            <span className="nc-card-progress">No chapters yet</span>
          )}
          <span className="nc-card-when">
            {relativeTime(series.last_captured_at)}
          </span>
        </p>
      </div>
    </Link>
  );
}
