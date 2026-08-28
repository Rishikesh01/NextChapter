import type { ReactNode } from 'react';

export type BannerKind = 'success' | 'error' | 'warn';

export interface StatusBannerProps {
  kind: BannerKind;
  children: ReactNode;
  /** Optional inline action (Retry, open settings) — same hue as the banner. */
  actionLabel?: string;
  onAction?: () => void;
}

const bannerClass: Record<BannerKind, string> = {
  success: 'nc-banner nc-banner-success',
  error: 'nc-banner nc-banner-error',
  warn: 'nc-banner nc-banner-warn',
};

function BannerIcon({ kind }: { kind: BannerKind }) {
  if (kind === 'success') {
    return (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
      >
        <circle cx="8" cy="8" r="6.5" stroke="currentColor" strokeWidth="1.3" />
        <path
          d="M5.2 8.2l1.9 1.9 3.7-3.9"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    );
  }
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      aria-hidden="true"
    >
      <circle cx="8" cy="8" r="6.5" stroke="currentColor" strokeWidth="1.3" />
      <path
        d="M8 4.8v3.9"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
      <circle cx="8" cy="11.2" r="0.9" fill="currentColor" />
    </svg>
  );
}

export function StatusBanner({
  kind,
  children,
  actionLabel,
  onAction,
}: StatusBannerProps) {
  return (
    <div
      className={bannerClass[kind]}
      role={kind === 'success' ? 'status' : 'alert'}
    >
      <BannerIcon kind={kind} />
      <p className="nc-banner-text">
        {children}
        {actionLabel !== undefined && (
          <>
            {' '}
            <button
              className="nc-banner-action"
              type="button"
              onClick={onAction}
            >
              {actionLabel}
            </button>
          </>
        )}
      </p>
    </div>
  );
}
