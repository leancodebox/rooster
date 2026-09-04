package httpapi

import (
	"embed"
	"io/fs"
)

//go:embed all:webdist
var webFiles embed.FS

func WebAssets() fs.FS {
	assets, err := fs.Sub(webFiles, "webdist")
	if err != nil {
		return nil
	}
	return assets
}
