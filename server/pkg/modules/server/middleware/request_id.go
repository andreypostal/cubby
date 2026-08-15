package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// requestIDKey holds the correlation id of the request being served
	requestIDKey    contextKey = "request_id"
	RequestIDHeader            = "X-Request-Id"
	maxRequestIDLen            = 64
)

// RequestID gives every request an id and echoes it back
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := sanitizeRequestID(c.GetHeader(RequestIDHeader))
		if id == "" {
			id = uuid.NewString()
		}

		c.Set(requestIDKey, id)
		c.Header(RequestIDHeader, id)

		c.Next()
	}
}

// GetRequestID returns the id of the request being served
func GetRequestID(c *gin.Context) string {
	if val, exists := c.Get(requestIDKey); exists {
		if id, ok := val.(string); ok {
			return id
		}
	}

	return ""
}

// sanitizeRequestID keeps a caller supplied id only while it stays short and plain
func sanitizeRequestID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > maxRequestIDLen {
		return ""
	}

	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
		default:
			return ""
		}
	}

	return id
}
