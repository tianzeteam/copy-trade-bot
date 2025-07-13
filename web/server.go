package web

import (
	"copytrade/logger"
	"copytrade/utils"
	"copytrade/wallet"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"copytrade/global"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// WebServer Web服务器
type WebServer struct {
	Router       *gin.Engine
	ConfigPath   string
	RedisService *wallet.RedisWalletService
}

// NewWebServer 创建新的Web服务器
func NewWebServer(configPath string, redisService *wallet.RedisWalletService) *WebServer {
	// 设置Gin为发布模式
	gin.SetMode(gin.ReleaseMode)

	// 创建引擎
	router := gin.Default()

	// 配置CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	return &WebServer{
		Router:       router,
		ConfigPath:   configPath,
		RedisService: redisService,
	}
}

// StartServer 启动Web服务器
func StartServer(configPath string) {
	// 创建Redis服务
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // 默认Redis地址
	}

	redisDB := 0                  // 默认使用DB 0
	redisKeyPrefix := "copytrade" // 默认键前缀

	// 创建Redis服务
	redisService, err := wallet.NewRedisWalletService(redisAddr, redisDB, redisKeyPrefix)
	if err != nil {
		logger.Errorf("创建Redis服务失败: %v", err)
		return
	}

	// 初始化认证系统
	InitAuth()

	// 创建服务器
	server := NewWebServer(configPath, redisService)

	// 注册路由
	server.RegisterRoutes()

	// 启动服务器
	port := 3000
	logger.Infof("启动Web服务器，监听端口 %d...", port)
	if err := server.Router.Run(fmt.Sprintf(":%d", port)); err != nil {
		logger.Errorf("启动Web服务器失败: %v", err)
	}
}

// RegisterRoutes 注册路由
func (s *WebServer) RegisterRoutes() {
	// 获取项目根目录
	rootDir := utils.GetRootDir()
	// 确保web目录存在
	webDir := filepath.Join(rootDir, "web")
	staticDir := filepath.Join(webDir, "static")

	logger.Infof("静态文件目录: %s", staticDir)

	// 静态文件服务
	s.Router.Static("/static", staticDir)
	s.Router.Static("/js", filepath.Join(staticDir, "js"))
	s.Router.Static("/css", filepath.Join(staticDir, "css"))

	// 处理登录页面请求
	s.Router.StaticFile("/login", filepath.Join(staticDir, "login.html"))

	// 处理根路径请求，重定向到登录页面
	s.Router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/login")
	})

	// 处理仪表盘页面请求
	s.Router.StaticFile("/dashboard", filepath.Join(staticDir, "dashboard.html"))

	// 注册认证路由
	RegisterAuthRoutes(s.Router)

	// 需要认证的API路由组
	api := s.Router.Group("/api")
	api.Use(AuthMiddleware())
	{
		// 注册配置API
		api.GET("/config", s.handleGetConfig)
		api.POST("/config", s.handleUpdateConfig)

		// 注册交易设置API
		api.GET("/trade-settings", s.handleGetTradeSettings)
		api.POST("/trade-settings", s.handleUpdateTradeSettings)
	}

	// 注册钱包控制器（需要认证）
	walletController := NewWalletController(s.RedisService)
	walletController.RegisterRoutesWithAuth(s.Router)

	// 注册Redis配置控制器（需要认证）
	configController := NewConfigController(s.RedisService)
	configController.RegisterRoutesWithAuth(s.Router)
}

// handleGetConfig 获取配置
func (s *WebServer) handleGetConfig(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取配置
	config, err := utils.LoadConfig(s.ConfigPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("加载配置失败: %v", err)})
		return
	}

	ctx.JSON(http.StatusOK, config)
}

// handleUpdateConfig 更新配置
func (s *WebServer) handleUpdateConfig(ctx *gin.Context) {
	// 解析请求体
	var updates struct {
		SellMode        string                 `json:"sell_mode"`
		MultiSellLevels []utils.MultiSellLevel `json:"multi_sell_levels"`
	}

	if err := ctx.ShouldBindJSON(&updates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("解析请求失败: %v", err)})
		return
	}

	// 先加载现有配置
	config, err := utils.LoadConfig(s.ConfigPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("加载配置失败: %v", err)})
		return
	}

	// 更新配置
	config.SellMode = updates.SellMode
	config.MultiSellLevels = updates.MultiSellLevels

	// 保存配置
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("序列化配置失败: %v", err)})
		return
	}

	if err := os.WriteFile(s.ConfigPath, configData, 0644); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存配置失败: %v", err)})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "success"})
}

