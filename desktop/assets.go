//go:build desktop

package main

import (
	"embed"
	"io/fs"
)

// Assets are staged from the same production web/dist as the headless server.
//
//go:embed all:assets/web
var embedded embed.FS

func FrontendAssets() fs.FS {
	assets, err := fs.Sub(embedded, "assets/web")
	if err != nil {
		panic(err)
	}
	return assets
}
