import type { CoverCandidate } from '../../lib/covers';

export interface CoverPickerProps {
  /** The series the chosen image will be attached to. */
  seriesTitle: string;
  candidates: CoverCandidate[];
  loading: boolean;
  /** Set while one candidate is being fetched and uploaded. */
  pendingUrl: string | null;
  onPick: (candidate: CoverCandidate) => void;
  onCancel: () => void;
}

/**
 * "Pick this series' cover" — a thumbnail grid of the images actually on
 * the page the user is looking at.
 *
 * The user picks rather than the extension guessing, because guessing is
 * unreliable in a way no heuristic fixes: a chapter page's og:image is
 * often a 1200x630 social card, and a series page's largest image can be
 * an ad banner. Ranking puts the likely cover first; the eye does the rest.
 *
 * Thumbnails render straight from their source URLs — an <img> tag needs
 * no CORS and no host permission, so the grid costs nothing to show. Only
 * the image the user actually chooses gets fetched for its bytes.
 */
export function CoverPicker({
  seriesTitle,
  candidates,
  loading,
  pendingUrl,
  onPick,
  onCancel,
}: CoverPickerProps) {
  return (
    <div>
      <div className="nc-picker-header">
        <h1 className="nc-picker-title">Pick a cover</h1>
        <p className="nc-picker-context nc-small">
          for <strong>{seriesTitle}</strong>
        </p>
      </div>

      {loading ? (
        <p className="nc-list-empty nc-small">Looking for images…</p>
      ) : candidates.length === 0 ? (
        <p className="nc-list-empty nc-small">
          No cover-sized images on this page. Open the series&rsquo; own page —
          the one listing its chapters — and try again.
        </p>
      ) : (
        <ul className="nc-cover-grid" aria-label="Choose a cover image">
          {candidates.map((candidate) => (
            <li key={candidate.url}>
              <button
                className="nc-cover-option"
                type="button"
                disabled={pendingUrl !== null}
                aria-busy={pendingUrl === candidate.url}
                title={candidate.url}
                onClick={() => {
                  onPick(candidate);
                }}
              >
                <img
                  className="nc-cover-option-img"
                  src={candidate.url}
                  alt=""
                  loading="lazy"
                />
                {pendingUrl === candidate.url && (
                  <span className="nc-cover-option-state nc-small">
                    Saving…
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="nc-cover-actions">
        <button
          className="nc-btn-secondary"
          type="button"
          disabled={pendingUrl !== null}
          onClick={onCancel}
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
