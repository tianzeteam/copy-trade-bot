package web

import (
	"context"
	"net/http"
	"time"

	"copytrade/logger"
	"copytrade/wallet"

	"github.com/gin-gonic/gin"
)

// WalletController 钱包控制器
type WalletController struct {
	RedisService *wallet.RedisWalletService
}

// NewWalletController 创建新的钱包控制器
func NewWalletController(redisService *wallet.RedisWalletService) *WalletController {
	return &WalletController{
		RedisService: redisService,
	}
}

// RegisterRoutes 注册路由
func (c *WalletController) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/wallets")
	{
		// 获取所有钱包
		api.GET("", c.GetAllWallets)
		// 获取单个钱包
		api.GET("/:address", c.GetWallet)
		// 创建钱包
		api.POST("", c.CreateWallet)
		// 更新钱包
		api.PUT("/:address", c.UpdateWallet)
		// 删除钱包
		api.DELETE("/:address", c.DeleteWallet)
	}
}

// WalletResponse 钱包响应结构体
type WalletResponse struct {
	Address       string `json:"address"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	TakeProfitPct int    `json:"take_profit_pct"`
	StopLossPct   int    `json:"stop_loss_pct"`
	ExitTimeSec   int    `json:"exit_time_sec"`
	Enabled       bool   `json:"enabled"`
	Timestamp     int64  `json:"timestamp"` // 添加时间戳确保响应唯一性
}

// GetAllWallets 获取所有钱包
func (c *WalletController) GetAllWallets(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取所有钱包
	wallets, err := c.RedisService.GetAllWallets(context.Background())
	if err != nil {
		logger.Errorf("获取所有钱包失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取钱包列表失败"})
		return
	}

	// 转换为响应格式
	response := make([]WalletResponse, 0, len(wallets))
	for _, w := range wallets {
		response = append(response, WalletResponse{
			Address:       w.Address.String(),
			Name:          w.Name,
			Type:          string(w.Type),
			TakeProfitPct: w.TakeProfitPct,
			StopLossPct:   w.StopLossPct,
			ExitTimeSec:   w.ExitTimeSec,
			Enabled:       true, // 从Redis获取的钱包都是已启用的
			Timestamp:     time.Now().Unix(),
		})
	}

	ctx.JSON(http.StatusOK, response)
}

// GetWallet 获取单个钱包
func (c *WalletController) GetWallet(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取地址参数
	address := ctx.Param("address")
	if address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "钱包地址不能为空"})
		return
	}

	// 获取钱包
	w, err := c.RedisService.GetWallet(context.Background(), address)
	if err != nil {
		logger.Errorf("获取钱包失败: %v", err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "钱包不存在"})
		return
	}

	// 转换为响应格式
	response := WalletResponse{
		Address:       w.Address.String(),
		Name:          w.Name,
		Type:          string(w.Type),
		TakeProfitPct: w.TakeProfitPct,
		StopLossPct:   w.StopLossPct,
		ExitTimeSec:   w.ExitTimeSec,
		Enabled:       true,
		Timestamp:     time.Now().Unix(),
	}

	ctx.JSON(http.StatusOK, response)
}

// CreateWallet 创建钱包
func (c *WalletController) CreateWallet(ctx *gin.Context) {
	// 解析请求体
	var request WalletResponse
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证必填字段
	if request.Address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "钱包地址不能为空"})
		return
	}

	// 创建钱包配置
	walletConfig := wallet.WalletConfig{
		Address:       request.Address,
		Name:          request.Name,
		Type:          request.Type,
		TakeProfitPct: request.TakeProfitPct,
		StopLossPct:   request.StopLossPct,
		ExitTimeSec:   request.ExitTimeSec,
		Enabled:       request.Enabled,
	}

	// 保存钱包
	err := c.RedisService.SaveWallet(context.Background(), &walletConfig)
	if err != nil {
		logger.Errorf("保存钱包失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "保存钱包失败"})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusCreated, gin.H{"message": "钱包创建成功", "address": request.Address})
}

// UpdateWallet 更新钱包
func (c *WalletController) UpdateWallet(ctx *gin.Context) {
	// 获取地址参数
	address := ctx.Param("address")
	if address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "钱包地址不能为空"})
		return
	}

	// 解析请求体
	var request WalletResponse
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 确保地址匹配
	if address != request.Address {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "路径中的地址与请求体中的地址不匹配"})
		return
	}

	// 创建钱包配置
	walletConfig := wallet.WalletConfig{
		Address:       request.Address,
		Name:          request.Name,
		Type:          request.Type,
		TakeProfitPct: request.TakeProfitPct,
		StopLossPct:   request.StopLossPct,
		ExitTimeSec:   request.ExitTimeSec,
		Enabled:       request.Enabled,
	}

	// 更新钱包
	err := c.RedisService.UpdateWallet(context.Background(), &walletConfig)
	if err != nil {
		logger.Errorf("更新钱包失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "更新钱包失败"})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{"message": "钱包更新成功", "address": request.Address})
}

// DeleteWallet 删除钱包
func (c *WalletController) DeleteWallet(ctx *gin.Context) {
	// 获取地址参数
	address := ctx.Param("address")
	if address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "钱包地址不能为空"})
		return
	}

	// 删除钱包
	err := c.RedisService.DeleteWallet(context.Background(), address)
	if err != nil {
		logger.Errorf("删除钱包失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "删除钱包失败"})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{"message": "钱包删除成功", "address": address})
}
