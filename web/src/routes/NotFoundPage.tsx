import { Link } from 'react-router';

/**
 * The backend deliberately 200-serves index.html for any unknown GET
 * (ADR-0010 §4), so unknown client routes land here instead of on
 * react-router's unstyled default error screen.
 */
export function NotFoundPage() {
  return (
    <div className="nc-empty">
      <p className="nc-empty-title">There&rsquo;s no page here</p>
      <p className="nc-empty-line nc-small">
        <Link to="/">Back to your library</Link>
      </p>
    </div>
  );
}
