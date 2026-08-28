import { EmptyState } from './EmptyState';

export interface NotConfiguredProps {
  onOpenSettings: () => void;
}

export function NotConfigured({ onOpenSettings }: NotConfiguredProps) {
  return (
    <EmptyState
      title="Set up NextChapter"
      text="Connect the extension to your NextChapter server to start capturing chapters."
    >
      <button
        className="nc-btn-primary nc-btn-capture"
        type="button"
        onClick={onOpenSettings}
      >
        Open settings
      </button>
    </EmptyState>
  );
}
