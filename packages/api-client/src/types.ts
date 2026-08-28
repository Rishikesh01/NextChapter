// Ergonomic aliases over the generated OpenAPI types. These reference the
// generated shapes only — request/response structure is never hand-written.
import type { components } from './generated/api';

type Schemas = components['schemas'];

export type ErrorBody = Schemas['handlers.ErrorBody'];
export type ErrorDetail = Schemas['handlers.ErrorDetail'];
export type APIToken = Schemas['models.APIToken'];
export type Credentials = Schemas['models.Credentials'];
export type Entry = Schemas['models.Entry'];
export type EntryCapture = Schemas['models.EntryCapture'];
export type EntryList = Schemas['models.EntryList'];
export type EntryPatch = Schemas['models.EntryPatch'];
export type Health = Schemas['models.Health'];
export type NewToken = Schemas['models.NewToken'];
export type Registration = Schemas['models.Registration'];
export type Series = Schemas['models.Series'];
export type SeriesDetail = Schemas['models.SeriesDetail'];
export type SeriesList = Schemas['models.SeriesList'];
export type SeriesNew = Schemas['models.SeriesNew'];
export type SeriesPatch = Schemas['models.SeriesPatch'];
export type SeriesSummary = Schemas['models.SeriesSummary'];
export type SiteList = Schemas['models.SiteList'];
export type SiteRule = Schemas['models.SiteRule'];
export type SiteRuleNew = Schemas['models.SiteRuleNew'];
export type SiteRulePatch = Schemas['models.SiteRulePatch'];
export type User = Schemas['models.User'];

export type SeriesStatus = NonNullable<SeriesNew['status']>;
