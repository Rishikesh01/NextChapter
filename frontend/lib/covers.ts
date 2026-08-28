import { browser } from 'wxt/browser';

/**
 * Cover acquisition (ADR-0011).
 *
 * The extension is the only component that ever fetches an image: it is
 * already on the page the user is looking at, so its request carries that
 * page's cookies and the server never has to dereference a user-supplied
 * URL. Everything here runs behind an explicit user gesture.
 */

/** One image the page offers as a possible cover. */
export interface CoverCandidate {
  url: string;
  /** Rendered/intrinsic size, used for ranking and for the picker's labels. */
  width: number;
  height: number;
  /** True for og:image / twitter:image — the page's own declared artwork. */
  declared: boolean;
}

/** Smallest edge we will even consider; below this it is an icon, not art. */
const MIN_EDGE = 80;
/** Covers are portrait. Anything wider than this is a banner or a spread. */
const MAX_ASPECT = 1.15;
const MAX_CANDIDATES = 24;

/**
 * Runs INSIDE the page (via scripting.executeScript), so it may only use
 * DOM globals and must be fully self-contained — no imports, no closure
 * over module scope.
 *
 * Collects the page's declared artwork (og:image, twitter:image,
 * link[rel=image_src]) plus every <img> big enough to be cover art, and
 * returns them de-duplicated with their natural dimensions.
 */
function collectImagesInPage(): CoverCandidate[] {
  const seen = new Map<string, CoverCandidate>();

  const add = (
    raw: string | null | undefined,
    width: number,
    height: number,
    declared: boolean,
  ) => {
    if (raw === null || raw === undefined || raw === '') return;
    let absolute: string;
    try {
      absolute = new URL(raw, document.baseURI).href;
    } catch {
      return;
    }
    if (!absolute.startsWith('http://') && !absolute.startsWith('https://')) {
      return;
    }
    const existing = seen.get(absolute);
    // Keep the larger measurement, and let a declared source win: the same
    // URL can appear both as og:image (unmeasured) and as a thumbnail.
    if (existing !== undefined) {
      existing.width = Math.max(existing.width, width);
      existing.height = Math.max(existing.height, height);
      existing.declared = existing.declared || declared;
      return;
    }
    seen.set(absolute, { url: absolute, width, height, declared });
  };

  for (const selector of [
    'meta[property="og:image"]',
    'meta[name="og:image"]',
    'meta[property="twitter:image"]',
    'meta[name="twitter:image"]',
  ]) {
    for (const el of document.querySelectorAll(selector)) {
      add(el.getAttribute('content'), 0, 0, true);
    }
  }
  for (const el of document.querySelectorAll('link[rel="image_src"]')) {
    add(el.getAttribute('href'), 0, 0, true);
  }
  for (const img of document.querySelectorAll('img')) {
    add(
      img.currentSrc !== '' ? img.currentSrc : img.src,
      img.naturalWidth,
      img.naturalHeight,
      false,
    );
  }
  return [...seen.values()];
}

/**
 * Orders candidates so the real cover is first.
 *
 * The page's declared artwork leads — but it is not automatically right:
 * a chapter page's og:image is frequently a 1200x630 social card, not the
 * 2:3 portrait art, which is exactly why the user gets to pick. After
 * that, portrait images sort ahead of landscape ones and bigger ahead of
 * smaller. Unmeasured images (dimensions 0, e.g. a meta tag) are kept —
 * they simply cannot be ranked on shape.
 */
export function rankCandidates(candidates: CoverCandidate[]): CoverCandidate[] {
  const measured = candidates.filter(
    (c) => c.width === 0 || c.height === 0 || isPlausibleCover(c),
  );
  return measured
    .slice()
    .sort((a, b) => {
      if (a.declared !== b.declared) return a.declared ? -1 : 1;
      const aPortrait = isPortrait(a) ? 0 : 1;
      const bPortrait = isPortrait(b) ? 0 : 1;
      if (aPortrait !== bPortrait) return aPortrait - bPortrait;
      return b.width * b.height - a.width * a.height;
    })
    .slice(0, MAX_CANDIDATES);
}

