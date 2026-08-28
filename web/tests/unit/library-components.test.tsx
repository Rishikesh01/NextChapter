import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MemoryRouter } from 'react-router';
import type { SeriesSummary } from '@nextchapter/api-client';
import { SeriesCard } from '../../src/components/SeriesCard';
import { SeriesFilters } from '../../src/components/SeriesFilters';

afterEach(cleanup);

const series: SeriesSummary = {
  id: 7,
  title: 'Solo Leveling',
  status: 'reading',
  rating: 9,
  tags: ['action', 'dungeon'],
  highest_chapter: 101,
  entry_count: 3,
  last_captured_at: new Date(Date.now() - 2 * 3600_000).toISOString(),
};

function renderCard(overrides?: Partial<SeriesSummary>) {
  render(
    <MemoryRouter>
      <SeriesCard series={{ ...series, ...overrides }} />
    </MemoryRouter>,
  );
}

describe('SeriesCard', () => {
  it('renders the rollups and links to the detail page', () => {
    renderCard();
    const card = screen.getByRole('link', { name: /Solo Leveling/ });
    expect(card).toHaveAttribute('href', '/library/7');
    expect(screen.getByText('Reading')).toBeVisible();
    expect(screen.getByText('★ 9')).toBeVisible();
    expect(screen.getByText('action')).toBeVisible();
    expect(screen.getByText(/Read till ch/)).toHaveTextContent(
      'Read till ch 101 · 3 sites',
    );
    expect(screen.getByText('2 h ago')).toBeVisible();
  });

  it('says "No chapters yet" for a null rollup, never ch 0', () => {
    renderCard({
      highest_chapter: undefined,
      entry_count: 0,
      last_captured_at: undefined,
    });
    expect(screen.getByText('No chapters yet')).toBeVisible();
    expect(screen.queryByText(/ch 0/)).not.toBeInTheDocument();
  });

  it('singularizes one site and omits a missing rating', () => {
    renderCard({ entry_count: 1, rating: undefined });
    expect(screen.getByText(/1 site$/)).toBeVisible();
    expect(screen.queryByText(/★/)).not.toBeInTheDocument();
  });
});

describe('SeriesFilters', () => {
  const base = {
    status: '' as const,
    tags: ['action'],
    shown: 6,
    total: 57,
    onStatusChange: vi.fn(),
    onAddTag: vi.fn(),
    onRemoveTag: vi.fn(),
  };

  it('shows the count and fires status changes', async () => {
    const props = { ...base, onStatusChange: vi.fn() };
    render(<SeriesFilters {...props} />);

    expect(screen.getByText('6 of 57 series')).toBeVisible();
    await userEvent.selectOptions(
      screen.getByLabelText('Filter by status'),
      'completed',
    );
    expect(props.onStatusChange).toHaveBeenCalledWith('completed');
  });

  it('adds a tag on Enter (lowercased) and removes via the chip', async () => {
    const props = { ...base, onAddTag: vi.fn(), onRemoveTag: vi.fn() };
    render(<SeriesFilters {...props} />);

    await userEvent.type(
      screen.getByLabelText('Filter by tag'),
      'Isekai{Enter}',
    );
    expect(props.onAddTag).toHaveBeenCalledWith('isekai');

    await userEvent.click(
      screen.getByRole('button', { name: 'Remove tag filter action' }),
    );
    expect(props.onRemoveTag).toHaveBeenCalledWith('action');
  });
});
