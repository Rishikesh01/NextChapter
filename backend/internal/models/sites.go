package models

import "time"

// SiteRule is a per-user record telling the browser extension how to
// recognise a chapter URL on a specific site and extract the series
// slug + chapter number from the path. UNIQUE per (user, host).
//
// On registration the server seeds [internal/sites].Defaults into this
// table for the new user; after that the row set is owned by the user
// — defaults are never re-applied retroactively when the seed list
// changes. Rules are independent of /entries: the server does not gate
// captures on whether a rule exists, the rule is parsing data for the
// extension's client-side detector.
type SiteRule struct {
	ID                  int64     `json:"id"`
	UserID              int64     `json:"-"`
	Host                string    `json:"host"`
	ChapterURLRegex     string    `json:"chapter_url_regex"`
	SlugCaptureGroup    string    `json:"slug_capture_group"`
	ChapterCaptureGroup string    `json:"chapter_capture_group"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// SiteRuleNew is both the POST /sites/rules JSON body and the input
// to the sites service's AddSiteRule method. `hostname` is the
// validator/v10 built-in for RFC-952 syntactic host checks
// (wuxiaworld.com passes; "" / "not a host" fail). The two
// capture-group fields use `alphanum` — letters + digits only, no
// underscores; pick capture-group names accordingly.
type SiteRuleNew struct {
	Host                string `json:"host"                  binding:"required,min=1,max=253,hostname"`
	ChapterURLRegex     string `json:"chapter_url_regex"     binding:"required,min=1,max=1024"`
	SlugCaptureGroup    string `json:"slug_capture_group"    binding:"required,min=1,max=64,alphanum"`
	ChapterCaptureGroup string `json:"chapter_capture_group" binding:"required,min=1,max=64,alphanum"`
}

// SiteRulePatch is both the PATCH /sites/rules/{id} JSON body and the
// input to the sites service's EditSiteRule method. Pointer fields
// use the standard absent/present binary: nil means "leave the column
// alone".
type SiteRulePatch struct {
	Host                *string `json:"host,omitempty"                  binding:"omitempty,min=1,max=253,hostname"`
	ChapterURLRegex     *string `json:"chapter_url_regex,omitempty"     binding:"omitempty,min=1,max=1024"`
	SlugCaptureGroup    *string `json:"slug_capture_group,omitempty"    binding:"omitempty,min=1,max=64,alphanum"`
	ChapterCaptureGroup *string `json:"chapter_capture_group,omitempty" binding:"omitempty,min=1,max=64,alphanum"`
}

// SiteList is the GET /sites wire envelope. Rules is the per-user
// site-rule set; TrackedHosts is the distinct site_host values that
// appear on the caller's entry rows. The two fields are independent
// — a host can be tracked (entries exist) without a rule, and a rule
// can exist without any captured entries.
type SiteList struct {
	Rules        []SiteRule `json:"rules"`
	TrackedHosts []string   `json:"tracked_hosts"`
}
