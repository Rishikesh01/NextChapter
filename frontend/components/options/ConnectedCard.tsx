export interface ConnectedCardProps {
  username: string;
  serverUrl: string;
  onDisconnect: () => void;
}

export function ConnectedCard({
  username,
  serverUrl,
  onDisconnect,
}: ConnectedCardProps) {
  return (
    <section className="nc-section">
      <div className="nc-connected">
        <div className="nc-connected-info">
          <p className="nc-connected-line nc-status-ok">
            <span className="nc-status-dot" />
            <span>
              Connected as <strong>{username}</strong>
            </span>
          </p>
          <p className="nc-connected-server nc-small">{serverUrl}</p>
        </div>
        <button
          className="nc-btn-secondary nc-btn-danger-quiet"
          type="button"
          onClick={onDisconnect}
        >
          Disconnect
        </button>
      </div>
    </section>
  );
}
