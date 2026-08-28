import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SeriesSummary } from '@nextchapter/api-client';
import { SeriesPicker } from '../../../components/popup/SeriesPicker';

afterEach(cleanup);

const series: SeriesSummary[] = [
  { id: 1, title: 'Solo Leveling', highest_chapter: 100, entry_count: 2 },
  { id: 2, title: 'Omniscient Reader', highest_chapter: 45.5, entry_count: 1 },
];

function renderPicker(overrides?: Partial<Parameters<typeof SeriesPicker>[0]>) {
  const props = {
    seriesSlug: 'solo-leveling',
    siteHost: 'reader.example.com',
    suggestedTitle: 'Solo Leveling',
    series,
    loading: false,
    busy: false,
    onPick: vi.fn(),
    onCreate: vi.fn(),
    ...overrides,
  };
  render(<SeriesPicker {...props} />);
  return props;
}

describe('SeriesPicker', () => {
  it('shows the capture context and per-series rollups', () => {
    renderPicker();
    expect(screen.getByText('solo-leveling')).toBeInTheDocument();
    expect(screen.getByText(/read till ch 100 · 2 sites/)).toBeInTheDocument();
    expect(screen.getByText(/read till ch 45.5 · 1 site$/)).toBeInTheDocument();
  });

  it('picks an existing series', async () => {
    const props = renderPicker();
    await userEvent.click(
      screen.getByRole('button', { name: /Omniscient Reader/ }),
    );
    expect(props.onPick).toHaveBeenCalledWith(series[1]);
  });

  it('creates from the pinned row with the suggested title', async () => {
    const props = renderPicker();
    await userEvent.click(
      screen.getByRole('button', { name: /Create new series/ }),
    );
    expect(props.onCreate).toHaveBeenCalledWith('Solo Leveling');
  });

  it('filters client-side and creates from the filter text', async () => {
    const props = renderPicker();
    await userEvent.type(screen.getByRole('searchbox'), 'reader');

    expect(
      screen.queryByRole('button', { name: /Solo Leveling$/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Omniscient Reader/ }),
    ).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole('button', { name: /Create new series/ }),
    );
    expect(props.onCreate).toHaveBeenCalledWith('reader');
  });

  it('shows the filtered-to-empty state but keeps the create row', async () => {
    renderPicker();
    await userEvent.type(screen.getByRole('searchbox'), 'zzz');
    expect(screen.getByText(/No series match/)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /Create new series/ }),
    ).toBeInTheDocument();
  });

  it('renders the create row alone for a zero-series account', () => {
    renderPicker({ series: [] });
    expect(
      screen.getByRole('button', { name: /Create new series/ }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/No series match/)).not.toBeInTheDocument();
  });
});
