package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recover returns a Gin middleware that catches panics, logs the stack trace,
// and responds with a 500 instead of crashing the process.
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				GetLogger(c).Error("panic recovered",
					"error", err,
					"stack", string(stack),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Oh no! An internal server error occurred",
				})
			}
		}()

		c.Next()
	}
}
