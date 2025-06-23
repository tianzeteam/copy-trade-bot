package web

import (
	"copytrade/global"
	"copytrade/wallet"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ConfigController 配置控制器
type ConfigController struct {
	redisService *wallet.RedisWalletService // 保留Redis服务引用以兼容现有代码
}

// NewConfigController 创建新的配置控制器
func NewConfigController(redisService *wallet.RedisWalletService) *ConfigController {
	return &ConfigController{
		redisService: redisService,
	}
}

// RegisterRoutes 注册路由
func (c *ConfigController) RegisterRoutes(router *gin.Engine) {
	router.GET("/api/redis-config", c.handleGetConfig)
	router.POST("/api/redis-config", c.handleUpdateConfig)
}

// handleGetConfig 获取内存中的配置
func (c *ConfigController) handleGetConfig(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 从全局内存获取配置
	config := global.GetConfig()

	ctx.JSON(http.StatusOK, config)
}

// handleUpdateConfig 更新内存中的配置
func (c *ConfigController) handleUpdateConfig(ctx *gin.Context) {
	// 解析请求体
	var updates map[string]interface{}
	if err := ctx.ShouldBindJSON(&updates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "解析请求失败"})
		return
	}

	// 更新各个字段
	for key, value := range updates {
		global.UpdateConfigField(key, value)
	}

	// 获取更新后的配置
	updatedConfig := global.GetConfig()

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "配置已更新",
		"config":  updatedConfig,
	})
}
