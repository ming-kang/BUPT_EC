// Package web owns the embedded frontend assets. By default the package
// compiles without frontend build output so a bare clone passes
// go vet/test/build; the real assets under web/dist are embedded only when
// building with -tags embed_assets (see Taskfile / release workflow).
package web

import "io/fs"

// placeholderHTML is served when the binary was built without embed_assets.
const placeholderHTML = `<!DOCTYPE html><html><head><meta charset="utf-8">` +
	`<title>frontend not built</title></head><body>` +
	`<h1>Frontend not built</h1>` +
	`<p>This binary was compiled without -tags embed_assets. ` +
	`Run <code>task build</code> for a complete binary.</p></body></html>`

// Dist returns the frontend asset tree rooted at dist/ (index.html at the
// root) and whether real embedded assets are present. Without the
// embed_assets build tag the tree contains only a placeholder index.html.
func Dist() (fs.FS, bool) { return distFS() }
