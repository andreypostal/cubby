package middleware

import (
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/gin-gonic/gin"
)

// freezeGateKey holds the DoneFunc of the current request
const freezeGateKey = "freeze-gate-leave"

func FreezeGate(f *core.Freezer) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		enter := f.Enter
		if !isMutating(ctx.Request.Method) {
			enter = f.EnterRead
		}

		leave, err := enter()
		if err != nil {
			_ = ctx.AbortWithError(http.StatusLocked, err)
			return
		}
		defer leave()

		ctx.Set(freezeGateKey, leave)

		ctx.Next()
	}
}

// LeaveFreezeGate releases the gate slot this request holds ahead of the handler returning
// The gate releases the slot again on the way out, doing it twice is a no-op
func LeaveFreezeGate(ctx *gin.Context) {
	leave, ok := ctx.Get(freezeGateKey)
	if !ok {
		return
	}

	if done, ok := leave.(core.DoneFunc); ok {
		done()
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}
