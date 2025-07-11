package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
	"github.com/go-redis/redis/v8"
)

// RedisWalletService Redis钱包服务
type RedisWalletService struct {
	Client     *redis.Client
	KeyPrefix  string
	Expiration time.Duration
}

// NewRedisWalletService 创建新的Redis钱包服务
func NewRedisWalletService(addr string, db int, keyPrefix string) (*RedisWalletService, error) {
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

	return &RedisWalletService{
		Client:     client,
		KeyPrefix:  keyPrefix,
		Expiration: 0, // 永不过期
	}, nil
}

// getWalletKey 获取钱包在Redis中的键名
func (s *RedisWalletService) getWalletKey(address string) string {
	return fmt.Sprintf("%s:wallet:%s", s.KeyPrefix, address)
}

// GetAllWallets 获取所有钱包
func (s *RedisWalletService) GetAllWallets(ctx context.Context) ([]*WalletSettings, error) {
	// 获取所有钱包键
	pattern := fmt.Sprintf("%s:wallet:*", s.KeyPrefix)
	keys, err := s.Client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("获取钱包键失败: %w", err)
	}

	wallets := make([]*WalletSettings, 0, len(keys))
	for _, key := range keys {
		walletJSON, err := s.Client.Get(ctx, key).Result()
		if err != nil {
			logger.Warnf("获取钱包数据失败: %v", err)
			continue
		}

		var walletConfig WalletConfig
		err = json.Unmarshal([]byte(walletJSON), &walletConfig)
		if err != nil {
			logger.Warnf("解析钱包数据失败: %v", err)
			continue
		}

		// 只返回启用的钱包
		/* 		if !walletConfig.Enabled {
			continue
		} */

		// 创建钱包设置
		walletSettings, err := CreateWalletSettings(walletConfig)
		if err != nil {
			logger.Warnf("创建钱包设置失败: %v", err)
			continue
		}

		wallets = append(wallets, walletSettings)
	}

	return wallets, nil
}

// GetWallet 获取单个钱包
func (s *RedisWalletService) GetWallet(ctx context.Context, address string) (*WalletSettings, error) {
	key := s.getWalletKey(address)
	walletJSON, err := s.Client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("钱包不存在: %s", address)
		}
		return nil, fmt.Errorf("获取钱包数据失败: %w", err)
	}

	var walletConfig WalletConfig
	err = json.Unmarshal([]byte(walletJSON), &walletConfig)
	if err != nil {
		return nil, fmt.Errorf("解析钱包数据失败: %w", err)
	}

	// 创建钱包设置
	return CreateWalletSettings(walletConfig)
}

// SaveWallet 保存钱包
func (s *RedisWalletService) SaveWallet(ctx context.Context, wallet *WalletConfig) error {
	// 验证钱包地址
	_, err := solana.PublicKeyFromBase58(wallet.Address)
	if err != nil {
		return fmt.Errorf("无效的钱包地址: %w", err)
	}

	// 序列化钱包数据
	walletJSON, err := json.Marshal(wallet)
	if err != nil {
		return fmt.Errorf("序列化钱包数据失败: %w", err)
	}

	// 保存到Redis
	key := s.getWalletKey(wallet.Address)
	err = s.Client.Set(ctx, key, walletJSON, s.Expiration).Err()
	if err != nil {
		return fmt.Errorf("保存钱包数据失败: %w", err)
	}

	return nil
}

// DeleteWallet 删除钱包
func (s *RedisWalletService) DeleteWallet(ctx context.Context, address string) error {
	key := s.getWalletKey(address)
	result, err := s.Client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("删除钱包失败: %w", err)
	}

	if result == 0 {
		return fmt.Errorf("钱包不存在: %s", address)
	}

	return nil
}

// UpdateWallet 更新钱包
func (s *RedisWalletService) UpdateWallet(ctx context.Context, wallet *WalletConfig) error {
	// 检查钱包是否存在
	key := s.getWalletKey(wallet.Address)
	exists, err := s.Client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("检查钱包是否存在失败: %w", err)
	}

	if exists == 0 {
		return fmt.Errorf("钱包不存在: %s", wallet.Address)
	}

	// 保存更新后的钱包
	return s.SaveWallet(ctx, wallet)
}
