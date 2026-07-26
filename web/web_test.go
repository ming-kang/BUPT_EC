//go:build !embed_assets

package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestDistServesPlaceholderWithoutEmbedTag(t *testing.T) {
	dist, embedded := Dist()
	if embedded {
		t.Fatal("Dist() reported embedded assets in a build without the embed_assets tag")
	}

	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		t.Fatalf("reading index.html from placeholder FS: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("placeholder index.html is empty")
	}
	if !strings.Contains(string(data), "embed_assets") {
		t.Errorf("placeholder index.html should mention embed_assets, got: %s", data)
	}
}
