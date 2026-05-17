package sites

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDefaultsCompileAndExposeNamedGroups walks every entry in
// [Defaults] and asserts (a) ChapterURLRegex is a valid Go regexp, and
// (b) the SlugCaptureGroup and ChapterCaptureGroup names actually
// appear as named sub-expressions in the compiled pattern. Catches
// regex typos and capture-group-name typos at build time before the
// seed list ever lands in a real user's table.
func TestDefaultsCompileAndExposeNamedGroups(t *testing.T) {
	r := require.New(t)
	for _, d := range Defaults {
		re, err := regexp.Compile(d.ChapterURLRegex)
		r.NoError(err, "default for host %q: regex must compile", d.Host)

		names := re.SubexpNames()
		has := func(target string) bool {
			for _, n := range names {
				if n == target {
					return true
				}
			}
			return false
		}
		r.True(has(d.SlugCaptureGroup),
			"default for host %q: slug_capture_group %q missing from regex (named groups: %v)",
			d.Host, d.SlugCaptureGroup, names)
		r.True(has(d.ChapterCaptureGroup),
			"default for host %q: chapter_capture_group %q missing from regex (named groups: %v)",
			d.Host, d.ChapterCaptureGroup, names)
	}
}
