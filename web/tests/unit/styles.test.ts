import { readFileSync, readdirSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * The SPA styles itself entirely through hand-written `nc-*` classes in
 * `src/styles/app.css` (mirroring design/web/), so a mistyped or renamed
 * class silently renders an unstyled element — nothing in the type checker,
 * ESLint, the prop-driven unit tests, or the role/text-based e2e specs can
 * see it. That is exactly how the whole app shell shipped unstyled once:
 * AppLayout emitted `nc-topnav*` / `nc-page` against a stylesheet that
 * defines `nc-nav*` / `nc-content`.
 *
 * These two tests close that hole statically, in milliseconds: every class
 * the components reference must exist in the CSS, and every class the CSS
 * defines must be referenced by a component.
 */

// Resolved from the Vitest root (web/), not import.meta.url — the jsdom
// environment serves modules over http:, so URL-relative paths do not map
// back onto the filesystem.
const SRC = resolve(process.cwd(), 'src');
const CSS_FILES = [
  join(SRC, 'styles/app.css'),
  resolve(process.cwd(), '../design/tokens.css'),
];

/** Every .ts/.tsx under src/, excluding the stylesheets themselves. */
function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return sourceFiles(path);
    return /\.tsx?$/.test(entry.name) ? [path] : [];
  });
}

/**
 * `nc-*` tokens appearing in component source. Matches bare tokens only:
 * the leading boundary skips CSS custom properties (`--nc-space-3`), which
 * appear in inline `style` props and are not class names.
 */
function classesUsedInSource(): Map<string, string[]> {
  const used = new Map<string, string[]>();
  for (const file of sourceFiles(SRC)) {
    const found =
      readFileSync(file, 'utf8').match(/(?<![-\w])nc-[a-z0-9-]+/g) ?? [];
    for (const name of found) {
      used.set(name, [...(used.get(name) ?? []), file.slice(SRC.length + 1)]);
    }
  }
  return used;
}

/** Class selectors in the stylesheets, with the leading dot trimmed. */
function classesDefinedInCss(): Set<string> {
  const defined = new Set<string>();
  for (const file of CSS_FILES) {
    const found = readFileSync(file, 'utf8').match(/\.nc-[a-z0-9-]+/g) ?? [];
    for (const selector of found) defined.add(selector.slice(1));
  }
  return defined;
}

describe('nc-* class names', () => {
  it('are all defined in the stylesheet', () => {
    const defined = classesDefinedInCss();
    const orphans = [...classesUsedInSource()]
      .filter(([name]) => !defined.has(name))
      .map(
        ([name, files]) =>
          `${name} (used in ${[...new Set(files)].join(', ')})`,
      );

    expect(orphans, 'component classes with no CSS rule').toEqual([]);
  });

  it('are all referenced by a component', () => {
    const used = classesUsedInSource();
    const dead = [...classesDefinedInCss()].filter((name) => !used.has(name));

    expect(dead, 'CSS rules no component references').toEqual([]);
  });
});
