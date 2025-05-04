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
	"runtime"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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

	// 创建Gin引擎
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

	redisService, err := wallet.NewRedisWalletService(redisAddr, redisDB, redisKeyPrefix)
	if err != nil {
		logger.Errorf("创建Redis服务失败: %v", err)
		return
	}

	// 创建服务器
	server := NewWebServer(configPath, redisService)

	// 注册路由
	server.RegisterRoutes()

	// 启动服务器
	port := 8081
	logger.Infof("启动Web服务器，监听端口 %d...", port)
	if err := server.Router.Run(fmt.Sprintf(":%d", port)); err != nil {
		logger.Errorf("启动Web服务器失败: %v", err)
	}
}

// RegisterRoutes 注册路由
func (s *WebServer) RegisterRoutes() {
	// 获取项目根目录
	_, b, _, _ := runtime.Caller(0)
	rootDir := filepath.Dir(filepath.Dir(b)) // 回到项目根目录
	
	// 确保web目录存在
	webDir := filepath.Join(rootDir, "web")
	staticDir := filepath.Join(webDir, "static")
	
	logger.Infof("静态文件目录: %s", staticDir)

	// 静态文件服务
	s.Router.Static("/static", staticDir)
	s.Router.StaticFile("/", filepath.Join(staticDir, "index.html"))

	// 注册配置API
	s.Router.GET("/api/config", s.handleGetConfig)
	s.Router.POST("/api/config", s.handleUpdateConfig)

	// 注册钱包控制器
	walletController := NewWalletController(s.RedisService)
	walletController.RegisterRoutes(s.Router)
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
