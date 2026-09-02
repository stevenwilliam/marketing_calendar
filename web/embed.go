// Package web serves the built single-page application.
//
// The build output is embedded in the binary, so a deployment is one file and
// there is no window in which the API is new and the assets are old.
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

// Mount serves the SPA. Any path that is not an API route falls through to
// index.html so client-side routing works on a refresh.
func Mount(r *gin.Engine) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// No build present: the API still serves. This is the development
		// case, where Vite serves the frontend on its own port.
		return
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "endpoint tidak ditemukan"})
			return
		}
		if _, err := fs.Stat(sub, strings.TrimPrefix(p, "/")); err == nil && p != "/" {
			// A real asset: let the file server set its own content type and
			// cache headers.
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}
