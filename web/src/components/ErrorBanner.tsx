export function ErrorBanner({ children }: { children: React.ReactNode }) {
  return (
    <div className="nc-banner nc-banner-error" role="alert">
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
        <circle cx="8" cy="11.1" r="0.9" fill="currentColor" />
      </svg>
      <p className="nc-banner-text">{children}</p>
    </div>
  );
}
