package management

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kooshapari/CLIProxyAPI/v7/pkg/llmproxy/misc"
)

func (h *Handler) DownloadAuthFile(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		c.JSON(400, gin.H{"error": "name must end with .json"})
		return
	}
	full, err := misc.ResolveSafeFilePathInDir(h.cfg.AuthDir, name)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "file not found"})
		} else {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to read file: %v", err)})
		}
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", name))
	c.Data(200, "application/json", data)
}

func (h *Handler) UploadAuthFile(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	ctx := c.Request.Context()
	if file, err := c.FormFile("file"); err == nil && file != nil {
		name := strings.TrimSpace(file.Filename)
		dst, err := misc.ResolveSafeFilePathInDir(h.cfg.AuthDir, name)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid auth file name"})
			return
		}
		if !strings.HasSuffix(strings.ToLower(filepath.Base(dst)), ".json") {
			c.JSON(400, gin.H{"error": "file must be .json"})
			return
		}
		if errSave := c.SaveUploadedFile(file, dst); errSave != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to save file: %v", errSave)})
			return
		}
		data, errRead := os.ReadFile(dst)
		if errRead != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to read saved file: %v", errRead)})
			return
		}
		if errReg := h.registerAuthFromFile(ctx, dst, data); errReg != nil {
			// Path traversal or other validation errors should return 400
			if strings.Contains(errReg.Error(), "escapes") || strings.Contains(errReg.Error(), "traversal") {
				c.JSON(400, gin.H{"error": "invalid auth file path"})
			} else {
				c.JSON(500, gin.H{"error": errReg.Error()})
			}
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
		return
	}
	name := c.Query("name")
	name = strings.TrimSpace(name)
	if name == "" {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(name), ".json") {
		c.JSON(400, gin.H{"error": "name must end with .json"})
		return
	}
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to read body"})
		return
	}
	dst, err := misc.ResolveSafeFilePathInDir(h.cfg.AuthDir, name)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	if errWrite := os.WriteFile(dst, data, 0o600); errWrite != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to write file: %v", errWrite)})
		return
	}
	if err = h.registerAuthFromFile(ctx, dst, data); err != nil {
		// Path traversal or other validation errors should return 400
		if strings.Contains(err.Error(), "escapes") || strings.Contains(err.Error(), "traversal") {
			c.JSON(400, gin.H{"error": "invalid auth file path"})
		} else {
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (h *Handler) DeleteAuthFile(c *gin.Context) {
	if h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}
	ctx := c.Request.Context()
	if all := c.Query("all"); all == "true" || all == "1" || all == "*" {
		entries, err := os.ReadDir(h.cfg.AuthDir)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to read auth dir: %v", err)})
			return
		}
		deleted := 0
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".json") {
				continue
			}
			full, err := misc.ResolveSafeFilePathInDir(h.cfg.AuthDir, name)
			if err != nil {
				c.JSON(500, gin.H{"error": fmt.Sprintf("invalid auth file path: %v", err)})
				return
			}
			if err = os.Remove(full); err == nil {
				if errDel := h.deleteTokenRecord(ctx, full); errDel != nil {
					c.JSON(500, gin.H{"error": errDel.Error()})
					return
				}
				deleted++
				h.disableAuth(ctx, full)
			}
		}
		c.JSON(200, gin.H{"status": "ok", "deleted": deleted})
		return
	}
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	full, err := misc.ResolveSafeFilePathInDir(h.cfg.AuthDir, name)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid name"})
		return
	}
	if err := os.Remove(full); err != nil {
		if os.IsNotExist(err) {
			c.JSON(404, gin.H{"error": "file not found"})
		} else {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to remove file: %v", err)})
		}
		return
	}
	if err := h.deleteTokenRecord(ctx, full); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	h.disableAuth(ctx, full)
	c.JSON(200, gin.H{"status": "ok"})
}
