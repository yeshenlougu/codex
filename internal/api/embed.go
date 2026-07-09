package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web-dist
var webDist embed.FS

// WebFS returns the embedded frontend filesystem (rooted at web-dist).
func WebFS() http.FileSystem {
	sub, err := fs.Sub(webDist, "web-dist")
	if err != nil {
		panic("embedded web-dist not found: " + err.Error())
	}
	return http.FS(sub)
}
