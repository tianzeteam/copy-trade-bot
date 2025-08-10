package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ParamSchemeController 参数方案控制器
type ParamSchemeController struct {
	SchemeService *ParamSchemeService
}

// NewParamSchemeController 创建新的参数方案控制器
func NewParamSchemeController(schemeService *ParamSchemeService) *ParamSchemeController {
	return &ParamSchemeController{
		SchemeService: schemeService,
	}
}

// RegisterRoutesWithAuth 注册需要认证的路由
func (c *ParamSchemeController) RegisterRoutesWithAuth(router *gin.Engine) {
	// 创建需要认证的API组
	api := router.Group("/api/param-schemes")
	api.Use(AuthMiddleware())
	{
		// 获取所有参数方案
		api.GET("", c.GetAllSchemes)
		// 获取单个参数方案
		api.GET("/:id", c.GetScheme)
		// 创建参数方案
		api.POST("", c.CreateScheme)
		// 更新参数方案
		api.PUT("/:id", c.UpdateScheme)
		// 删除参数方案
		api.DELETE("/:id", c.DeleteScheme)
		// 获取默认参数方案
		api.GET("/default", c.GetDefaultScheme)
		// 设置默认参数方案
		api.POST("/default/:id", c.SetDefaultScheme)
	}
}

// SchemeRequest 参数方案请求结构体
type SchemeRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`

	// 交易参数
	SellDelayTime int `json:"sell_delay_time"`

	// 交易触发和池子大小参数
	MinFollowTriggerAmount float64 `json:"min_follow_trigger_amount"`
	MaxFollowTriggerAmount float64 `json:"max_follow_trigger_amount"`
	MinPoolSize            float64 `json:"min_pool_size"`
	MaxPoolSize            float64 `json:"max_pool_size"`

	// 交易参数
	MaxBuyAmount         float64 `json:"max_buy_amount"`
	MinBuyAmount         float64 `json:"min_buy_amount"`
	BuySolAmount         float64 `json:"buy_sol_amount"`
	SellPercentage       float64 `json:"sell_percentage"`
	StopLossPercentage   float64 `json:"stop_loss_percentage"`
	TakeProfitPercentage float64 `json:"take_profit_percentage"`

	// 滑点参数
	BuySlippage  float64 `json:"buy_slippage"`
	SellSlippage float64 `json:"sell_slippage"`

	// 交易手续费参数
	BuyPriorityFee  float64 `json:"buy_priority_fee"`
	BuyTipFee       float64 `json:"buy_tip_fee"`
	SellPriorityFee float64 `json:"sell_priority_fee"`
	SellTipFee      float64 `json:"sell_tip_fee"`
}

// GetAllSchemes 获取所有参数方案
func (c *ParamSchemeController) GetAllSchemes(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取所有参数方案
	schemes, err := c.SchemeService.ListSchemes(context.Background())
	if err != nil {
		logger.Errorf("获取所有参数方案失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取参数方案列表失败"})
		return
	}

	ctx.JSON(http.StatusOK, schemes)
}

// GetScheme 获取单个参数方案
func (c *ParamSchemeController) GetScheme(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取ID参数
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数方案ID不能为空"})
		return
	}

	// 获取参数方案
	scheme, err := c.SchemeService.GetScheme(context.Background(), id)
	if err != nil {
		logger.Errorf("获取参数方案失败: %v", err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "参数方案不存在"})
		return
	}

	ctx.JSON(http.StatusOK, scheme)
}

// CreateScheme 创建参数方案
func (c *ParamSchemeController) CreateScheme(ctx *gin.Context) {
	// 解析请求体
	var request SchemeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 验证必填字段
	if request.Name == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数方案名称不能为空"})
		return
	}

	// 生成ID（如果未提供）
	if request.ID == "" {
		request.ID = uuid.New().String()
	}

	// 创建参数方案
	scheme := &types.ParamScheme{
		ID:                     request.ID,
		Name:                   request.Name,
		Description:            request.Description,
		IsDefault:              request.IsDefault,
		SellDelayTime:          request.SellDelayTime,
		MinFollowTriggerAmount: request.MinFollowTriggerAmount,
		MaxFollowTriggerAmount: request.MaxFollowTriggerAmount,
		MinPoolSize:            request.MinPoolSize,
		MaxPoolSize:            request.MaxPoolSize,
		MaxBuyAmount:           request.MaxBuyAmount,
		MinBuyAmount:           request.MinBuyAmount,
		BuySolAmount:           request.BuySolAmount,
		SellPercentage:         request.SellPercentage,
		StopLossPercentage:     request.StopLossPercentage,
		TakeProfitPercentage:   request.TakeProfitPercentage,
		BuySlippage:            request.BuySlippage,
		SellSlippage:           request.SellSlippage,
		BuyPriorityFee:         request.BuyPriorityFee,
		BuyTipFee:              request.BuyTipFee,
		SellPriorityFee:        request.SellPriorityFee,
		SellTipFee:             request.SellTipFee,
	}

	// 保存参数方案
	err := c.SchemeService.CreateScheme(context.Background(), scheme)
	if err != nil {
		logger.Errorf("保存参数方案失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "保存参数方案失败"})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusCreated, gin.H{"message": "参数方案创建成功", "id": scheme.ID})
}

// UpdateScheme 更新参数方案
func (c *ParamSchemeController) UpdateScheme(ctx *gin.Context) {
	// 获取ID参数
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数方案ID不能为空"})
		return
	}

	// 解析请求体
	var request SchemeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 确保ID匹配
	if id != request.ID && request.ID != "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "路径中的ID与请求体中的ID不匹配"})
		return
	}

	// 获取现有参数方案
	existingScheme, err := c.SchemeService.GetScheme(context.Background(), id)
	if err != nil {
		logger.Errorf("获取参数方案失败: %v", err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "参数方案不存在"})
		return
	}

	// 更新参数方案字段
	existingScheme.Name = request.Name
	existingScheme.Description = request.Description
	existingScheme.IsDefault = request.IsDefault
	existingScheme.SellDelayTime = request.SellDelayTime
	existingScheme.MinFollowTriggerAmount = request.MinFollowTriggerAmount
	existingScheme.MaxFollowTriggerAmount = request.MaxFollowTriggerAmount
	existingScheme.MinPoolSize = request.MinPoolSize
	existingScheme.MaxPoolSize = request.MaxPoolSize
	existingScheme.MaxBuyAmount = request.MaxBuyAmount
	existingScheme.MinBuyAmount = request.MinBuyAmount
	existingScheme.BuySolAmount = request.BuySolAmount
	existingScheme.SellPercentage = request.SellPercentage
	existingScheme.StopLossPercentage = request.StopLossPercentage
	existingScheme.TakeProfitPercentage = request.TakeProfitPercentage
	existingScheme.BuySlippage = request.BuySlippage
	existingScheme.SellSlippage = request.SellSlippage
	existingScheme.BuyPriorityFee = request.BuyPriorityFee
	existingScheme.BuyTipFee = request.BuyTipFee
	existingScheme.SellPriorityFee = request.SellPriorityFee
	existingScheme.SellTipFee = request.SellTipFee
	existingScheme.UpdatedAt = time.Now()

	// 保存更新后的参数方案
	err = c.SchemeService.UpdateScheme(context.Background(), existingScheme)
	if err != nil {
		logger.Errorf("更新参数方案失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "更新参数方案失败"})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{"message": "参数方案更新成功", "id": id})
}

// DeleteScheme 删除参数方案
func (c *ParamSchemeController) DeleteScheme(ctx *gin.Context) {
	// 获取ID参数
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数方案ID不能为空"})
		return
	}

	// 删除参数方案
	err := c.SchemeService.DeleteScheme(context.Background(), id)
	if err != nil {
		logger.Errorf("删除参数方案失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "删除参数方案失败: " + err.Error()})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{"message": "参数方案删除成功", "id": id})
}

// GetDefaultScheme 获取默认参数方案
func (c *ParamSchemeController) GetDefaultScheme(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取默认参数方案
	scheme, err := c.SchemeService.GetDefaultScheme(context.Background())
	if err != nil {
		logger.Errorf("获取默认参数方案失败: %v", err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "默认参数方案不存在"})
		return
	}

	ctx.JSON(http.StatusOK, scheme)
}

// SetDefaultScheme 设置默认参数方案
func (c *ParamSchemeController) SetDefaultScheme(ctx *gin.Context) {
	// 获取ID参数
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数方案ID不能为空"})
		return
	}

	// 获取参数方案
	scheme, err := c.SchemeService.GetScheme(context.Background(), id)
	if err != nil {
		logger.Errorf("获取参数方案失败: %v", err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "参数方案不存在"})
		return
	}

	// 设置为默认方案
	scheme.IsDefault = true
	err = c.SchemeService.UpdateScheme(context.Background(), scheme)
	if err != nil {
		logger.Errorf("设置默认参数方案失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "设置默认参数方案失败"})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{"message": "默认参数方案设置成功", "id": id})
}

// InitDefaultScheme 初始化默认参数方案（如果不存在）
func (c *ParamSchemeController) InitDefaultScheme() error {
	ctx := context.Background()

	// 检查是否已存在默认方案
	_, err := c.SchemeService.GetDefaultScheme(ctx)
	if err == nil {
		// 已存在默认方案，无需创建
		return nil
	}

	// 从全局配置创建默认方案
	config := global.GetConfig()
	_, err = c.SchemeService.CreateDefaultSchemeFromConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("创建默认参数方案失败: %w", err)
	}

	logger.Info("成功创建默认参数方案")
	return nil
}
