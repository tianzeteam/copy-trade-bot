package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
	"github.com/go-redis/redis/v8"
)

// BlacklistAddress 黑名单地址结构
type BlacklistAddress struct {
	Address   string    `json:"address"`    // 钱包地址
	Note      string    `json:"note"`       // 备注说明
	AddedAt   time.Time `json:"added_at"`   // 添加时间
	AddedBy   string    `json:"added_by"`   // 添加者
	IsEnabled bool      `json:"is_enabled"` // 是否启用
}

// BlacklistService 黑名单服务
type BlacklistService struct {
	Client    *redis.Client
	KeyPrefix string
}

// NewBlacklistService 创建新的黑名单服务
func NewBlacklistService(client *redis.Client, keyPrefix string) *BlacklistService {
	return &BlacklistService{
		Client:    client,
		KeyPrefix: keyPrefix,
	}
}

// getBlacklistKey 获取黑名单在Redis中的键名
func (s *BlacklistService) getBlacklistKey(address string) string {
	return fmt.Sprintf("%s:blacklist:%s", s.KeyPrefix, address)
}

// getBlacklistSetKey 获取黑名单集合的键名
func (s *BlacklistService) getBlacklistSetKey() string {
	return fmt.Sprintf("%s:blacklist:set", s.KeyPrefix)
}

// AddAddress 添加黑名单地址
func (s *BlacklistService) AddAddress(ctx context.Context, address, note, addedBy string) error {
	// 验证地址格式
	_, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return fmt.Errorf("无效的钱包地址: %w", err)
	}

	// 检查是否已存在
	exists, err := s.IsBlacklisted(ctx, address)
	if err != nil {
		return fmt.Errorf("检查地址是否存在失败: %w", err)
	}
	if exists {
		return fmt.Errorf("地址已在黑名单中: %s", address)
	}

	// 创建黑名单记录
	blacklistAddr := &BlacklistAddress{
		Address:   address,
		Note:      note,
		AddedAt:   time.Now(),
		AddedBy:   addedBy,
		IsEnabled: true,
	}

	// 序列化
	data, err := json.Marshal(blacklistAddr)
	if err != nil {
		return fmt.Errorf("序列化黑名单数据失败: %w", err)
	}

	// 保存到Redis
	key := s.getBlacklistKey(address)
	err = s.Client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("保存黑名单数据失败: %w", err)
	}

	// 添加到集合中
	setKey := s.getBlacklistSetKey()
	err = s.Client.SAdd(ctx, setKey, address).Err()
	if err != nil {
		return fmt.Errorf("添加到黑名单集合失败: %w", err)
	}

	logger.Infof("添加黑名单地址: %s, 备注: %s, 添加者: %s", address, note, addedBy)
	return nil
}

// AddAddressBatch 批量添加黑名单地址
func (s *BlacklistService) AddAddressBatch(ctx context.Context, addresses []string, note, addedBy string) ([]string, []string) {
	var successAddrs []string
	var failedAddrs []string

	for _, addr := range addresses {
		// 去除空格
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}

		err := s.AddAddress(ctx, addr, note, addedBy)
		if err != nil {
			logger.Warnf("添加黑名单地址失败: %s, 错误: %v", addr, err)
			failedAddrs = append(failedAddrs, addr)
		} else {
			successAddrs = append(successAddrs, addr)
		}
	}

	return successAddrs, failedAddrs
}

// RemoveAddress 删除黑名单地址
func (s *BlacklistService) RemoveAddress(ctx context.Context, address string) error {
	// 从集合中删除
	setKey := s.getBlacklistSetKey()
	err := s.Client.SRem(ctx, setKey, address).Err()
	if err != nil {
		return fmt.Errorf("从黑名单集合删除失败: %w", err)
	}

	// 删除详细信息
	key := s.getBlacklistKey(address)
	result, err := s.Client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("删除黑名单数据失败: %w", err)
	}

	if result == 0 {
		return fmt.Errorf("黑名单地址不存在: %s", address)
	}

	logger.Infof("删除黑名单地址: %s", address)
	return nil
}

