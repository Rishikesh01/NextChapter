// Package webui carries the embedded web library SPA build (ADR-0010).
//
// dist/ holds a committed one-line placeholder index.html so the embed
// always compiles and backend CI stays Go-only; `make -C frontend
// web-embed` overwrites it with the real Vite build for release
// artifacts, and `web-unembed` restores the placeholder. The `all:`
// prefix keeps Vite's underscore-prefixed files included.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded web UI rooted at dist/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Unreachable: "dist" is a compile-time embed directive.
		panic(err)
	}
	return sub
}
