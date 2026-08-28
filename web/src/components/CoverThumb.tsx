import { useState } from 'react';

export interface CoverThumbProps {
  /** Same-origin cover URL, or null when the series has no cover. */
  src: string | null;
  /** Series title — seeds the placeholder's initial. */
  title: string;
  /** Larger variant for the series detail page. */
  large?: boolean;
}

/**
 * A series cover, or a lettered placeholder when there is none.
 *
 * The placeholder is not decoration: a coverless series still needs a
 * stable, recognisable block so the grid does not reflow into a ragged
 * mess when only some series have art. It also covers the case where a
 * cover exists but fails to load (the row was deleted between the
 * listing and the image request), which onError folds back into the
 * same placeholder rather than a broken-image icon.
 */
export function CoverThumb({ src, title, large }: CoverThumbProps) {
  const [failed, setFailed] = useState(false);
  const className = `nc-cover${large === true ? ' nc-cover-lg' : ''}`;

  if (src === null || failed) {
    // The first character carries more recognition than a generic icon
    // and costs nothing. aria-hidden because the accessible name is
    // already on the surrounding link / heading.
    return (
      <div className={`${className} nc-cover-empty`} aria-hidden="true">
        {title.trim().slice(0, 1).toUpperCase()}
      </div>
    );
  }
  return (
    <img
      className={className}
      src={src}
      alt=""
      loading="lazy"
      decoding="async"
      onError={() => {
        setFailed(true);
      }}
    />
  );
}
