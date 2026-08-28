import { useCallback, useEffect, useState } from 'react';
import { browser } from 'wxt/browser';
import {
  ApiError,
  type EntryCapture,
  type SeriesSummary,
  type SiteRule,
} from '@nextchapter/api-client';
import { extensionApiClient } from '../../lib/api';
import { getRulesCache, getSettings, setRulesCache } from '../../lib/storage';
import { isFresh, toRulesCache } from '../../lib/rules-cache';
import { detectPosition, normalizeHost } from '../../lib/url-detection';
import { getCaptureTab, type CaptureTab } from '../../lib/tabs';
import { prettifySlug, suggestSeriesTitle } from '../../lib/titles';
import { PopupHeader } from '../../components/popup/PopupHeader';
import { EmptyState } from '../../components/popup/EmptyState';
import { NotConfigured } from '../../components/popup/NotConfigured';
import { DetectedCapture } from '../../components/popup/DetectedCapture';
import { ManualCaptureForm } from '../../components/popup/ManualCaptureForm';
import { SeriesPicker } from '../../components/popup/SeriesPicker';
import { StatusBanner } from '../../components/popup/StatusBanner';
import { parseChapterInput } from '../../components/popup/ChapterInput';

type View =
  | { kind: 'loading' }
  | { kind: 'not-configured' }
  | { kind: 'uncapturable' }
  | { kind: 'capture'; tab: CaptureTab; detected: boolean }
  | { kind: 'pick-series'; payload: EntryCapture; suggestedTitle: string }
  | { kind: 'done'; created: boolean; title: string; chapter: number };

interface Banner {
  kind: 'error' | 'warn';
  text: string;
  action?: { label: string; run: () => void };
}

const client = extensionApiClient();

function openSettings() {
  void browser.runtime.openOptionsPage();
}