// handleGetTradeSettings 获取交易设置
func (s *WebServer) handleGetTradeSettings(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取配置
	config, err := utils.LoadConfig(s.ConfigPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("加载配置失败: %v", err)})
		return
	}

	// 只返回交易设置相关的配置
	tradeSettings := gin.H{
		"pump_trade_enable":         config.PumpTradeEnable,
		"pump_amm_trade_enable":     config.PumpAmmTradeEnable,
		"raydium_trade_enable":      config.RaydiumTradeEnable,
		"repeat_trade_enable":       config.RepeatTradeEnable,
		"sell_delay_time":           config.SellDelayTime,
		"min_pool_size":             config.MinPoolSize,
		"max_pool_size":             config.MaxPoolSize,
		"min_follow_trigger_amount": config.MinFollowTriggerAmount,
		"max_follow_trigger_amount": config.MaxFollowTriggerAmount,
		"buy_slippage":              config.BuySlippage,
		"buy_priority_fee":          config.BuyPriorityFee,
		"buy_tip_fee":               config.BuyTipFee,
		"buy_sol_amount":            config.BuySolAmount,
		"sell_slippage":             config.SellSlippage,
		"sell_priority_fee":         config.SellPriorityFee,
		"sell_tip_fee":              config.SellTipFee,
	}

	ctx.JSON(http.StatusOK, tradeSettings)
}

// handleUpdateTradeSettings 更新交易设置
func (s *WebServer) handleUpdateTradeSettings(ctx *gin.Context) {

	// 交易设置请求体
	type TradeSettingsRequest struct {
		PumpTradeEnable    bool `json:"pump_trade_enable"`
		PumpAmmTradeEnable bool `json:"pump_amm_trade_enable"`
		RaydiumTradeEnable bool `json:"raydium_trade_enable"`
		RepeatTradeEnable  bool `json:"repeat_trade_enable"`
		SellDelayTime      int  `json:"sell_delay_time"`

		MinPoolSize            float64 `json:"min_pool_size"`
		MaxPoolSize            float64 `json:"max_pool_size"`
		MinFollowTriggerAmount float64 `json:"min_follow_trigger_amount"`
		MaxFollowTriggerAmount float64 `json:"max_follow_trigger_amount"`
		BuySlippage            float64 `json:"buy_slippage"`
		BuyPriorityFee         float64 `json:"buy_priority_fee"`
		BuyTipFee              float64 `json:"buy_tip_fee"`
		BuySolAmount           float64 `json:"buy_sol_amount"`
		SellSlippage           float64 `json:"sell_slippage"`
		SellPriorityFee        float64 `json:"sell_priority_fee"`
		SellTipFee             float64 `json:"sell_tip_fee"`
	}

	var updates TradeSettingsRequest

	if err := ctx.ShouldBindJSON(&updates); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("解析请求失败: %v", err)})
		return
	}

	// 将结构体转换为map以便更新配置
	updateMap := map[string]any{
		"pump_trade_enable":         updates.PumpTradeEnable,
		"pump_amm_trade_enable":     updates.PumpAmmTradeEnable,
		"raydium_trade_enable":      updates.RaydiumTradeEnable,
		"repeat_trade_enable":       updates.RepeatTradeEnable,
		"sell_delay_time":           updates.SellDelayTime,
		"min_pool_size":             updates.MinPoolSize,
		"max_pool_size":             updates.MaxPoolSize,
		"min_follow_trigger_amount": updates.MinFollowTriggerAmount,
		"max_follow_trigger_amount": updates.MaxFollowTriggerAmount,
		"buy_slippage":              updates.BuySlippage,
		"buy_priority_fee":          updates.BuyPriorityFee,
		"buy_tip_fee":               updates.BuyTipFee,
		"buy_sol_amount":            updates.BuySolAmount,
		"sell_slippage":             updates.SellSlippage,
		"sell_priority_fee":         updates.SellPriorityFee,
		"sell_tip_fee":              updates.SellTipFee,
	}

	// 更新各个字段
	for key, value := range updateMap {
		global.UpdateConfigField(key, value)
	}

	// 获取更新后的配置
	updatedConfig := global.GetConfig()

	logger.Infof("更新后的配置: %v", updatedConfig)

	// 加载当前配置
	config, err := utils.LoadConfig(s.ConfigPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("加载配置失败: %v", err)})
		return
	}

	// 更新配置
	config.PumpTradeEnable = updates.PumpTradeEnable
	config.PumpAmmTradeEnable = updates.PumpAmmTradeEnable
	config.RaydiumTradeEnable = updates.RaydiumTradeEnable
	config.RepeatTradeEnable = updates.RepeatTradeEnable
	config.SellDelayTime = updates.SellDelayTime
	config.MinPoolSize = updates.MinPoolSize
	config.MaxPoolSize = updates.MaxPoolSize
	config.MinFollowTriggerAmount = updates.MinFollowTriggerAmount
	config.MaxFollowTriggerAmount = updates.MaxFollowTriggerAmount
	config.BuySlippage = updates.BuySlippage
	config.BuyPriorityFee = updates.BuyPriorityFee
	config.BuyTipFee = updates.BuyTipFee
	config.BuySolAmount = updates.BuySolAmount
	config.SellSlippage = updates.SellSlippage
	config.SellPriorityFee = updates.SellPriorityFee
	config.SellTipFee = updates.SellTipFee

	// 保存配置
	configData, err := yaml.Marshal(config)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("序列化配置失败: %v", err)})
		return
	}

	if err := os.WriteFile(s.ConfigPath, configData, 0644); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存配置失败: %v", err)})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "success"})
}
