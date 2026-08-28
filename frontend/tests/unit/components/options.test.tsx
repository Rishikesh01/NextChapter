import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ConnectionStatus } from '../../../components/options/ConnectionStatus';
import { ServerStep } from '../../../components/options/ServerStep';
import { SignInStep } from '../../../components/options/SignInStep';
import { PasteTokenStep } from '../../../components/options/PasteTokenStep';
import { ConnectedCard } from '../../../components/options/ConnectedCard';

afterEach(cleanup);

describe('ConnectionStatus', () => {
  it.each([
    ['unchecked', 'Not checked yet'],
    ['checking', 'Checking…'],
    ['ok', 'Server reachable'],
  ] as const)('%s renders "%s"', (state, expected) => {
    render(<ConnectionStatus state={state} />);
    expect(screen.getByRole('status')).toHaveTextContent(expected);
  });

  it('appends the failure detail in the bad state', () => {
    render(<ConnectionStatus state="bad" detail="check the URL" />);
    expect(screen.getByRole('status')).toHaveTextContent(
      'Could not reach server — check the URL',
    );
  });
});

describe('ServerStep', () => {
  it('edits the URL and connects on submit', async () => {
    const onUrlChange = vi.fn();
    const onConnect = vi.fn();
    render(
      <ServerStep
        url=""
        state="unchecked"
        onUrlChange={onUrlChange}
        onConnect={onConnect}
      />,
    );

    await userEvent.type(screen.getByLabelText('Server URL'), 'h');
    expect(onUrlChange).toHaveBeenCalledWith('h');
    await userEvent.click(screen.getByRole('button', { name: 'Connect' }));
    expect(onConnect).toHaveBeenCalledOnce();
  });

  it('disables Connect while checking', () => {
    render(
      <ServerStep
        url=""
        state="checking"
        onUrlChange={vi.fn()}
        onConnect={vi.fn()}
      />,
    );
    expect(screen.getByRole('button', { name: 'Connect' })).toBeDisabled();
  });
});

describe('SignInStep', () => {
  it('submits credentials', async () => {
    const onSubmit = vi.fn();
    render(<SignInStep busy={false} onSubmit={onSubmit} />);

    await userEvent.type(screen.getByLabelText('Username'), 'rishi');
    await userEvent.type(screen.getByLabelText('Password'), 'hunter22');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(onSubmit).toHaveBeenCalledWith('rishi', 'hunter22', false);
  });

  it('toggles to account creation', async () => {
    const onSubmit = vi.fn();
    render(<SignInStep busy={false} onSubmit={onSubmit} />);

    await userEvent.click(
      screen.getByRole('button', { name: /Create an account instead/ }),
    );
    await userEvent.type(screen.getByLabelText('Username'), 'rishi');
    await userEvent.type(screen.getByLabelText('Password'), 'hunter22');
    await userEvent.click(
      screen.getByRole('button', { name: 'Create account' }),
    );
    expect(onSubmit).toHaveBeenCalledWith('rishi', 'hunter22', true);
  });

  it('does not submit empty fields', async () => {
    const onSubmit = vi.fn();
    render(<SignInStep busy={false} onSubmit={onSubmit} />);
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

describe('PasteTokenStep', () => {
  it('links to the server swagger UI and submits a trimmed token', async () => {
    const onSubmit = vi.fn();
    render(
      <PasteTokenStep
        serverUrl="https://next.example.com"
        busy={false}
        onSubmit={onSubmit}
      />,
    );

    expect(
      screen.getByRole('link', { name: 'next.example.com/swagger' }),
    ).toHaveAttribute('href', 'https://next.example.com/swagger/index.html');
    await userEvent.type(screen.getByLabelText('API token'), '  nca_abc  ');
    await userEvent.click(screen.getByRole('button', { name: 'Save token' }));
    expect(onSubmit).toHaveBeenCalledWith('nca_abc');
  });
});

describe('ConnectedCard', () => {
  it('shows the connection summary and disconnects', async () => {
    const onDisconnect = vi.fn();
    render(
      <ConnectedCard
        username="rishikesh"
        serverUrl="https://next.example.com"
        onDisconnect={onDisconnect}
      />,
    );

    expect(screen.getByText('rishikesh')).toBeInTheDocument();
    expect(screen.getByText('https://next.example.com')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Disconnect' }));
    expect(onDisconnect).toHaveBeenCalledOnce();
  });
});
