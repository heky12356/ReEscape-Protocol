package admin

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"project-yume/internal/config"
	"project-yume/internal/metrics"
	"project-yume/internal/utils"

	"github.com/gin-gonic/gin"
)

func setNoCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func requestIDMiddleware() gin.HandlerFunc {
	header := strings.TrimSpace(config.GetConfig().RequestIDHeader)
	if header == "" {
		header = "X-Request-ID"
	}

	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(header))
		if requestID == "" {
			requestID = utils.NewRequestID("http")
		}

		c.Set("request_id", requestID)
		c.Writer.Header().Set(header, requestID)
		c.Next()
	}
}

func accessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		requestID := requestIDFromContext(c)
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		status := c.Writer.Status()
		latency := time.Since(startedAt)
		statusText := strconv.Itoa(status)

		labels := map[string]string{
			"method": c.Request.Method,
			"path":   path,
			"status": statusText,
		}
		metrics.IncCounter(
			"bot_http_requests_total",
			"Total HTTP requests by path, method, and status.",
			labels,
		)
		metrics.ObserveDuration(
			"bot_http_request_duration",
			"HTTP request duration.",
			latency,
			labels,
		)

		fields := []utils.Field{
			utils.String("request_id", requestID),
			utils.String("method", c.Request.Method),
			utils.String("path", path),
			utils.Int("status", status),
			utils.Duration("latency", latency),
			utils.String("client_ip", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, utils.String("errors", c.Errors.String()))
		}

		if status >= http.StatusInternalServerError {
			utils.Errorw("http request completed", fields...)
			return
		}
		utils.Infow("http request completed", fields...)
	}
}

func requestIDFromContext(c *gin.Context) string {
	if value, ok := c.Get("request_id"); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}

func resolveMetricsPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/metrics"
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}

func ensureDirWritable(path string) error {
	resolved := strings.TrimSpace(path)
	if resolved == "" {
		return fmt.Errorf("path is empty")
	}
	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return err
	}

	file, err := os.CreateTemp(resolved, ".health-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	return file.Close()
}

func latestLogFileName(logDir string) (string, error) {
	files, err := listLogFiles(logDir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	return files[0].Name, nil
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
