// Package sites holds the compiled-in default site-rule seed list,
// the SitesService interface that the HTTP handlers consume, and the
// concrete Service / Repository implementations.
//
// On user registration, [Service.SeedSiteRulesForUser] copies
// [Defaults] into the site_rule table for that user. After that, the
// DB is the source of truth — defaults are NEVER re-applied
// retroactively if this file changes. Operators who want to ship a
// new common rule add it here for FUTURE users; existing users add it
// via POST /sites/rules.
//
// There is no admin "re-seed defaults" endpoint. If seeding fails on
// either the /auth/register or env-var-bootstrap path the user
// account still exists; they can add rules manually via the API.
package sites

import "github.com/enable-it/nextchapter/backend/internal/models"

// Defaults is the compiled-in seed list. Order is irrelevant. Each
// entry must compile as a Go regexp and the named groups referenced
// by SlugCaptureGroup / ChapterCaptureGroup must exist in the
// pattern — defaults_test.go pins both at build time.
//
// Capture-group names use letters + digits only (no underscores) to
// satisfy validator/v10's `alphanum` tag at the wire layer, so that
// a default that round-trips through POST /sites/rules can be
// reproduced from the client.
var Defaults = []models.SiteRuleNew{
	{
		Host:                "reader.example.com",
		ChapterURLRegex:     `^/series/(?P<slug>[^/]+)/chapter-(?P<chapter>[0-9]+(?:\.[0-9]+)?)$`,
		SlugCaptureGroup:    "slug",
		ChapterCaptureGroup: "chapter",
	},
	{
		Host:                "comics.example.org",
		ChapterURLRegex:     `^/comic/(?P<slug>[^/]+)/(?P<chapter>[^/]+-chapter-[0-9]+(?:\.[0-9]+)?)$`,
		SlugCaptureGroup:    "slug",
		ChapterCaptureGroup: "chapter",
	},
}
