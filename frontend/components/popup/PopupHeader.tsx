export interface PopupHeaderProps {
  /** Host of the page under capture, or a neutral product label. */
  host: string;
  onOpenSettings: () => void;
}

export function PopupHeader({ host, onOpenSettings }: PopupHeaderProps) {
  return (
    <header className="nc-header">
      <span className="nc-header-host" title={host}>
        {host}
      </span>
      <button
        className="nc-iconbtn"
        type="button"
        aria-label="Settings"
        title="Settings"
        onClick={onOpenSettings}
      >
        <svg
          width="16"
          height="16"
          viewBox="0 0 16 16"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="M8 10.2a2.2 2.2 0 1 0 0-4.4 2.2 2.2 0 0 0 0 4.4Z"
            stroke="currentColor"
            strokeWidth="1.3"
          />
          <path
            d="M13.3 8c0-.4.5-.8.4-1.2-.1-.4-.7-.5-.9-.9-.2-.3 0-.9-.3-1.2-.3-.3-.9-.1-1.2-.3-.4-.2-.5-.8-.9-.9-.4-.1-.8.4-1.2.4h-.4c-.4 0-.8-.5-1.2-.4-.4.1-.5.7-.9.9-.3.2-.9 0-1.2.3-.3.3-.1.9-.3 1.2-.2.4-.8.5-.9.9-.1.4.4.8.4 1.2s-.5.8-.4 1.2c.1.4.7.5.9.9.2.3 0 .9.3 1.2.3.3.9.1 1.2.3.4.2.5.8.9.9.4.1.8-.4 1.2-.4h.4c.4 0 .8.5 1.2.4.4-.1.5-.7.9-.9.3-.2.9 0 1.2-.3.3-.3.1-.9.3-1.2.2-.4.8-.5.9-.9.1-.4-.4-.8-.4-1.2Z"
            stroke="currentColor"
            strokeWidth="1.1"
          />
        </svg>
      </button>
    </header>
  );
}
