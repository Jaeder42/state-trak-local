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

func Handler() gin.HandlerFunc {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

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
