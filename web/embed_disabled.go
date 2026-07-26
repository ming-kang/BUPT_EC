//go:build !embed_assets

package web

import (
	"io/fs"
	"testing/fstest"
)

func distFS() (fs.FS, bool) {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(placeholderHTML)},
	}, false
}
