package subscribe

import (
	"context"
	"encoding/json"
	"fmt"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/types"
	"copytrade/wallet"

	"github.com/gagliardetto/solana-go"
	"github.com/go-redis/redis/v8"
)

// RedisPubSubService Redis发布/订阅服务
type RedisPubSubService struct {
	Client             *redis.Client
	TradeChannel       string
	Ctx                context.Context
	Cancel             context.CancelFunc
	TransactionMonitor *wallet.TransactionMonitor
}

// NewRedisPubSubService 创建新的Redis发布/订阅服务
func NewRedisPubSubService(redisClient *redis.Client, tradeChannel string, transactionMonitor *wallet.TransactionMonitor) (*RedisPubSubService, error) {
	client := redisClient

	// 测试连接
	ctx, cancel := context.WithCancel(context.Background())
	_, err := client.Ping(ctx).Result()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return &RedisPubSubService{
		Client:             client,
		TradeChannel:       tradeChannel,
		Ctx:                ctx,
		Cancel:             cancel,
		TransactionMonitor: transactionMonitor,
	}, nil
}

// StartSubscription 开始订阅交易消息
func (s *RedisPubSubService) StartSubscription() {
	logger.Infof("开始订阅Redis交易消息通道: %s", s.TradeChannel)

	pubsub := s.Client.Subscribe(s.Ctx, s.TradeChannel)
	defer pubsub.Close()

	// 确认订阅成功
	_, err := pubsub.Receive(s.Ctx)
	if err != nil {
		logger.Errorf("订阅Redis通道失败: %v", err)
		return
	}

	// 获取消息通道
	ch := pubsub.Channel()

	// 处理消息
	for msg := range ch {
		logger.Infof("收到Redis交易消息: %s", msg.Payload)
		go s.handleTradeMessage(msg.Payload)
	}
}

// Stop 停止订阅
func (s *RedisPubSubService) Stop() {
	logger.Infof("停止Redis交易消息订阅")
	s.Cancel()
}

// handleTradeMessage 处理交易消息
func (s *RedisPubSubService) handleTradeMessage(msgPayload string) {
	// 解析消息
	var message types.ClosePositionMessage
	err := json.Unmarshal([]byte(msgPayload), &message)
	if err != nil {
		logger.Errorf("解析交易消息失败: %v", err)
		return
	}

	// 验证消息格式
	if message.MintAddress == "" || message.Platform == "" {
		logger.Errorf("交易消息格式无效: 缺少必要字段")
		return
	}

	// 根据消息类型处理
	switch message.MessageType {
	case "delay_sell":
		s.handleDelaySellMessage(message)
	default:
		logger.Warnf("不支持的交易操作: %s", message.MessageType)
	}
}

// handleDelaySellMessage 处理延迟卖出消息
func (s *RedisPubSubService) handleDelaySellMessage(message types.ClosePositionMessage) {
	// 验证平台
	validPlatforms := []string{"pumpfun", "pumpamm", "cpmm", "launchpad", "raydiumV4"}
	if !contains(validPlatforms, message.Platform) {
		logger.Errorf("不支持的交易平台: %s", message.Platform)
		return
	}

	// 验证代币地址
	mintAddress, err := solana.PublicKeyFromBase58(message.MintAddress)
	if err != nil {
		logger.Errorf("无效的代币地址: %v", err)
		return
	}

	logger.Infof("执行%s平台延迟卖出操作，代币: %s", message.Platform, mintAddress.String())
	assetInfo, ok := global.GetPendingHoldAsset(mintAddress.String())
	if !ok {
		logger.Errorf("未找到代币%s的持仓信息，无法执行卖出操作", mintAddress.String())
		return
	}

	sellAmount := assetInfo.Amount

	switch message.Platform {
	case "pumpfun":
		tradeInfo := global.GetTradeInfo(mintAddress.String())
		if tradeInfo == nil {
			logger.Errorf("未找到代币%s的交易信息，无法执行买入操作", mintAddress.String())
			return
		}
		queue.SendTokenToSellChannel(0, assetInfo, sellAmount, assetInfo.EntryPrice, message.Platform, "延迟卖出策略", "延迟卖出")
	case "pumpamm":
		tradeInfo := global.GetAmmTradeInfo(mintAddress.String())
		if tradeInfo == nil {
			logger.Errorf("未找到代币%s的交易信息，无法执行买入操作", mintAddress.String())
			return
		}
		queue.SendTokenToSellChannel(0, assetInfo, sellAmount, assetInfo.EntryPrice, message.Platform, "延迟卖出策略", "延迟卖出")
	case "cpmm":
		tradeInfo := global.GetCpmmTradeInfo(mintAddress.String())
		if tradeInfo == nil {
			logger.Errorf("未找到代币%s的交易信息，无法执行买入操作", mintAddress.String())
			return
		}
		queue.SendTokenToSellChannel(0, assetInfo, sellAmount, assetInfo.EntryPrice, message.Platform, "延迟卖出策略", "延迟卖出")
	case "launchpad":
		tradeInfo := global.GetLaunchpadTradeInfo(mintAddress.String())
		if tradeInfo == nil {
			logger.Errorf("未找到代币%s的交易信息，无法执行买入操作", mintAddress.String())
			return
		}
		queue.SendTokenToSellChannel(0, assetInfo, sellAmount, assetInfo.EntryPrice, message.Platform, "延迟卖出策略", "延迟卖出")

	case "raydiumV4":
		tradeInfo := global.GetAmmRaydiuV4TradeInfo(mintAddress.String())
		if tradeInfo == nil {
			logger.Errorf("未找到代币%s的交易信息，无法执行买入操作", mintAddress.String())
			return
		}
		queue.SendTokenToSellChannel(0, assetInfo, sellAmount, assetInfo.EntryPrice, message.Platform, "延迟卖出策略", "延迟卖出")
	}

}

// contains 检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// PublishTradeMessage 发布交易消息
func (s *RedisPubSubService) PublishTradeMessage(message types.TradeMessage) error {
	// 序列化消息
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化交易消息失败: %w", err)
	}

	// 发布消息
	err = s.Client.Publish(s.Ctx, s.TradeChannel, msgJSON).Err()
	if err != nil {
		return fmt.Errorf("发布交易消息失败: %w", err)
	}

	logger.Infof("已发布交易消息: %s", string(msgJSON))
	return nil
}

// PublishMessage 直接发布交易消息到Redis通道（无需服务实例）
func PublishMessage(redisAddr string, redisDB int, channel string, message types.TradeMessage) error {
	// 连接Redis
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // Redis没有密码
		DB:       redisDB,
	})

	// 创建上下文
	ctx := context.Background()

	// 测试连接
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("连接Redis失败: %w", err)
	}

	// 序列化消息
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化交易消息失败: %w", err)
	}

	// 发布消息
	err = client.Publish(ctx, channel, msgJSON).Err()
	if err != nil {
		return fmt.Errorf("发布交易消息失败: %w", err)
	}

	logger.Infof("已发布交易消息到通道 %s: %s", channel, string(msgJSON))

	// 关闭连接
	_ = client.Close()

	return nil
}

// ValidatePlatform 验证交易平台是否有效
func ValidatePlatform(platform string) bool {
	validPlatforms := []string{"pump", "pumpamm", "cpmm", "launchpad"}
	return contains(validPlatforms, platform)
}
