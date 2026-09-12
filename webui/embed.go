package webui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var assets embed.FS

var Version = "dev"
var BuildVersion = "dev"

func Register(r *gin.Engine) {
	dist, _ := fs.Sub(assets, "dist")
	files := http.FileServer(http.FS(dist))
	r.GET("/app-config.js", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		c.Header("Content-Type", "application/javascript; charset=utf-8")
		c.String(http.StatusOK, "window.__DOMAINSPRITE__=%q;", Version+"+"+BuildVersion)
	})
	r.GET("/assets/*filepath", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/")
		files.ServeHTTP(c.Writer, c.Request)
	})
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/auth" || strings.HasPrefix(p, "/auth/") || p == "/api" || strings.HasPrefix(p, "/api/") || p == "/certificate" || strings.HasPrefix(p, "/certificate/") || p == "/fast" || strings.HasPrefix(p, "/fast/") || p == "/health" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not Found"})
			return
		}
		b, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.String(http.StatusServiceUnavailable, "Web UI has not been built")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, mime.TypeByExtension(path.Ext("index.html")), b)
	})
}
