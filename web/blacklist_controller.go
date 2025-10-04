package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"copytrade/logger"
	"copytrade/wallet"

	"github.com/gin-gonic/gin"
)

// BlacklistController 黑名单控制器
type BlacklistController struct {
	BlacklistService *wallet.BlacklistService
}

// NewBlacklistController 创建新的黑名单控制器
func NewBlacklistController(blacklistService *wallet.BlacklistService) *BlacklistController {
	return &BlacklistController{
		BlacklistService: blacklistService,
	}
}

// RegisterRoutesWithAuth 注册需要认证的路由
func (c *BlacklistController) RegisterRoutesWithAuth(router *gin.Engine) {
	api := router.Group("/api/blacklist")
	api.Use(AuthMiddleware())
	{
		// 获取所有黑名单地址
		api.GET("", c.GetAllAddresses)
		// 获取单个黑名单地址
		api.GET("/:address", c.GetAddress)
		// 添加黑名单地址
		api.POST("", c.AddAddress)
		// 批量添加黑名单地址
		api.POST("/batch", c.AddAddressBatch)
		// 更新黑名单地址
		api.PUT("/:address", c.UpdateAddress)
		// 删除黑名单地址
		api.DELETE("/:address", c.DeleteAddress)
		// 清空所有黑名单
		api.DELETE("", c.ClearAll)
	}
}

// BlacklistAddressResponse 黑名单地址响应结构
type BlacklistAddressResponse struct {
	Address   string `json:"address"`
	Note      string `json:"note"`
	AddedAt   int64  `json:"added_at"`
	AddedBy   string `json:"added_by"`
	IsEnabled bool   `json:"is_enabled"`
	Timestamp int64  `json:"timestamp"` // 添加时间戳确保响应唯一性
}

// AddAddressRequest 添加黑名单地址请求
type AddAddressRequest struct {
	Address string `json:"address" binding:"required"`
	Note    string `json:"note"`
}

// AddAddressBatchRequest 批量添加黑名单地址请求
type AddAddressBatchRequest struct {
	Addresses string `json:"addresses" binding:"required"` // 逗号分隔的地址列表
	Note      string `json:"note"`
}

// UpdateAddressRequest 更新黑名单地址请求
type UpdateAddressRequest struct {
	Note      string `json:"note"`
	IsEnabled bool   `json:"is_enabled"`
}

// GetAllAddresses 获取所有黑名单地址
func (c *BlacklistController) GetAllAddresses(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取所有黑名单地址
	addresses, err := c.BlacklistService.GetAllAddresses(context.Background())
	if err != nil {
		logger.Errorf("获取黑名单列表失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取黑名单列表失败"})
		return
	}

	// 转换为响应格式
	response := make([]BlacklistAddressResponse, 0, len(addresses))
	for _, addr := range addresses {
		response = append(response, BlacklistAddressResponse{
			Address:   addr.Address,
			Note:      addr.Note,
			AddedAt:   addr.AddedAt.Unix(),
			AddedBy:   addr.AddedBy,
			IsEnabled: addr.IsEnabled,
			Timestamp: time.Now().Unix(),
		})
	}

	ctx.JSON(http.StatusOK, response)
}

// GetAddress 获取单个黑名单地址
func (c *BlacklistController) GetAddress(ctx *gin.Context) {
	// 设置缓存控制头
	setNoCacheHeaders(ctx)

	// 获取地址参数
	address := ctx.Param("address")
	if address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "地址不能为空"})
		return
	}

	// 获取黑名单地址
	addr, err := c.BlacklistService.GetAddress(context.Background(), address)
	if err != nil {
		logger.Errorf("获取黑名单地址失败: %v", err)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "黑名单地址不存在"})
		return
	}

	// 转换为响应格式
	response := BlacklistAddressResponse{
		Address:   addr.Address,
		Note:      addr.Note,
		AddedAt:   addr.AddedAt.Unix(),
		AddedBy:   addr.AddedBy,
		IsEnabled: addr.IsEnabled,
		Timestamp: time.Now().Unix(),
	}

	ctx.JSON(http.StatusOK, response)
}

// AddAddress 添加黑名单地址
func (c *BlacklistController) AddAddress(ctx *gin.Context) {
	// 解析请求体
	var request AddAddressRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 获取当前用户
	username, exists := ctx.Get("username")
	if !exists {
		username = "system"
	}

	// 添加黑名单地址
	err := c.BlacklistService.AddAddress(context.Background(), request.Address, request.Note, username.(string))
	if err != nil {
		logger.Errorf("添加黑名单地址失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusCreated, gin.H{
		"message": "黑名单地址添加成功",
		"address": request.Address,
	})
}

// AddAddressBatch 批量添加黑名单地址
func (c *BlacklistController) AddAddressBatch(ctx *gin.Context) {
	// 解析请求体
	var request AddAddressBatchRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 获取当前用户
	username, exists := ctx.Get("username")
	if !exists {
		username = "system"
	}

	// 解析地址列表（支持逗号、分号、换行符分隔）
	addressesStr := request.Addresses
	addressesStr = strings.ReplaceAll(addressesStr, ";", ",")
	addressesStr = strings.ReplaceAll(addressesStr, "\n", ",")
	addressesStr = strings.ReplaceAll(addressesStr, "\r", "")
	addresses := strings.Split(addressesStr, ",")

	// 批量添加
	successAddrs, failedAddrs := c.BlacklistService.AddAddressBatch(
		context.Background(),
		addresses,
		request.Note,
		username.(string),
	)

	// 返回结果
	ctx.JSON(http.StatusOK, gin.H{
		"message":        "批量添加完成",
		"success_count":  len(successAddrs),
		"failed_count":   len(failedAddrs),
		"success_addrs":  successAddrs,
		"failed_addrs":   failedAddrs,
	})
}

// UpdateAddress 更新黑名单地址
func (c *BlacklistController) UpdateAddress(ctx *gin.Context) {
	// 获取地址参数
	address := ctx.Param("address")
	if address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "地址不能为空"})
		return
	}

	// 解析请求体
	var request UpdateAddressRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据"})
		return
	}

	// 更新黑名单地址
	err := c.BlacklistService.UpdateAddress(context.Background(), address, request.Note, request.IsEnabled)
	if err != nil {
		logger.Errorf("更新黑名单地址失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{
		"message": "黑名单地址更新成功",
		"address": address,
	})
}

// DeleteAddress 删除黑名单地址
func (c *BlacklistController) DeleteAddress(ctx *gin.Context) {
	// 获取地址参数
	address := ctx.Param("address")
	if address == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "地址不能为空"})
		return
	}

	// 删除黑名单地址
	err := c.BlacklistService.RemoveAddress(context.Background(), address)
	if err != nil {
		logger.Errorf("删除黑名单地址失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{
		"message": "黑名单地址删除成功",
		"address": address,
	})
}

// ClearAll 清空所有黑名单
func (c *BlacklistController) ClearAll(ctx *gin.Context) {
	// 清空所有黑名单
	err := c.BlacklistService.ClearAll(context.Background())
	if err != nil {
		logger.Errorf("清空黑名单失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "清空黑名单失败"})
		return
	}

	// 返回成功响应
	ctx.JSON(http.StatusOK, gin.H{
		"message": "黑名单已清空",
	})
}
