package web

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"

	"copytrade/asset"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/utils"
	"copytrade/wallet"

	"github.com/gagliardetto/solana-go"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// PrivateKeyImportRequest 私钥导入请求结构体
type PrivateKeyImportRequest struct {
	PrivateKey string `json:"private_key" binding:"required"`
	Name       string `json:"name" binding:"required"`
}

// PrivateKeyImportResponse 私钥导入响应结构体
type PrivateKeyImportResponse struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// PrivateKeyWalletResponse 私钥钱包响应结构体
type PrivateKeyWalletResponse struct {
	Address string `json:"address"`
	Name    string `json:"name"`
	Type    string `json:"type"`
}

// WalletImportController 钱包导入控制器
type WalletImportController struct {
	RedisService *wallet.RedisWalletService
	ConfigPath   string
}

// NewWalletImportController 创建新的钱包导入控制器
func NewWalletImportController(redisService *wallet.RedisWalletService, configPath string) *WalletImportController {
	return &WalletImportController{
		RedisService: redisService,
		ConfigPath:   configPath,
	}
}

// RegisterRoutes 注册路由
func (c *WalletImportController) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api/wallets")
	api.Use(AuthMiddleware())
	{
		api.POST("/import-private-key", c.ImportPrivateKey)
		api.GET("/private-keys", c.GetPrivateKeyWallets)
		api.DELETE("/private-key/:address", c.DeletePrivateKeyWallet)
	}
}

// ImportPrivateKey 导入私钥钱包
func (c *WalletImportController) ImportPrivateKey(ctx *gin.Context) {
	var req PrivateKeyImportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证私钥格式
	privateKey, err := c.validateAndParsePrivateKey(req.PrivateKey)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 从私钥生成公钥和地址
	publicKey := privateKey.PublicKey()
	address := publicKey.String()

	_botWallet := solana.Wallet{PrivateKey: privateKey}

	global.UpdateConfigField("trade_wallet_key", _botWallet)
	botWallet := global.GetConfig().TradeWalletKey
	logger.Infof("更新后的botWallet: %s", botWallet.PublicKey().String())
	// 更新钱包监控器
	asset.GetBalanceManager().UpdateWalletAddress(address)
	// 加载当前配置
	config, err := utils.LoadConfig(c.ConfigPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("加载配置失败: %v", err)})
		return
	}

	// 只更新trade_wallet_key
	config.PrivateKey = privateKey.String()
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
	// 返回成功响应
	response := PrivateKeyImportResponse{
		Address: address,
		Name:    req.Name,

		Message: "钱包导入成功",
	}

	ctx.JSON(http.StatusCreated, response)
}

// GetPrivateKeyWallets 获取私钥钱包信息（系统只保存一个私钥）
func (c *WalletImportController) GetPrivateKeyWallets(ctx *gin.Context) {
	// 检查是否存在私钥
	// 加载当前配置
	config, err := utils.LoadConfig(c.ConfigPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("加载配置失败: %v", err)})
		return
	}

	// 获取私钥并生成地址
	privateKeyStr := config.PrivateKey
	if privateKeyStr == "" {
		ctx.JSON(http.StatusOK, []PrivateKeyWalletResponse{})
		return
	}

	privateKey, err := c.validateAndParsePrivateKey(privateKeyStr)
	if err != nil {
		logger.Errorf("解析私钥失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "解析私钥失败"})
		return
	}

	address := privateKey.PublicKey().String()

	// 返回单个私钥钱包信息
	wallet := PrivateKeyWalletResponse{
		Address: address,
		Name:    "默认私钥钱包",
		Type:    "high_frequency",
	}

	ctx.JSON(http.StatusOK, []PrivateKeyWalletResponse{wallet})
}

// DeletePrivateKeyWallet 删除私钥钱包（系统只保存一个私钥）
func (c *WalletImportController) DeletePrivateKeyWallet(ctx *gin.Context) {
	// 删除私钥
	if err := c.deletePrivateKey(); err != nil {
		logger.Errorf("删除私钥失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "删除私钥失败"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "私钥已删除"})
}

// validateAndParsePrivateKey 验证并解析私钥
func (c *WalletImportController) validateAndParsePrivateKey(privateKeyStr string) (solana.PrivateKey, error) {
	privateKeyStr = strings.TrimSpace(privateKeyStr)

	// 检查空字符串
	if privateKeyStr == "" {
		return nil, fmt.Errorf("私钥不能为空")
	}

	// 尝试Base58格式（Solana标准格式）
	if len(privateKeyStr) >= 32 && len(privateKeyStr) <= 128 {
		privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
		if err == nil && privateKey.IsValid() {
			return privateKey, nil
		}
	}

	// 尝试十六进制格式（64位）
	if len(privateKeyStr) == 64 {
		// 检查是否为有效的十六进制字符串
		if _, err := hex.DecodeString(privateKeyStr); err != nil {
			return nil, fmt.Errorf("十六进制私钥格式无效: %v", err)
		}

		// 尝试从十六进制解析
		privateKeyBytes, err := hex.DecodeString(privateKeyStr)
		if err != nil {
			return nil, fmt.Errorf("十六进制私钥解析失败: %v", err)
		}

		if len(privateKeyBytes) != 32 {
			return nil, fmt.Errorf("私钥长度必须为32字节")
		}

		// 创建私钥
		privateKey := solana.PrivateKey(privateKeyBytes)
		if privateKey.IsValid() {
			return privateKey, nil
		}
	}

	// 尝试助记词格式
	if strings.Contains(privateKeyStr, " ") {
		// 检查是否为助记词
		words := strings.Fields(privateKeyStr)
		if len(words) == 12 || len(words) == 24 {
			// 这里可以添加助记词解析逻辑
			return nil, fmt.Errorf("助记词格式暂不支持，请使用Base58或十六进制私钥")
		}
	}

	return nil, fmt.Errorf("不支持的私钥格式，请使用Base58或64位十六进制私钥")
}

// savePrivateKey 保存私钥（加密存储）
func (c *WalletImportController) savePrivateKey(privateKey string) error {
	// 使用固定key存储私钥，系统只保存一个私钥
	key := fmt.Sprintf("%s:private_key", c.RedisService.KeyPrefix)

	// 简单的base64编码存储（生产环境应使用更安全的加密）
	encoded := base64.StdEncoding.EncodeToString([]byte(privateKey))

	return c.RedisService.Client.Set(context.Background(), key, encoded, 0).Err()
}

// getPrivateKey 获取私钥
func (c *WalletImportController) getPrivateKey() (string, error) {
	key := fmt.Sprintf("%s:private_key", c.RedisService.KeyPrefix)

	encoded, err := c.RedisService.Client.Get(context.Background(), key).Result()
	if err != nil {
		return "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

// hasPrivateKey 检查是否存在私钥
func (c *WalletImportController) hasPrivateKey() (bool, error) {
	key := fmt.Sprintf("%s:private_key", c.RedisService.KeyPrefix)
	exists, err := c.RedisService.Client.Exists(context.Background(), key).Result()
	return exists > 0, err
}

// deletePrivateKey 删除私钥
func (c *WalletImportController) deletePrivateKey() error {
	key := fmt.Sprintf("%s:private_key", c.RedisService.KeyPrefix)
	return c.RedisService.Client.Del(context.Background(), key).Err()
}
