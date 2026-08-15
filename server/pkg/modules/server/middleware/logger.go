package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

type contextKey string

const (
	loggerKey contextKey = "logger"
)

// InjectLogger takes a base logger, enriches it, and sets it in the Gin context.
func InjectLogger(baseLogger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Enrich the logger with data specific to this request
		reqLogger := baseLogger.With(
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"client_ip", c.ClientIP(),
		)

		if id := GetRequestID(c); id != "" {
			reqLogger = reqLogger.With("request_id", id)
		}

		// 2. Inject it into Gin's key/value store
		c.Set(loggerKey, reqLogger)

		c.Next()
	}
}

// GetLogger is a helper function to retrieve the logger safely
func GetLogger(c *gin.Context) *slog.Logger {
	if val, exists := c.Get(loggerKey); exists {
		if logger, ok := val.(*slog.Logger); ok {
			return logger
		}
	}
	// Fallback to the default logger if something goes wrong
	return slog.Default()
}

// Logger returns a Gin middleware that logs each request using slog.
// Logs are emitted after the request completes so status and latency are available.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		attrs := []any{
			"status", status,
			"method", c.Request.Method,
			"path", path,
			"latency", latency,
			"ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		}

		if query != "" {
			attrs = append(attrs, "query", query)
		}

		// Attach request ID if set by another middleware.
		if reqID := GetRequestID(c); reqID != "" {
			attrs = append(attrs, "request_id", reqID)
		}

		// Attach errors written via c.Error().
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case status >= 500:
			slog.Error("request", attrs...)
		case status >= 400:
			slog.Warn("request", attrs...)
		default:
			slog.Info("request", attrs...)
		}
	}
}
