package web

import (
	"github.com/gin-gonic/gin"
)

// setNoCacheHeaders 设置无缓存头
func setNoCacheHeaders(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("Expires", "0")
}
