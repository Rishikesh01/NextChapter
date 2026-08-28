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
import { isFresh } from '../../lib/rules-cache';
import { refreshRulesCache } from '../../lib/rules-sync';
import { detectPosition, normalizeHost } from '../../lib/url-detection';
import { getCaptureTab, type CaptureTab } from '../../lib/tabs';
import { prettifySlug, suggestSeriesTitle } from '../../lib/titles';
import {
  buildRule,
  canOfferRuleBuilder,
  guessDraft,
  pathSegments,
  previewRule,
  type RuleDraft,
} from '../../lib/rule-builder';
import type { DetectedPosition } from '../../lib/url-detection';
import { PopupHeader } from '../../components/popup/PopupHeader';
import { EmptyState } from '../../components/popup/EmptyState';
import { NotConfigured } from '../../components/popup/NotConfigured';
import { DetectedCapture } from '../../components/popup/DetectedCapture';
import { ManualCaptureForm } from '../../components/popup/ManualCaptureForm';
import { SeriesPicker } from '../../components/popup/SeriesPicker';
import { StatusBanner } from '../../components/popup/StatusBanner';
import { RuleBuilder } from '../../components/popup/RuleBuilder';
import { CoverPicker } from '../../components/popup/CoverPicker';
import {
  fetchCoverBlob,
  findCoverCandidates,
  type CoverCandidate,
} from '../../lib/covers';
import { parseChapterInput } from '../../components/popup/ChapterInput';

