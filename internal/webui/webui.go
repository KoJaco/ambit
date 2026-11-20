// Package webui embeds the built frontend bundle. The bundle is produced by
// `make build` (frontend/build/client copied into dist/) and is gitignored, so
// a fresh clone builds and tests without it; Available reports whether it is present.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded bundle rooted at dist.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return dist
	}
	return sub
}

// Available reports whether the bundle contains index.html.
func Available() bool {
	info, err := fs.Stat(FS(), "index.html")
	return err == nil && !info.IsDir()
}