// GetAddress 获取单个黑名单地址信息
func (s *BlacklistService) GetAddress(ctx context.Context, address string) (*BlacklistAddress, error) {
	key := s.getBlacklistKey(address)
	data, err := s.Client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("黑名单地址不存在: %s", address)
		}
		return nil, fmt.Errorf("获取黑名单数据失败: %w", err)
	}

	var blacklistAddr BlacklistAddress
	err = json.Unmarshal([]byte(data), &blacklistAddr)
	if err != nil {
		return nil, fmt.Errorf("解析黑名单数据失败: %w", err)
	}

	return &blacklistAddr, nil
}

// GetAllAddresses 获取所有黑名单地址
func (s *BlacklistService) GetAllAddresses(ctx context.Context) ([]*BlacklistAddress, error) {
	// 从集合中获取所有地址
	setKey := s.getBlacklistSetKey()
	addresses, err := s.Client.SMembers(ctx, setKey).Result()
	if err != nil {
		return nil, fmt.Errorf("获取黑名单集合失败: %w", err)
	}

	blacklist := make([]*BlacklistAddress, 0, len(addresses))
	for _, addr := range addresses {
		blacklistAddr, err := s.GetAddress(ctx, addr)
		if err != nil {
			logger.Warnf("获取黑名单地址详情失败: %s, 错误: %v", addr, err)
			continue
		}
		blacklist = append(blacklist, blacklistAddr)
	}

	return blacklist, nil
}

// IsBlacklisted 检查地址是否在黑名单中
func (s *BlacklistService) IsBlacklisted(ctx context.Context, address string) (bool, error) {
	setKey := s.getBlacklistSetKey()
	exists, err := s.Client.SIsMember(ctx, setKey, address).Result()
	if err != nil {
		return false, fmt.Errorf("检查黑名单失败: %w", err)
	}
	return exists, nil
}

// UpdateAddress 更新黑名单地址信息
func (s *BlacklistService) UpdateAddress(ctx context.Context, address, note string, isEnabled bool) error {
	// 获取现有记录
	blacklistAddr, err := s.GetAddress(ctx, address)
	if err != nil {
		return err
	}

	// 更新字段
	blacklistAddr.Note = note
	blacklistAddr.IsEnabled = isEnabled

	// 序列化
	data, err := json.Marshal(blacklistAddr)
	if err != nil {
		return fmt.Errorf("序列化黑名单数据失败: %w", err)
	}

	// 保存到Redis
	key := s.getBlacklistKey(address)
	err = s.Client.Set(ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("更新黑名单数据失败: %w", err)
	}

	logger.Infof("更新黑名单地址: %s, 备注: %s, 启用: %v", address, note, isEnabled)
	return nil
}

// GetEnabledAddresses 获取所有启用的黑名单地址
func (s *BlacklistService) GetEnabledAddresses(ctx context.Context) ([]string, error) {
	allAddresses, err := s.GetAllAddresses(ctx)
	if err != nil {
		return nil, err
	}

	enabledAddrs := make([]string, 0)
	for _, addr := range allAddresses {
		if addr.IsEnabled {
			enabledAddrs = append(enabledAddrs, addr.Address)
		}
	}

	return enabledAddrs, nil
}

// ClearAll 清空所有黑名单
func (s *BlacklistService) ClearAll(ctx context.Context) error {
	// 获取所有地址
	addresses, err := s.GetAllAddresses(ctx)
	if err != nil {
		return err
	}

	// 删除所有地址
	for _, addr := range addresses {
		err := s.RemoveAddress(ctx, addr.Address)
		if err != nil {
			logger.Warnf("删除黑名单地址失败: %s, 错误: %v", addr.Address, err)
		}
	}

	logger.Infof("清空所有黑名单地址，共删除 %d 个", len(addresses))
	return nil
}
