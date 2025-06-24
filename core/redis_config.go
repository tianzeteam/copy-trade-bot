package core

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"copytrade/logger"

	"github.com/go-redis/redis/v8"
)

// ConfigService Redis配置服务
type ConfigService struct {
	Client     *redis.Client
	KeyPrefix  string
	Expiration time.Duration
}

// Config 全局配置结构
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

// NewConfigService 创建新的Redis配置服务
func NewConfigService(addr string, db int, keyPrefix string) (*ConfigService, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // Redis没有密码
		DB:       db,
	})

	// 测试连接
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return &ConfigService{
		Client:     client,
		KeyPrefix:  keyPrefix,
		Expiration: 0, // 永不过期
	}, nil
}

// getConfigKey 获取配置在Redis中的键名
func (s *ConfigService) getConfigKey() string {
	return fmt.Sprintf("%s:global_config", s.KeyPrefix)
}

// GetConfig 获取全局配置
func (s *ConfigService) GetConfig(ctx context.Context) (*Config, error) {
	key := s.getConfigKey()
	configJSON, err := s.Client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// 配置不存在，返回默认配置
			return s.getDefaultConfig(), nil
		}
		return nil, fmt.Errorf("获取配置数据失败: %w", err)
	}

	var config Config
	err = json.Unmarshal([]byte(configJSON), &config)
	if err != nil {
		return nil, fmt.Errorf("解析配置数据失败: %w", err)
	}

	return &config, nil
}

// SaveConfig 保存全局配置
func (s *ConfigService) SaveConfig(ctx context.Context, config *Config) error {
	// 更新时间戳
	config.UpdatedAt = time.Now()

	// 序列化配置数据
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置数据失败: %w", err)
	}

	// 保存到Redis
	key := s.getConfigKey()
	err = s.Client.Set(ctx, key, configJSON, s.Expiration).Err()
	if err != nil {
		return fmt.Errorf("保存配置数据失败: %w", err)
	}

	logger.Infof("成功保存全局配置")
	return nil
}

// UpdateConfig 更新部分配置
func (s *ConfigService) UpdateConfig(ctx context.Context, updates map[string]any) error {
	// 获取当前配置
	config, err := s.GetConfig(ctx)
	if err != nil {
		return err
	}

	// 将当前配置转换为map
	configMap := make(map[string]any)
	configJSON, _ := json.Marshal(config)
	json.Unmarshal(configJSON, &configMap)

	// 应用更新
	maps.Copy(configMap, updates)

	// 转换回Config结构
	updatedConfigJSON, _ := json.Marshal(configMap)
	var updatedConfig Config
	json.Unmarshal(updatedConfigJSON, &updatedConfig)

	// 保存更新后的配置
	return s.SaveConfig(ctx, &updatedConfig)
}

// getDefaultConfig 获取默认配置
func (s *ConfigService) getDefaultConfig() *Config {
	return &Config{
		MaxPoolSize:            10.0,  // 默认最大池子大小为10 SOL
		MinFollowTriggerAmount: 0.1,   // 默认最小触发买入数量为0.1 SOL
		MaxBuyAmount:           1.0,   // 默认最大买入金额为1 SOL
		MinBuyAmount:           0.05,  // 默认最小买入金额为0.05 SOL
		SellPercentage:         100.0, // 默认卖出100%
		StopLossPercentage:     10.0,  // 默认止损10%
		TakeProfitPercentage:   50.0,  // 默认止盈50%
		UpdatedAt:              time.Now(),
	}
}
