export type ConnectionState = 'unchecked' | 'checking' | 'ok' | 'bad';

export interface ConnectionStatusProps {
  state: ConnectionState;
  /** Failure detail shown in the 'bad' state. */
  detail?: string;
}

const text: Record<ConnectionState, string> = {
  unchecked: 'Not checked yet',
  checking: 'Checking…',
  ok: 'Server reachable',
  bad: 'Could not reach server',
};

export function ConnectionStatus({ state, detail }: ConnectionStatusProps) {
  const cls =
    state === 'ok'
      ? 'nc-status nc-status-ok'
      : state === 'bad'
        ? 'nc-status nc-status-bad'
        : 'nc-status';
  return (
    <p className={cls} role="status">
      <span className="nc-status-dot" />
      {state === 'bad' && detail !== undefined
        ? `${text.bad} — ${detail}`
        : text[state]}
    </p>
  );
}