type View =
  | { kind: 'loading' }
  | { kind: 'not-configured' }
  | { kind: 'uncapturable' }
  | { kind: 'capture'; tab: CaptureTab; detected: boolean }
  | { kind: 'pick-series'; payload: EntryCapture; suggestedTitle: string }
  | { kind: 'done'; created: boolean; title: string; chapter: number }
  // Cover flow (ADR-0011): choose the series, then pick one of the images
  // actually present on the page the user is looking at.
  | { kind: 'cover-series'; tab: CaptureTab }
  | { kind: 'cover-image'; tab: CaptureTab; series: SeriesSummary }
  | { kind: 'cover-done'; title: string };

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
  /** Non-null while the inline rule builder replaces the manual form. */
  const [ruleDraft, setRuleDraft] = useState<RuleDraft | null>(null);
  const [candidates, setCandidates] = useState<CoverCandidate[]>([]);
  const [candidatesLoading, setCandidatesLoading] = useState(false);
  /** URL of the candidate currently being fetched + uploaded. */
  const [pendingCover, setPendingCover] = useState<string | null>(null);

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
        if (cache === undefined) {
          try {
            cache = await refreshRulesCache();
          } catch {
            // No rules available — manual capture still works.
          }
        } else {
          refreshRulesCache().catch(() => undefined); // keep the stale cache on failure
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

  // ---- rule builder (ADR-0009) ----

  const openRuleBuilder = () => {
    if (view.kind !== 'capture') return;
    const segments = pathSegments(view.tab.url);
    const draft = segments !== null ? guessDraft(segments) : null;
    if (draft !== null) setRuleDraft(draft);
  };

  // Capture with what the (just-saved or refetched) rule detects, exactly as
  // a future auto-detection would.
  const captureDetected = (position: DetectedPosition) => {
    if (view.kind !== 'capture') return;
    setRuleDraft(null);
    setSlug(position.seriesSlug);
    setChapter(String(position.chapter));
    setView({ kind: 'capture', tab: view.tab, detected: true });
    submitCapture(
      {
        site_host: position.siteHost,
        series_slug: position.seriesSlug,
        site_title: view.tab.title,
        chapter: position.chapter,
        url: view.tab.url,
      },
      prettifySlug(position.seriesSlug),
    );
  };

  const saveRule = () => {
    if (view.kind !== 'capture' || ruleDraft === null) return;
    const url = view.tab.url;
    const rule = buildRule(url, ruleDraft);
    const position = rule !== null ? previewRule(url, rule) : null;
    if (rule === null || position === null) return; // Save is disabled anyway

    setBusy(true);
    setBanner(null);
    void (async () => {
      try {
        const created = await client.createSiteRule(rule);
        // Write-through: insert the 201 body directly — a refetch could fail
        // right after a successful create and leave a fresh-looking cache
        // without the new rule (design/flows/rules.md). A background refetch
        // still reconciles trackedHosts etc. when it can.
        const cache = await getRulesCache();
        await setRulesCache({
          rules: [...(cache?.rules ?? []), created],
          trackedHosts: cache?.trackedHosts ?? [],
          fetchedAt: cache?.fetchedAt ?? 0,
        });
        refreshRulesCache().catch(() => undefined);
        setBusy(false);
        captureDetected(position);
      } catch (err) {
        setBusy(false);
        if (
          err instanceof ApiError &&
          err.status === 422 &&
          err.fields?.host !== undefined
        ) {
          // A host-field 422 is either "this host already has a rule" (our
          // cache was stale) or the host failed server validation. Refetch:
          // if the server's rule detects this page, capture with it; either
          // way the builder closes back to the manual form
          // (design/flows/capture.md §5a).
          const cache = await refreshRulesCache().catch(() => undefined);
          const detected =
            cache !== undefined ? detectPosition(cache.rules, url) : null;
          if (detected !== null) {
            captureDetected(detected);
            return;
          }
          setRuleDraft(null);
          const pageHost = normalizeHost(new URL(url).hostname);
          const hostHasRule =
            cache?.rules.some(
              (existing) =>
                existing.host !== undefined &&
                normalizeHost(existing.host) === pageHost,
            ) ?? false;
          setBanner({
            kind: 'error',
            text: hostHasRule
              ? 'A rule for this site already exists — manage it in settings.'
              : `Couldn't save the rule: ${err.fields.host}`,
          });
          return;
        }
        if (err instanceof ApiError && err.unauthorized) {
          setBanner({
            kind: 'error',
            text: 'Token rejected —',
            action: { label: 'open settings', run: openSettings },
          });
        } else if (err instanceof ApiError && err.status === 0) {
          setBanner({
            kind: 'warn',
            text: 'Could not reach your server.',
            action: { label: 'Retry', run: saveRule },
          });
        } else {
          setBanner({
            kind: 'error',
            text: err instanceof ApiError ? err.message : String(err),
          });
        }
      }
    })();
  };

  /**
   * Entry point for the cover flow. Starts the page scan immediately and
   * loads the series list in parallel — the user picks the series while
   * the images are still being enumerated.
   */
  const startCoverFlow = useCallback(
    (tab: CaptureTab) => {
      setBanner(null);
      setView({ kind: 'cover-series', tab });
      void loadSeries();
      setCandidatesLoading(true);
      setCandidates([]);
      void (async () => {
        try {
          setCandidates(await findCoverCandidates(tab.id));
        } catch {
          setCandidates([]);
        } finally {
          setCandidatesLoading(false);
        }
      })();
    },
    [loadSeries],
  );

  /**
   * Fetches the chosen image's bytes and stores them against the series.
   * The fetch happens on the page (or, failing that, behind a host
   * permission the user grants) — never on the server, which has no
   * outbound HTTP at all (ADR-0011 §1).
   */
  const pickCover = useCallback(
    (tab: CaptureTab, target: SeriesSummary, candidate: CoverCandidate) => {
      void (async () => {
        if (target.id === undefined) return;
        setPendingCover(candidate.url);
        setBanner(null);
        try {
          const blob = await fetchCoverBlob(tab.id, candidate.url);
          if (blob === null) {
            setBanner({
              kind: 'warn',
              text: 'Could not read that image — try another.',
            });
            return;
          }
          await client.setSeriesCover(target.id, blob, tab.url);
          setView({
            kind: 'cover-done',
            title: target.title ?? 'this series',
          });
        } catch (err) {
          if (err instanceof ApiError && err.unauthorized) {
            setBanner({
              kind: 'error',
              text: 'Token rejected —',
              action: { label: 'open settings', run: openSettings },
            });
          } else {
            setBanner({ kind: 'warn', text: 'Could not save that cover.' });
          }
        } finally {
          setPendingCover(null);
        }
      })();
    },
    [],
  );

  const host = (() => {
    switch (view.kind) {
      case 'capture':
        return normalizeHost(new URL(view.tab.url).hostname);
      case 'pick-series':
        return view.payload.site_host;
      case 'cover-series':
      case 'cover-image':
        return normalizeHost(new URL(view.tab.url).hostname);
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
      {view.kind === 'capture' && (
        <div className="nc-cover-entry">
          <button
            className="nc-btn-link"
            type="button"
            onClick={() => {
              startCoverFlow(view.tab);
            }}
          >
            Set a series cover from this page
          </button>
        </div>
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
        ) : ruleDraft !== null ? (
          <RuleBuilder
            segments={pathSegments(view.tab.url) ?? []}
            draft={ruleDraft}
            preview={(() => {
              const rule = buildRule(view.tab.url, ruleDraft);
              return rule !== null ? previewRule(view.tab.url, rule) : null;
            })()}
            busy={busy}
            onDraftChange={setRuleDraft}
            onSave={saveRule}
            onBack={() => {
              setRuleDraft(null);
            }}
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
            onCreateRule={
              canOfferRuleBuilder(view.tab.url) ? openRuleBuilder : undefined
            }
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
      {view.kind === 'cover-series' && (
        <SeriesPicker
          seriesSlug="cover"
          siteHost={new URL(view.tab.url).hostname}
          suggestedTitle=""
          hideCreate
          heading="Set a cover for…"
          series={series}
          loading={seriesLoading}
          busy={busy}
          onPick={(picked) => {
            setView({ kind: 'cover-image', tab: view.tab, series: picked });
          }}
          onCreate={() => undefined}
        />
      )}
      {view.kind === 'cover-image' && (
        <CoverPicker
          seriesTitle={view.series.title ?? 'this series'}
          candidates={candidates}
          loading={candidatesLoading}
          pendingUrl={pendingCover}
          onPick={(candidate) => {
            pickCover(view.tab, view.series, candidate);
          }}
          onCancel={() => {
            setView({ kind: 'cover-series', tab: view.tab });
          }}
        />
      )}
      {view.kind === 'cover-done' && (
        <div className="nc-popup-banner nc-popup-banner-only">
          <StatusBanner kind="success">
            Cover saved for <strong>{view.title}</strong>
          </StatusBanner>
        </div>
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
