import { browser, type Browser } from 'wxt/browser';

export interface CaptureTab {
  url: string;
  title: string;
}

function toCaptureTab(tab: Browser.tabs.Tab | undefined): CaptureTab | null {
  // tab.url is only populated when activeTab (or a host permission) grants
  // access; an http(s) check also rules out chrome:// and extension pages.
  if (tab?.url === undefined) return null;
  if (!tab.url.startsWith('http://') && !tab.url.startsWith('https://'))
    return null;
  return { url: tab.url, title: tab.title ?? '' };
}

/**
 * The tab the user wants to capture. In production the popup is not a tab, so
 * the active tab of the last focused window is the page under the toolbar
 * button. In e2e the popup itself is opened as a page — making IT the active
 * tab — so fall back to any active http(s) tab in another window.
 */
export async function getCaptureTab(): Promise<CaptureTab | null> {
  const [focused] = await browser.tabs.query({
    active: true,
    lastFocusedWindow: true,
  });
  const direct = toCaptureTab(focused);
  if (direct !== null) return direct;

  const actives = await browser.tabs.query({ active: true });
  for (const tab of actives) {
    const candidate = toCaptureTab(tab);
    if (candidate !== null) return candidate;
  }
  return null;
}