export function App() {
  const [view, setView] = useState<View>({ kind: 'loading' });
  const [banner, setBanner] = useState<Banner | null>(null);
  const [slug, setSlug] = useState('');
  const [chapter, setChapter] = useState('');
  const [slugError, setSlugError] = useState<string>();
  const [chapterError, setChapterError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [series, setSeries] = useState<SeriesSummary[]>([]);
  const [seriesLoading, setSeriesLoading] = useState(false);

  useEffect(() => {
    void (async () => {
      const settings = await getSettings();
      if (settings === undefined) {
        setView({ kind: 'not-configured' });
        return;
      }
      const tab = await getCaptureTab();
      if (tab === null) {
        setView({ kind: 'uncapturable' });
        return;
      }

      // Stale-while-revalidate: render from cache immediately, refresh in the
      // background when it's past the TTL; the refreshed rules serve the NEXT
      // popup open. Only a cold cache blocks on the network.
      let cache = await getRulesCache();
      if (!isFresh(cache, Date.now())) {
        const refresh = client
          .getSites()
          .then((list) => setRulesCache(toRulesCache(list, Date.now())));
        if (cache === undefined) {
          try {
            await refresh;
            cache = await getRulesCache();
          } catch {
            // No rules available — manual capture still works.
          }
        } else {
          refresh.catch(() => undefined); // keep the stale cache on failure
        }
      }

      const rules: SiteRule[] = cache?.rules ?? [];
      const detected = detectPosition(rules, tab.url);
      if (detected !== null) {
        setSlug(detected.seriesSlug);
        setChapter(String(detected.chapter));
      }
      setView({ kind: 'capture', tab, detected: detected !== null });
    })();
  }, []);

  const handleCaptureError = useCallback((err: unknown, retry: () => void) => {
    if (err instanceof ApiError) {
      if (err.unauthorized) {
        setBanner({
          kind: 'error',
          text: 'Token rejected —',
          action: { label: 'open settings', run: openSettings },
        });
        return false;
      }
      if (err.status === 0) {
        setBanner({
          kind: 'warn',
          text: 'Could not reach your server.',
          action: { label: 'Retry', run: retry },
        });
        return false;
      }
      if (err.fields !== undefined) {
        setSlugError(err.fields.series_slug);
        setChapterError(err.fields.chapter);
        if (
          err.fields.series_slug === undefined &&
          err.fields.chapter === undefined
        ) {
          setBanner({ kind: 'error', text: err.message });
        }
        return false;
      }
      setBanner({ kind: 'error', text: err.message });
      return false;
    }
    setBanner({ kind: 'error', text: String(err) });
    return false;
  }, []);

  const loadSeries = useCallback(async () => {
    setSeriesLoading(true);
    try {
      const collected: SeriesSummary[] = [];
      // No server-side text search (ADR-0008) — fetch pages and filter locally.
      for (let offset = 0; offset < 1000; offset += 200) {
        const page = await client.listSeries({ limit: 200, offset });
        collected.push(...(page.items ?? []));
        if (collected.length >= (page.total ?? 0)) break;
      }
      setSeries(collected);
    } catch {
      setSeries([]); // the picker still offers "create new"
    } finally {
      setSeriesLoading(false);
    }
  }, []);

  const submitCapture = useCallback(
    (payload: EntryCapture, displayTitle: string) => {
      setBusy(true);
      setBanner(null);
      client
        .capture(payload)
        .then(({ entry, created }) => {
          setView({
            kind: 'done',
            created,
            title: displayTitle,
            chapter: entry.last_chapter ?? payload.chapter,
          });
        })
        .catch((err: unknown) => {
          if (err instanceof ApiError && err.needsSeriesBinding) {
            const tabTitle = view.kind === 'capture' ? view.tab.title : '';
            setView({
              kind: 'pick-series',
              payload,
              suggestedTitle: suggestSeriesTitle(tabTitle, payload.series_slug),
            });
            void loadSeries();
            return;
          }
          handleCaptureError(err, () => {
            submitCapture(payload, displayTitle);
          });
        })
        .finally(() => {
          setBusy(false);
        });
    },
    [view, loadSeries, handleCaptureError],
  );

  const capture = useCallback(() => {
    if (view.kind !== 'capture') return;
    const chapterValue = parseChapterInput(chapter);
    const slugValue = slug.trim();
    setSlugError(slugValue === '' ? 'Enter the series slug' : undefined);
    setChapterError(
      chapterValue === null
        ? 'Enter a chapter number like 101 or 45.5'
        : undefined,
    );
    if (slugValue === '' || chapterValue === null) return;

    const payload: EntryCapture = {
      site_host: normalizeHost(new URL(view.tab.url).hostname),
      series_slug: slugValue,
      site_title: view.tab.title,
      chapter: chapterValue,
      url: view.tab.url,
    };
    submitCapture(payload, prettifySlug(slugValue));
  }, [view, slug, chapter, submitCapture]);

  const host = (() => {
    switch (view.kind) {
      case 'capture':
        return normalizeHost(new URL(view.tab.url).hostname);
      case 'pick-series':
        return view.payload.site_host;
      default:
        return 'NextChapter';
    }
  })();

  return (
    <>
      <PopupHeader host={host} onOpenSettings={openSettings} />
      {banner !== null && (
        <div className="nc-popup-banner">
          <StatusBanner
            kind={banner.kind}
            actionLabel={banner.action?.label}
            onAction={banner.action?.run}
          >
            {banner.text}
          </StatusBanner>
        </div>
      )}
      {view.kind === 'not-configured' && (
        <NotConfigured onOpenSettings={openSettings} />
      )}
      {view.kind === 'uncapturable' && (
        <EmptyState
          title="Nothing to capture here"
          text="Open a chapter page, then click the NextChapter button to save your spot."
        />
      )}
      {view.kind === 'capture' &&
        (view.detected ? (
          <DetectedCapture
            seriesTitle={prettifySlug(slug)}
            chapter={chapter}
            chapterError={chapterError}
            busy={busy}
            onChapterChange={setChapter}
            onCapture={capture}
          />
        ) : (
          <ManualCaptureForm
            slug={slug}
            chapter={chapter}
            slugError={slugError}
            chapterError={chapterError}
            busy={busy}
            onSlugChange={setSlug}
            onChapterChange={setChapter}
            onCapture={capture}
          />
        ))}
      {view.kind === 'pick-series' && (
        <SeriesPicker
          seriesSlug={view.payload.series_slug}
          siteHost={view.payload.site_host}
          suggestedTitle={view.suggestedTitle}
          series={series}
          loading={seriesLoading}
          busy={busy}
          onPick={(picked) => {
            submitCapture(
              { ...view.payload, series_id: picked.id },
              picked.title ?? prettifySlug(view.payload.series_slug),
            );
          }}
          onCreate={(title) => {
            submitCapture({ ...view.payload, new_series_title: title }, title);
          }}
        />
      )}
      {view.kind === 'done' && (
        <div className="nc-popup-banner nc-popup-banner-only">
          <StatusBanner kind="success">
            {view.created ? 'Started tracking ' : 'Advanced '}
            <strong>{view.title}</strong>
            {view.created ? ' at ch ' : ' to ch '}
            <strong>{String(view.chapter)}</strong>
          </StatusBanner>
        </div>
      )}
    </>
  );
}
