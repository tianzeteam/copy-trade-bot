package test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"copytrade/logger"
	"copytrade/wallet"
)

// Config 配置结构体
type Config struct {
	// 交易参数
	MaxPoolSize            float64 `json:"maxPoolSize"`            // 最大池子大小 (SOL)
	MinFollowTriggerAmount float64 `json:"minFollowTriggerAmount"` // 最小触发买入数量 (SOL)

	// 可以添加更多配置参数
	MaxBuyAmount         float64 `json:"maxBuyAmount"`         // 最大买入金额 (SOL)
	MinBuyAmount         float64 `json:"minBuyAmount"`         // 最小买入金额 (SOL)
	SellPercentage       float64 `json:"sellPercentage"`       // 卖出百分比
	StopLossPercentage   float64 `json:"stopLossPercentage"`   // 止损百分比
	TakeProfitPercentage float64 `json:"takeProfitPercentage"` // 止盈百分比

	// 其他系统配置
	UpdatedAt time.Time `json:"updatedAt"` // 最后更新时间
}

// 配置存储的Redis键
const configKey = "global_config"

// getDefaultConfig 获取默认配置
func getDefaultConfig() *Config {
	return &Config{
		MaxPoolSize:            10.0,
		MinFollowTriggerAmount: 1.0,
		MaxBuyAmount:           1.0,
		MinBuyAmount:           0.1,
		SellPercentage:         50,
		StopLossPercentage:     10,
		TakeProfitPercentage:   150,
		UpdatedAt:              time.Now(),
	}
}

func main() {
	// 设置Redis连接参数
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // 默认Redis地址
	}

	redisDB := 0                  // 默认使用DB 0
	redisKeyPrefix := "copytrade" // 默认键前缀

	// 创建Redis服务
	redisService, err := wallet.NewRedisWalletService(redisAddr, redisDB, redisKeyPrefix)
	if err != nil {
		logger.Fatalf("创建Redis服务失败: %v", err)
	}

	// 获取当前配置
	ctx := context.Background()
	configJSON, err := redisService.Client.Get(ctx, configKey).Result()

	var currentConfig Config
	if err != nil {
		fmt.Println("配置不存在，使用默认配置")
		currentConfig = *getDefaultConfig()
	} else {
		if err := json.Unmarshal([]byte(configJSON), &currentConfig); err != nil {
			fmt.Printf("解析配置失败: %v，使用默认配置\n", err)
			currentConfig = *getDefaultConfig()
		} else {
			configJSON, _ := json.MarshalIndent(currentConfig, "", "  ")
			fmt.Printf("当前配置:\n%s\n\n", string(configJSON))
		}
	}

	// 更新部分配置
	updates := map[string]interface{}{
		"maxPoolSize":            5.0,
		"minFollowTriggerAmount": 0.5,
		"maxBuyAmount":           0.8,
		"sellPercentage":         75,
		"stopLossPercentage":     15,
		"takeProfitPercentage":   200,
	}

	fmt.Printf("更新配置: %+v\n", updates)

	// 将当前配置转换为map
	currentMap := make(map[string]interface{})
	currentJSON, _ := json.Marshal(currentConfig)
	json.Unmarshal(currentJSON, &currentMap)

	// 应用更新
	for key, value := range updates {
		currentMap[key] = value
	}

	// 更新时间戳
	currentMap["updatedAt"] = time.Now()

	// 转换回Config结构
	updatedJSON, _ := json.Marshal(currentMap)
	var updatedConfig Config
	json.Unmarshal(updatedJSON, &updatedConfig)

	// 保存到Redis
	saveJSON, err := json.Marshal(updatedConfig)
	if err != nil {
		logger.Fatalf("序列化配置失败: %v", err)
	}

	// 写入Redis
	if err := redisService.Client.Set(ctx, configKey, saveJSON, 0).Err(); err != nil {
		logger.Fatalf("保存Redis配置失败: %v", err)
	}

	fmt.Println("配置更新成功")

	// 获取更新后的配置
	updatedConfigJSON, err := redisService.Client.Get(ctx, configKey).Result()
	if err != nil {
		logger.Fatalf("获取更新后的配置失败: %v", err)
	}

	var finalConfig Config
	if err := json.Unmarshal([]byte(updatedConfigJSON), &finalConfig); err != nil {
		logger.Fatalf("解析更新后的配置失败: %v", err)
	}

	prettyJSON, _ := json.MarshalIndent(finalConfig, "", "  ")
	fmt.Printf("更新后的配置:\n%s\n", string(prettyJSON))
}
