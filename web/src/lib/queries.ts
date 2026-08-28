// TanStack Query layer: one key factory, one hook per read, mutations with
// their invalidations. Screens never call the api client directly.
import {
  QueryCache,
  QueryClient,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import {
  ApiError,
  type Credentials,
  type EntryPatch,
  type ListSeriesQuery,
  type NewToken,
  type Registration,
  type SeriesNew,
  type SeriesPatch,
  type SiteRuleNew,
  type SiteRulePatch,
} from '@nextchapter/api-client';
import { api } from './api';

export const queryKeys = {
  me: ['me'] as const,
  series: ['series'] as const,
  seriesList: (filters: ListSeriesQuery) =>
    ['series', 'list', filters] as const,
  seriesDetail: (seriesID: number) => ['series', 'detail', seriesID] as const,
  sites: ['sites'] as const,
};

export function makeQueryClient(): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => {
        // A session that expires mid-use: any data query 401s → back to
        // login. The 'me' probe is excluded — RequireAuth handles it
        // client-side without a full reload.
        if (
          error instanceof ApiError &&
          error.unauthorized &&
          query.queryKey[0] !== 'me'
        ) {
          const next = encodeURIComponent(
            window.location.pathname + window.location.search,
          );
          window.location.assign(`/login?next=${next}`);
        }
      },
    }),
    defaultOptions: {
      queries: {
        staleTime: 15_000,
        retry: (failureCount, error) => {
          if (
            error instanceof ApiError &&
            (error.status === 401 || error.status === 404)
          ) {
            return false;
          }
          return failureCount < 2;
        },
      },
    },
  });
}

// ---- reads ----

export function useMe() {
  return useQuery({
    queryKey: queryKeys.me,
    queryFn: () => api.me(),
    retry: false,
  });
}

export function useSeriesList(filters: ListSeriesQuery) {
  return useQuery({
    queryKey: queryKeys.seriesList(filters),
    queryFn: () => api.listSeries(filters),
  });
}

export const LIBRARY_PAGE_SIZE = 60;

/** The library grid: offset pages accumulated behind a "Load more" button. */
export function useSeriesPages(
  filters: Omit<ListSeriesQuery, 'limit' | 'offset'>,
) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.seriesList(filters), 'pages'] as const,
    queryFn: ({ pageParam }) =>
      api.listSeries({
        ...filters,
        limit: LIBRARY_PAGE_SIZE,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      const fetched = allPages.reduce(
        (count, page) => count + (page.items?.length ?? 0),
        0,
      );
      return fetched < (lastPage.total ?? 0) ? fetched : undefined;
    },
  });
}

export function useSeriesDetail(seriesID: number) {
  return useQuery({
    queryKey: queryKeys.seriesDetail(seriesID),
    queryFn: () => api.getSeries(seriesID),
  });
}

export function useSites() {
  return useQuery({
    queryKey: queryKeys.sites,
    queryFn: () => api.getSites(),
  });
}

// ---- auth mutations ----

export function useLogin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (creds: Credentials) => api.auth.login(creds),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.me }),
  });
}

export function useRegister() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (registration: Registration) => api.auth.register(registration),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.me }),
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.auth.logout(),
    onSuccess: () => {
      queryClient.clear();
    },
  });
}

export function useMintToken() {
  return useMutation({
    mutationFn: (body: NewToken) => api.auth.mintToken(body),
  });
}

// ---- series mutations ----

export function useCreateSeries() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (draft: SeriesNew) => api.createSeries(draft),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.series }),
  });
}

export function usePatchSeries(seriesID: number) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (patch: SeriesPatch) => api.patchSeries(seriesID, patch),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.series }),
  });
}

export function useDeleteSeries() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (seriesID: number) => api.deleteSeries(seriesID),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.series }),
  });
}

/**
 * Removing a cover invalidates the whole series tree: cover_updated_at
 * rides on every summary (it is the grid's existence flag), so the
 * listing and the detail both go stale at once.
 */
export function useDeleteCover() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (seriesID: number) => api.deleteSeriesCover(seriesID),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.series }),
  });
}

// ---- entry mutations (rollups live on series, so invalidate series) ----

export function usePatchEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ entryID, patch }: { entryID: number; patch: EntryPatch }) =>
      api.patchEntry(entryID, patch),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.series }),
  });
}

export function useDeleteEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (entryID: number) => api.deleteEntry(entryID),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.series }),
  });
}

// ---- site-rule mutations ----

export function useCreateRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (rule: SiteRuleNew) => api.createSiteRule(rule),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.sites }),
  });
}

export function usePatchRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ruleID, patch }: { ruleID: number; patch: SiteRulePatch }) =>
      api.patchSiteRule(ruleID, patch),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.sites }),
  });
}

export function useDeleteRule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ruleID: number) => api.deleteSiteRule(ruleID),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: queryKeys.sites }),
  });
}
