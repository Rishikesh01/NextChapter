import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PopupHeader } from '../../../components/popup/PopupHeader';
import { EmptyState } from '../../../components/popup/EmptyState';
import { NotConfigured } from '../../../components/popup/NotConfigured';
import {
  ChapterInput,
  parseChapterInput,
} from '../../../components/popup/ChapterInput';
import { DetectedCapture } from '../../../components/popup/DetectedCapture';
import { ManualCaptureForm } from '../../../components/popup/ManualCaptureForm';
import { StatusBanner } from '../../../components/popup/StatusBanner';

afterEach(cleanup);

describe('PopupHeader', () => {
  it('shows the host and opens settings from the gear', async () => {
    const onOpenSettings = vi.fn();
    render(
      <PopupHeader host="reader.example.com" onOpenSettings={onOpenSettings} />,
    );

    expect(screen.getByText('reader.example.com')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Settings' }));
    expect(onOpenSettings).toHaveBeenCalledOnce();
  });
});

describe('EmptyState', () => {
  it('renders title and text', () => {
    render(
      <EmptyState
        title="Nothing to capture here"
        text="Open a chapter page."
      />,
    );
    expect(screen.getByText('Nothing to capture here')).toBeInTheDocument();
    expect(screen.getByText('Open a chapter page.')).toBeInTheDocument();
  });
});

describe('NotConfigured', () => {
  it('routes to settings', async () => {
    const onOpenSettings = vi.fn();
    render(<NotConfigured onOpenSettings={onOpenSettings} />);
    await userEvent.click(
      screen.getByRole('button', { name: 'Open settings' }),
    );
    expect(onOpenSettings).toHaveBeenCalledOnce();
  });
});

describe('ChapterInput', () => {
  it('reports edits and shows a field error', async () => {
    const onChange = vi.fn();
    render(
      <ChapterInput
        value="45.5"
        onChange={onChange}
        error="Enter a chapter number"
      />,
    );

    const input = screen.getByLabelText('Chapter');
    expect(input).toHaveValue('45.5');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByText('Enter a chapter number')).toBeInTheDocument();
    await userEvent.type(input, '1');
    expect(onChange).toHaveBeenCalledWith('45.51');
  });
});

describe('parseChapterInput', () => {
  it.each([
    ['101', 101],
    [' 45.5 ', 45.5],
    ['0', 0],
  ])('accepts %s', (input, expected) => {
    expect(parseChapterInput(input)).toBe(expected);
  });

  it.each([[''], ['abc'], ['1.2.3'], ['-4'], ['1e3']])(
    'rejects %s',
    (input) => {
      expect(parseChapterInput(input)).toBeNull();
    },
  );
});

describe('DetectedCapture', () => {
  it('shows the detected series and submits on the capture button', async () => {
    const onCapture = vi.fn();
    render(
      <DetectedCapture
        seriesTitle="Solo Leveling"
        chapter="101"
        busy={false}
        onChapterChange={vi.fn()}
        onCapture={onCapture}
        host="reader.example.com"
        autoTrack={null}
        onToggleAutoTrack={vi.fn()}
      />,
    );

    expect(
      screen.getByRole('heading', { name: 'Solo Leveling' }),
    ).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole('button', { name: 'Capture chapter' }),
    );
    expect(onCapture).toHaveBeenCalledOnce();
  });

  it('disables the button while busy', () => {
    render(
      <DetectedCapture
        seriesTitle="Solo Leveling"
        chapter="101"
        busy
        onChapterChange={vi.fn()}
        onCapture={vi.fn()}
        host="reader.example.com"
        autoTrack={null}
        onToggleAutoTrack={vi.fn()}
      />,
    );
    expect(screen.getByRole('button', { name: 'Capturing…' })).toBeDisabled();
  });
});

describe('ManualCaptureForm', () => {
  it('edits slug and chapter and submits', async () => {
    const onSlugChange = vi.fn();
    const onCapture = vi.fn();
    render(
      <ManualCaptureForm
        slug=""
        chapter=""
        busy={false}
        onSlugChange={onSlugChange}
        onChapterChange={vi.fn()}
        onCapture={onCapture}
      />,
    );

    await userEvent.type(screen.getByLabelText('Series slug'), 's');
    expect(onSlugChange).toHaveBeenCalledWith('s');
    await userEvent.click(
      screen.getByRole('button', { name: 'Capture chapter' }),
    );
    expect(onCapture).toHaveBeenCalledOnce();
  });

  it('marks an invalid slug', () => {
    render(
      <ManualCaptureForm
        slug=""
        chapter=""
        slugError="Enter the series slug"
        busy={false}
        onSlugChange={vi.fn()}
        onChapterChange={vi.fn()}
        onCapture={vi.fn()}
      />,
    );
    expect(screen.getByLabelText('Series slug')).toHaveAttribute(
      'aria-invalid',
      'true',
    );
    expect(screen.getByText('Enter the series slug')).toBeInTheDocument();
  });
});

describe('StatusBanner', () => {
  it('renders success as a status', () => {
    render(
      <StatusBanner kind="success">
        Advanced <strong>Solo Leveling</strong> to ch <strong>46</strong>
      </StatusBanner>,
    );
    expect(screen.getByRole('status')).toHaveTextContent(
      'Advanced Solo Leveling to ch 46',
    );
  });

  it('renders an error with an inline action', async () => {
    const onAction = vi.fn();
    render(
      <StatusBanner
        kind="error"
        actionLabel="open settings"
        onAction={onAction}
      >
        Token rejected —
      </StatusBanner>,
    );
    expect(screen.getByRole('alert')).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole('button', { name: 'open settings' }),
    );
    expect(onAction).toHaveBeenCalledOnce();
  });

  it('hides the auto-track toggle until the permission state is known', () => {
    render(
      <DetectedCapture
        seriesTitle="Solo Leveling"
        chapter="101"
        busy={false}
        onChapterChange={vi.fn()}
        onCapture={vi.fn()}
        host="reader.example.com"
        autoTrack={null}
        onToggleAutoTrack={vi.fn()}
      />,
    );
    // Rendering it unchecked while the real answer is still loading would
    // read as "off" and invite a pointless second permission prompt.
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('names the host and warns about the access ask when off', async () => {
    const onToggle = vi.fn();
    render(
      <DetectedCapture
        seriesTitle="Solo Leveling"
        chapter="101"
        busy={false}
        onChapterChange={vi.fn()}
        onCapture={vi.fn()}
        host="reader.example.com"
        autoTrack={false}
        onToggleAutoTrack={onToggle}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /reader\.example\.com/ });
    expect(toggle).not.toBeChecked();
    expect(
      screen.getByText(/Asks for access to this site/),
    ).toBeInTheDocument();

    await userEvent.click(toggle);
    expect(onToggle).toHaveBeenCalledWith(true);
  });

  it('reports the live state and turns back off', async () => {
    const onToggle = vi.fn();
    render(
      <DetectedCapture
        seriesTitle="Solo Leveling"
        chapter="101"
        busy={false}
        onChapterChange={vi.fn()}
        onCapture={vi.fn()}
        host="reader.example.com"
        autoTrack
        onToggleAutoTrack={onToggle}
      />,
    );

    const toggle = screen.getByRole('checkbox', { name: /reader\.example\.com/ });
    expect(toggle).toBeChecked();
    await userEvent.click(toggle);
    expect(onToggle).toHaveBeenCalledWith(false);
  });
});
