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
//
// The list is deliberately short. A rule extracts the chapter number
// from the URL path, and most licensed comic platforms are SPA readers
// that keep the chapter in an opaque id, a query parameter, or nowhere
// in the URL at all — none of which this shape can express. Sites that
// do carry a chapter number in the path are the only ones worth
// seeding; everything else is what the rule builder (ADR-0009) is for.
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
		// Wuxiaworld licenses its translations from the original
		// Chinese and Korean publishers. Chapter segments look like
		// "nshba-chapter-45" or "de-book-2-chapter-15";
		// parseChapterNumber takes the LAST numeric run, so the
		// book-prefixed form still yields the chapter.
		Host:                "wuxiaworld.com",
		ChapterURLRegex:     `^/novel/(?P<slug>[^/]+)/(?P<chapter>[^/]+-chapter-[0-9]+(?:\.[0-9]+)?)$`,
		SlugCaptureGroup:    "slug",
		ChapterCaptureGroup: "chapter",
	},
}
