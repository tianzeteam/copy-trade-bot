package web

import (
	"copytrade/global"
	"copytrade/utils"
	"copytrade/wallet"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// ConfigController 配置控制器
type ConfigController struct {
	redisService *wallet.RedisWalletService // 保留Redis服务引用以兼容现有代码
	ConfigPath   string                     // 配置文件路径
}

// NewConfigController 创建新的配置控制器
func NewConfigController(redisService *wallet.RedisWalletService, configPath string) *ConfigController {
	return &ConfigController{
		redisService: redisService,
		ConfigPath:   configPath,
	}
}

// RegisterRoutes 注册路由（不需要认证，仅用于兼容旧代码）
func (c *ConfigController) RegisterRoutes(router *gin.Engine) {
	router.GET("/api/redis-config", c.handleGetConfig)
	router.POST("/api/redis-config", c.handleUpdateConfig)
}

// RegisterRoutesWithAuth 注册需要认证的路由
func (c *ConfigController) RegisterRoutesWithAuth(router *gin.Engine) {
	// 创建需要认证的API组
	api := router.Group("/api")
	api.Use(AuthMiddleware())
	{
		api.GET("/redis-config", c.handleGetConfig)
		api.POST("/redis-config", c.handleUpdateConfig)
	}
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
	var updates map[string]any
	if err := ctx.ShouldBindJSON(&updates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "解析请求失败"})
		return
	}

	// 更新各个字段
	for key, value := range updates {
		global.UpdateConfigField(key, value)
	}

	// 保存配置到文件
	// 加载当前配置
	config, err := utils.LoadConfig(c.ConfigPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("加载配置失败: %v", err)})
		return
	}

	// 只更新capital_recovery_percent
	if capitalRecoveryPercent, ok := updates["capital_recovery_percent"]; ok {
		switch v := capitalRecoveryPercent.(type) {
		case float64:
			config.CapitalRecoveryPercent = v
		case int:
			config.CapitalRecoveryPercent = float64(v)
		case string:
			if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
				config.CapitalRecoveryPercent = floatVal
			}
		}
	}
	// 保留其他配置不变

	// 保存配置
	configData, err := yaml.Marshal(config)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("序列化私钥配置失败: %v", err)})
		return
	}

	if err := os.WriteFile(c.ConfigPath, configData, 0644); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存私钥配置失败: %v", err)})
		return
	}

	// 获取更新后的配置
	updatedConfig := global.GetConfig()

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "配置已更新",
		"config":  updatedConfig,
	})
}
