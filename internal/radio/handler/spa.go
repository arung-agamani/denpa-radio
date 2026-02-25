package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// SPAHandler serves the built Svelte frontend. Any path that does not match
// an existing file under webDir falls back to index.html so the client-side
// router can handle the route.
type SPAHandler struct {
	webDir string
}

func NewSPAHandler(webDir string) *SPAHandler {
	return &SPAHandler{webDir: webDir}
}

// Handle is the gin handler function for the SPA fallback route.
func (h *SPAHandler) Handle(c *gin.Context) {
	absWebDir, err := filepath.Abs(h.webDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "server configuration error"})
		return
	}

	reqPath := c.Request.URL.Path
	if reqPath == "/" {
		reqPath = "/index.html"
	}

	cleanPath := filepath.Clean(reqPath)
	filePath := filepath.Join(absWebDir, cleanPath)

	absFilePath, err := filepath.Abs(filePath)
	if err != nil || (!strings.HasPrefix(absFilePath, absWebDir+string(filepath.Separator)) && absFilePath != absWebDir) {
		absFilePath = filepath.Join(absWebDir, "index.html")
	}

	info, err := os.Stat(absFilePath)
	if err == nil && !info.IsDir() {
		http.ServeFile(c.Writer, c.Request, absFilePath)
		return
	}

	indexPath := filepath.Join(absWebDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		slog.Warn("Frontend not built", "webDir", h.webDir)
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusNotFound)
		json.NewEncoder(c.Writer).Encode(map[string]string{
			"status": "error",
			"error":  "Frontend not built. Run 'bun run build' in the web/ directory.",
		})
		return
	}

	http.ServeFile(c.Writer, c.Request, indexPath)
}
