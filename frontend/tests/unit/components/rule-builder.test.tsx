import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RuleBuilder } from '../../../components/popup/RuleBuilder';
import { ManualCaptureForm } from '../../../components/popup/ManualCaptureForm';
import { buildRule, previewRule } from '../../../lib/rule-builder';

const URL_UNDER_TEST =
  'https://readnovel.example/manga/the-mad-dog/chapter-54/';
const SEGMENTS = ['manga', 'the-mad-dog', 'chapter-54'];

function previewFor(slugIndex: number, chapterIndex: number) {
  const rule = buildRule(URL_UNDER_TEST, { slugIndex, chapterIndex });
  return rule !== null ? previewRule(URL_UNDER_TEST, rule) : null;
}

function renderBuilder(overrides?: Partial<Parameters<typeof RuleBuilder>[0]>) {
  const props = {
    segments: SEGMENTS,
    draft: { slugIndex: 1, chapterIndex: 2 },
    preview: previewFor(1, 2),
    busy: false,
    onDraftChange: vi.fn(),
    onSave: vi.fn(),
    onBack: vi.fn(),
    ...overrides,
  };
  render(<RuleBuilder {...props} />);
  return props;
}

afterEach(cleanup);

describe('RuleBuilder', () => {
  it('renders every segment with pre-selected radios and a live preview', () => {
    renderBuilder();

    expect(
      screen.getByRole('radio', { name: 'Series name part: the-mad-dog' }),
    ).toBeChecked();
    expect(
      screen.getByRole('radio', { name: 'Chapter part: chapter-54' }),
    ).toBeChecked();
    expect(
      screen.getByText('the-mad-dog', { selector: '.nc-rule-preview-slug' }),
    ).toBeVisible();
    expect(screen.getByText('54')).toBeVisible();
    expect(
      screen.getByRole('button', { name: 'Save rule & capture' }),
    ).toBeEnabled();
  });

  it('reports draft changes from the radio grid', async () => {
    const props = renderBuilder();
    await userEvent.click(
      screen.getByRole('radio', { name: 'Series name part: manga' }),
    );
    expect(props.onDraftChange).toHaveBeenCalledWith({
      slugIndex: 0,
      chapterIndex: 2,
    });
  });

  it('disables Save and explains when the chapter part has no number', () => {
    renderBuilder({
      draft: { slugIndex: 1, chapterIndex: 0 },
      preview: previewFor(1, 0),
    });

    expect(screen.getByText(/“manga” has no number/)).toBeVisible();
    expect(
      screen.getByRole('button', { name: 'Save rule & capture' }),
    ).toBeDisabled();
  });

  it('disables Save when one segment is picked for both roles', () => {
    renderBuilder({ draft: { slugIndex: 2, chapterIndex: 2 }, preview: null });

    expect(
      screen.getByText('The series and chapter must be different parts.'),
    ).toBeVisible();
    expect(
      screen.getByRole('button', { name: 'Save rule & capture' }),
    ).toBeDisabled();
  });

  it('saves on submit and goes back on the link', async () => {
    const props = renderBuilder();
    await userEvent.click(
      screen.getByRole('button', { name: 'Save rule & capture' }),
    );
    expect(props.onSave).toHaveBeenCalledOnce();
    await userEvent.click(
      screen.getByRole('button', { name: 'Back to manual entry' }),
    );
    expect(props.onBack).toHaveBeenCalledOnce();
  });
});

describe('ManualCaptureForm rule entry point', () => {
  const base = {
    slug: '',
    chapter: '',
    busy: false,
    onSlugChange: vi.fn(),
    onChapterChange: vi.fn(),
    onCapture: vi.fn(),
  };

  it('offers the builder when onCreateRule is provided', async () => {
    const onCreateRule = vi.fn();
    render(<ManualCaptureForm {...base} onCreateRule={onCreateRule} />);

    expect(
      screen.getByText(
        'A rule lets NextChapter detect chapters here automatically.',
      ),
    ).toBeVisible();
    await userEvent.click(
      screen.getByRole('button', { name: 'Create a rule from this page' }),
    );
    expect(onCreateRule).toHaveBeenCalledOnce();
  });

  it('shows no entry point (and no hint) when a rule cannot be built here', () => {
    render(<ManualCaptureForm {...base} />);
    expect(
      screen.queryByRole('button', { name: 'Create a rule from this page' }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/detect chapters here automatically/),
    ).not.toBeInTheDocument();
  });
});
