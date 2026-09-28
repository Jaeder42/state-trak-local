package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var dist embed.FS

// Dist returns the embedded client build (web/dist) as an fs.FS — served by
// the HTTP server (below) and by the Wails desktop app's asset server.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// Handler serves the embedded SPA: known files straight from the FS, and
// index.html for extensionless paths (client-side routing fallback).
func Handler() gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(Dist()))

	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/" || !strings.Contains(path, ".") {
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}
