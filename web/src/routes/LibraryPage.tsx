import { Link, useSearchParams } from 'react-router';
import type { SeriesStatus } from '@nextchapter/api-client';
import { useSeriesPages } from '../lib/queries';
import { SeriesCard } from '../components/SeriesCard';
import { SeriesFilters } from '../components/SeriesFilters';
import { ErrorBanner } from '../components/ErrorBanner';

export function LibraryPage() {
  // Filter state lives in the query string so it survives reload/back.
  const [searchParams, setSearchParams] = useSearchParams();
  const status = (searchParams.get('status') ?? '') as SeriesStatus | '';
  const tags = searchParams.getAll('tag');
  const filtered = status !== '' || tags.length > 0;

  const pages = useSeriesPages({
    ...(status !== '' ? { status } : {}),
    ...(tags.length > 0 ? { tag: tags } : {}),
  });

  const items = pages.data?.pages.flatMap((page) => page.items ?? []) ?? [];
  const total = pages.data?.pages.at(-1)?.total ?? 0;

  const updateParams = (mutate: (params: URLSearchParams) => void) => {
    const next = new URLSearchParams(searchParams);
    mutate(next);
    setSearchParams(next, { replace: true });
  };

  return (
    <>
      <h1 className="nc-page-title">Library</h1>

      {(items.length > 0 || filtered) && (
        <SeriesFilters
          status={status}
          tags={tags}
          shown={items.length}
          total={total}
          onStatusChange={(nextStatus) => {
            updateParams((params) => {
              if (nextStatus === '') params.delete('status');
              else params.set('status', nextStatus);
            });
          }}
          onAddTag={(tag) => {
            updateParams((params) => {
              params.append('tag', tag);
            });
          }}
          onRemoveTag={(tag) => {
            updateParams((params) => {
              const kept = params
                .getAll('tag')
                .filter((existing) => existing !== tag);
              params.delete('tag');
              for (const existing of kept) params.append('tag', existing);
            });
          }}
        />
      )}

      {pages.isError && <ErrorBanner>{pages.error.message}</ErrorBanner>}

      {items.length > 0 && (
        <div className="nc-grid">
          {items.map((series) => (
            <SeriesCard key={series.id} series={series} />
          ))}
        </div>
      )}

      {!pages.isPending && items.length === 0 && !filtered && (
        <div className="nc-empty">
          <p className="nc-empty-title">Nothing here yet</p>
          <p className="nc-empty-line nc-small">
            Capture your first chapter from the browser extension — it appears
            here instantly.
          </p>
          <p className="nc-empty-line nc-small">
            Need a token for the extension?{' '}
            <Link to="/settings">Create one in Settings</Link>.
          </p>
        </div>
      )}

      {!pages.isPending && items.length === 0 && filtered && (
        <div className="nc-empty">
          <p className="nc-empty-title">No series match these filters</p>
          <p className="nc-empty-line nc-small">
            <button
              className="nc-btn-link"
              type="button"
              onClick={() => {
                setSearchParams(new URLSearchParams(), { replace: true });
              }}
            >
              Clear filters
            </button>
          </p>
        </div>
      )}

      {pages.hasNextPage && (
        <div className="nc-loadmore">
          <button
            className="nc-btn-secondary"
            type="button"
            disabled={pages.isFetchingNextPage}
            onClick={() => {
              void pages.fetchNextPage();
            }}
          >
            {pages.isFetchingNextPage ? 'Loading…' : 'Load more'}
          </button>
          <p className="nc-loadmore-caption nc-small">
            Showing {items.length} of {total}
          </p>
        </div>
      )}
    </>
  );
}
