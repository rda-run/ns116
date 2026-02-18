package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:templates all:static all:migrations
var content embed.FS

func TemplateFS() fs.FS {
	return content
}

func MigrationsFS() fs.FS {
	return content
}

func StaticHandler() http.Handler {
	fsys, _ := fs.Sub(content, ".")
	return http.FileServer(http.FS(fsys))
}
