import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SiteRule } from '@nextchapter/api-client';
import { RulesSection } from '../../../components/options/RulesSection';

afterEach(cleanup);

const rules: SiteRule[] = [
  {
    id: 1,
    host: 'reader.example.com',
    chapter_url_regex: '^/series/(?P<slug>[^/]+)$',
  },
  {
    id: 2,
    host: 'comics.example.org',
    chapter_url_regex: '^/comic/(?P<slug>[^/]+)$',
  },
];

function renderSection(
  overrides?: Partial<Parameters<typeof RulesSection>[0]>,
) {
  const props = { rules, busy: false, onDelete: vi.fn(), ...overrides };
  render(<RulesSection {...props} />);
  return props;
}

describe('RulesSection', () => {
  it('lists host and pattern per rule', () => {
    renderSection();
    expect(screen.getByText('reader.example.com')).toBeVisible();
    expect(screen.getByText('^/comic/(?P<slug>[^/]+)$')).toBeVisible();
  });

  it('shows the empty state when there are no rules', () => {
    renderSection({ rules: [] });
    expect(screen.getByText(/No rules yet/)).toBeVisible();
    expect(
      screen.queryByRole('button', { name: /Delete rule/ }),
    ).not.toBeInTheDocument();
  });

  it('deletes only after the inline confirm', async () => {
    const props = renderSection();

    await userEvent.click(
      screen.getByRole('button', {
        name: 'Delete rule for comics.example.org',
      }),
    );
    expect(props.onDelete).not.toHaveBeenCalled();
    expect(screen.getByText('Delete rule?')).toBeVisible();
    // Cancel holds focus so Enter–Enter can never destroy data.
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();

    await userEvent.click(screen.getByRole('button', { name: 'Confirm' }));
    expect(props.onDelete).toHaveBeenCalledWith(2);
  });

  it('cancel and Escape both leave the rule alone', async () => {
    const props = renderSection();

    await userEvent.click(
      screen.getByRole('button', {
        name: 'Delete rule for comics.example.org',
      }),
    );
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByText('Delete rule?')).not.toBeInTheDocument();

    await userEvent.click(
      screen.getByRole('button', {
        name: 'Delete rule for comics.example.org',
      }),
    );
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByText('Delete rule?')).not.toBeInTheDocument();
    expect(props.onDelete).not.toHaveBeenCalled();
  });

  it('surfaces a delete failure as the red status line', () => {
    renderSection({ deleteError: "Couldn't delete the rule — not found" });
    expect(screen.getByRole('status')).toHaveTextContent(
      "Couldn't delete the rule — not found",
    );
  });
});
