import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SiteRule } from '@nextchapter/api-client';
import { RuleForm } from '../../src/components/RuleForm';

afterEach(cleanup);

const existing: SiteRule = {
  id: 5,
  host: 'manhua.example.net',
  chapter_url_regex: '^/manga/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+)/?$',
  slug_capture_group: 'slug',
  chapter_capture_group: 'chapter',
};

describe('RuleForm', () => {
  it('creates with the full body and default group names', async () => {
    const onSubmit = vi.fn();
    render(<RuleForm busy={false} onSubmit={onSubmit} onCancel={vi.fn()} />);

    await userEvent.type(screen.getByLabelText('Host'), 'scans.example.org');
    // userEvent.type parses [ and { as key descriptors — paste regex strings.
    await userEvent.click(screen.getByLabelText('Chapter URL pattern'));
    await userEvent.paste('^/read/(?P<slug>[^/]+)/(?P<chapter>[0-9.]+)$');
    await userEvent.click(screen.getByRole('button', { name: 'Save rule' }));

    expect(onSubmit).toHaveBeenCalledWith({
      host: 'scans.example.org',
      chapter_url_regex: '^/read/(?P<slug>[^/]+)/(?P<chapter>[0-9.]+)$',
      slug_capture_group: 'slug',
      chapter_capture_group: 'chapter',
    });
  });

  it('prefills edit mode and submits only the changed fields', async () => {
    const onSubmit = vi.fn();
    render(
      <RuleForm
        rule={existing}
        busy={false}
        onSubmit={onSubmit}
        onCancel={vi.fn()}
      />,
    );

    const regex = screen.getByLabelText('Chapter URL pattern');
    expect(regex).toHaveValue(existing.chapter_url_regex);
    await userEvent.clear(regex);
    await userEvent.paste('^/comic/(?P<slug>[^/]+)/(?P<chapter>[0-9.]+)$');
    await userEvent.click(screen.getByRole('button', { name: 'Save rule' }));

    expect(onSubmit).toHaveBeenCalledWith({
      chapter_url_regex: '^/comic/(?P<slug>[^/]+)/(?P<chapter>[0-9.]+)$',
    });
  });

  it('prefills the host from a no-rule hint row', () => {
    render(
      <RuleForm
        initialHost="scans.example.org"
        busy={false}
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByLabelText('Host')).toHaveValue('scans.example.org');
  });

  it('renders server field errors in place of the help text', () => {
    render(
      <RuleForm
        busy={false}
        fieldErrors={{
          host: 'already has a rule',
          chapter_url_regex: 'failed to compile',
        }}
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByText('already has a rule')).toBeVisible();
    expect(screen.getByText('failed to compile')).toBeVisible();
    expect(screen.getByLabelText('Host')).toHaveAttribute(
      'aria-invalid',
      'true',
    );
  });

  it('cancel fires without submitting', async () => {
    const onCancel = vi.fn();
    const onSubmit = vi.fn();
    render(<RuleForm busy={false} onSubmit={onSubmit} onCancel={onCancel} />);
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onCancel).toHaveBeenCalledOnce();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
