import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Entry, SeriesSummary } from '@nextchapter/api-client';
import { EntryRow } from '../../src/components/EntryRow';
import { TagEditor } from '../../src/components/TagEditor';
import { ConfirmInline } from '../../src/components/ConfirmInline';
import { ReassignEntryDialog } from '../../src/components/ReassignEntryDialog';

afterEach(cleanup);

const entry: Entry = {
  id: 3,
  series_id: 7,
  site_host: 'reader.example.com',
  series_slug: 'solo-leveling',
  site_title: 'Solo Leveling Chapter 101 – Example Reader',
  last_chapter: 101,
  last_url: 'https://reader.example.com/series/solo-leveling/chapter-101',
  last_captured_at: new Date().toISOString(),
};

function renderRow(overrides?: Partial<Parameters<typeof EntryRow>[0]>) {
  const props = {
    entry,
    busy: false,
    onMove: vi.fn(),
    onSave: vi.fn(),
    onRemove: vi.fn(),
    ...overrides,
  };
  render(
    <table>
      <tbody>
        <EntryRow {...props} />
      </tbody>
    </table>,
  );
  return props;
}

describe('EntryRow', () => {
  it('links Continue reading to last_url in a new tab', () => {
    renderRow();
    const link = screen.getByRole('link', { name: /Continue reading/ });
    expect(link).toHaveAttribute('href', entry.last_url);
    expect(link).toHaveAttribute('target', '_blank');
    expect(link.getAttribute('rel')).toContain('noopener');
  });

  it('opens the inline edit prefilled and saves a corrected chapter + URL', async () => {
    const props = renderRow();
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }));

    const chapter = screen.getByLabelText('Chapter');
    expect(chapter).toHaveValue('101');
    await userEvent.clear(chapter);
    await userEvent.type(chapter, '102.5');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(props.onSave).toHaveBeenCalledWith({
      last_chapter: 102.5,
      last_url: entry.last_url,
    });
  });

  it('rejects a non-numeric chapter with a field error', async () => {
    const props = renderRow();
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }));
    await userEvent.clear(screen.getByLabelText('Chapter'));
    await userEvent.type(screen.getByLabelText('Chapter'), 'abc');
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(screen.getByText(/Enter a chapter number/)).toBeVisible();
    expect(props.onSave).not.toHaveBeenCalled();
  });

  it('removes only after the confirm; Move fires directly', async () => {
    const props = renderRow();
    await userEvent.click(screen.getByRole('button', { name: 'Remove' }));
    expect(props.onRemove).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }));
    expect(props.onRemove).toHaveBeenCalledOnce();

    await userEvent.click(screen.getByRole('button', { name: 'Move' }));
    expect(props.onMove).toHaveBeenCalledOnce();
  });
});

describe('TagEditor', () => {
  it('commits a valid tag as a full replacement list', async () => {
    const onChange = vi.fn();
    render(<TagEditor tags={['action']} busy={false} onChange={onChange} />);

    await userEvent.type(screen.getByLabelText('Add a tag'), 'isekai{Enter}');
    expect(onChange).toHaveBeenCalledWith(['action', 'isekai']);
  });

  it('rejects an invalid tag with the pattern hint', async () => {
    const onChange = vi.fn();
    render(<TagEditor tags={[]} busy={false} onChange={onChange} />);

    await userEvent.type(screen.getByLabelText('Add a tag'), 'Bad Tag!{Enter}');
    expect(
      screen.getByText(/lowercase letters, digits and dashes/),
    ).toBeVisible();
    expect(onChange).not.toHaveBeenCalled();
  });

  it('removes a tag via its chip', async () => {
    const onChange = vi.fn();
    render(
      <TagEditor
        tags={['action', 'dungeon']}
        busy={false}
        onChange={onChange}
      />,
    );

    await userEvent.click(
      screen.getByRole('button', { name: 'Remove tag action' }),
    );
    expect(onChange).toHaveBeenCalledWith(['dungeon']);
  });
});

describe('ConfirmInline', () => {
  it('arms, focuses Cancel, and never confirms on Enter–Enter', async () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmInline label="Delete" question="Delete?" onConfirm={onConfirm} />,
    );

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }));
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
    await userEvent.keyboard('{Enter}');
    expect(onConfirm).not.toHaveBeenCalled();
    expect(screen.queryByText('Delete?')).not.toBeInTheDocument();
  });

  it('escape disarms; Confirm fires', async () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmInline label="Delete" question="Delete?" onConfirm={onConfirm} />,
    );

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByText('Delete?')).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }));
    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }));
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});

describe('ReassignEntryDialog', () => {
  const seriesList: SeriesSummary[] = [
    {
      id: 9,
      title: 'Omniscient Reader',
      highest_chapter: 45.5,
      entry_count: 1,
    },
    { id: 11, title: 'Wind Breaker', entry_count: 0 },
  ];

  function renderDialog(
    overrides?: Partial<Parameters<typeof ReassignEntryDialog>[0]>,
  ) {
    const props = {
      entry,
      series: seriesList,
      busy: false,
      onPick: vi.fn(),
      onCreate: vi.fn(),
      onClose: vi.fn(),
      ...overrides,
    };
    render(<ReassignEntryDialog {...props} />);
    return props;
  }

  it('shows the context and picks an existing series', async () => {
    const props = renderDialog();
    expect(screen.getByText('solo-leveling')).toBeVisible();
    await userEvent.click(
      screen.getByRole('button', { name: /Omniscient Reader/ }),
    );
    expect(props.onPick).toHaveBeenCalledWith(9);
  });

  it('filters and creates from the typed title', async () => {
    const props = renderDialog();
    await userEvent.type(screen.getByRole('searchbox'), 'ORV Remastered');

    expect(
      screen.queryByRole('button', { name: /Wind Breaker/ }),
    ).not.toBeInTheDocument();
    await userEvent.click(
      screen.getByRole('button', { name: /Create new series/ }),
    );
    expect(props.onCreate).toHaveBeenCalledWith('ORV Remastered');
  });

  it('prefills the create row from the entry site title and closes on Cancel', async () => {
    const props = renderDialog();
    expect(
      screen.getByRole('button', {
        name: `Create new series: “${entry.site_title ?? ''}”`,
      }),
    ).toBeVisible();
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(props.onClose).toHaveBeenCalledOnce();
  });
});
