//go:build embed_assets

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

func distFS() (fs.FS, bool) {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // impossible: dist is the embedded root
	}
	return sub, true
}