function isPortrait(c: CoverCandidate): boolean {
  return c.width > 0 && c.height > 0 && c.width / c.height <= MAX_ASPECT;
}

/** Big enough to be artwork rather than an icon, sprite or tracking pixel. */
function isPlausibleCover(c: CoverCandidate): boolean {
  return c.width >= MIN_EDGE && c.height >= MIN_EDGE;
}

/**
 * Enumerates cover candidates on the given tab.
 *
 * Requires the `scripting` permission plus host access to the tab, which
 * `activeTab` grants for the duration of the user's click on the toolbar
 * button — so this works with no standing host permissions (ADR-0011 §7).
 */
export async function findCoverCandidates(
  tabId: number,
): Promise<CoverCandidate[]> {
  const results = await browser.scripting.executeScript({
    target: { tabId },
    func: collectImagesInPage,
  });
  const found = results[0]?.result;
  return rankCandidates(Array.isArray(found) ? found : []);
}

/**
 * Runs INSIDE the page. Fetches the image with the page's own credentials
 * and returns it as a data: URL, which is the only structured-cloneable
 * way to get bytes back out of executeScript.
 *
 * Doing this in the page rather than from the extension is the whole
 * point: the request is same-origin-ish to the site, carries its cookies,
 * and looks like the page loading its own artwork — which is what defeats
 * the hotlink protection that would break a plain <img src> in the web UI.
 */
async function fetchImageInPage(url: string): Promise<string | null> {
  try {
    const response = await fetch(url, { credentials: 'include' });
    if (!response.ok) return null;
    const blob = await response.blob();
    // 5MiB server cap; bail before spending time on base64.
    if (blob.size > 5 * 1024 * 1024) return null;
    return await new Promise<string | null>((resolve) => {
      const reader = new FileReader();
      reader.onload = () => {
        resolve(typeof reader.result === 'string' ? reader.result : null);
      };
      reader.onerror = () => {
        resolve(null);
      };
      reader.readAsDataURL(blob);
    });
  } catch {
    return null;
  }
}

/**
 * Fetches one candidate's bytes.
 *
 * Tries the page context first — no permission prompt, and the request
 * carries the page's credentials. That fails when the image lives on a
 * CDN that sends no CORS headers (common), so the fallback asks for host
 * permission on the image's own origin and fetches from the extension,
 * where host permissions exempt the request from CORS entirely.
 *
 * Returns null when both routes fail; the caller surfaces that as
 * "couldn't read that image, try another".
 */
export async function fetchCoverBlob(
  tabId: number,
  url: string,
): Promise<Blob | null> {
  const [inPage] = await browser.scripting.executeScript({
    target: { tabId },
    func: fetchImageInPage,
    args: [url],
  });
  const dataUrl = inPage?.result;
  if (typeof dataUrl === 'string' && dataUrl.startsWith('data:')) {
    return dataUrlToBlob(dataUrl);
  }
  return fetchViaHostPermission(url);
}

async function fetchViaHostPermission(url: string): Promise<Blob | null> {
  let origin: string;
  try {
    origin = `${new URL(url).origin}/*`;
  } catch {
    return null;
  }
  // permissions.request must be called from a user gesture; the popup's
  // click handler still counts as one at this point.
  const granted = await browser.permissions.request({ origins: [origin] });
  if (!granted) return null;
  try {
    const response = await fetch(url);
    if (!response.ok) return null;
    return await response.blob();
  } catch {
    return null;
  }
}

/** Decodes a data: URL produced by fetchImageInPage back into bytes. */
export function dataUrlToBlob(dataUrl: string): Blob | null {
  const comma = dataUrl.indexOf(',');
  if (comma === -1) return null;
  const header = dataUrl.slice(5, comma);
  const isBase64 = header.endsWith(';base64');
  const mime = (isBase64 ? header.slice(0, -';base64'.length) : header) || '';
  const payload = dataUrl.slice(comma + 1);
  try {
    if (!isBase64) {
      return new Blob([decodeURIComponent(payload)], { type: mime });
    }
    const binary = atob(payload);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return new Blob([bytes], { type: mime });
  } catch {
    return null;
  }
}
