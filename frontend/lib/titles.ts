// Pure display helpers — no I/O, unit tested without a browser.

/** "the-beginning-after-the-end" -> "The Beginning After The End". */
export function prettifySlug(slug: string): string {
  return slug
    .split(/[-_]+/)
    .filter((word) => word !== '')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

/**
 * Suggest a series title for the "create new series" row from the page title:
 * cut site-name suffixes at the first separator, drop a trailing
 * "Chapter 101"-style fragment, fall back to the prettified slug.
 */
export function suggestSeriesTitle(tabTitle: string, slug: string): string {
  const beforeSeparator = tabTitle.split(/\s+[|–—-]\s+/)[0] ?? '';
  const withoutChapter = beforeSeparator
    .replace(/\s*(?:chapter|ch\.?|episode|ep\.?)\s*\d+(?:\.\d+)?\s*$/i, '')
    .trim();
  return withoutChapter !== '' ? withoutChapter : prettifySlug(slug);
}
