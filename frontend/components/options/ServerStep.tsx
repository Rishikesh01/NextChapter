import { useId } from 'react';
import { ConnectionStatus, type ConnectionState } from './ConnectionStatus';

export interface ServerStepProps {
  url: string;
  state: ConnectionState;
  stateDetail?: string;
  onUrlChange: (value: string) => void;
  /** Runs inside the click/submit gesture — it triggers the host-permission prompt. */
  onConnect: () => void;
}

export function ServerStep({
  url,
  state,
  stateDetail,
  onUrlChange,
  onConnect,
}: ServerStepProps) {
  const id = useId();
  const submit = (event: { preventDefault: () => void }) => {
    event.preventDefault();
    onConnect();
  };

  return (
    <section className="nc-section">
      <h2 className="nc-section-title">
        <span className="nc-step">1.</span> Server
      </h2>
      <form className="nc-field" onSubmit={submit}>
        <label htmlFor={id}>Server URL</label>
        <div className="nc-row">
          <input
            className="nc-input"
            id={id}
            type="url"
            placeholder="https://nextchapter.example.com"
            autoComplete="off"
            spellCheck={false}
            value={url}
            onChange={(event) => {
              onUrlChange(event.target.value);
            }}
          />
          <button
            className="nc-btn-secondary"
            type="submit"
            disabled={state === 'checking'}
          >
            Connect
          </button>
        </div>
        <ConnectionStatus state={state} detail={stateDetail} />
      </form>
    </section>
  );
}
