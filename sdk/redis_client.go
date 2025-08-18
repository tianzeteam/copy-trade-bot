package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"copytrade/logger"
	"copytrade/types"

	"github.com/go-redis/redis/v8"
)

// TradeSubscriptionClient 交易订阅客户端SDK
type TradeSubscriptionClient struct {
	redisClient *redis.Client
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewTradeSubscriptionClient 创建新的交易订阅客户端
func NewTradeSubscriptionClient(client *redis.Client) (*TradeSubscriptionClient, error) {

	ctx, cancel := context.WithCancel(context.Background())

	// 测试连接
	_, err := client.Ping(ctx).Result()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return &TradeSubscriptionClient{
		redisClient: client,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Close 关闭订阅客户端连接
func (c *TradeSubscriptionClient) Close() error {
	c.cancel() // 取消上下文
	return c.redisClient.Close()
}

// PublishTradeMessage 发布交易消息
func (c *TradeSubscriptionClient) PublishTradeMessage(channel string, message types.TradeMessage) error {
	// 验证平台
	if !ValidatePlatform(message.Platform) {
		return fmt.Errorf("不支持的交易平台: %s", message.Platform)
	}

	// 序列化消息
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化交易消息失败: %w", err)
	}

	// 发布消息
	err = c.redisClient.Publish(c.ctx, channel, msgJSON).Err()
	if err != nil {
		return fmt.Errorf("发布交易消息失败: %w", err)
	}

	logger.Infof("已发布交易消息到通道 %s: %s", channel, string(msgJSON))
	return nil
}

// PublishDelayedTradeMessage 发布延迟交易消息
func (c *TradeSubscriptionClient) PublishDelayedTradeMessage(channel string, message types.TradeMessage, delayMilliseconds int) error {
	// 验证平台
	if !ValidatePlatform(message.Platform) {
		return fmt.Errorf("不支持的交易平台: %s", message.Platform)
	}

	// 序列化消息
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化交易消息失败: %w", err)
	}

	// 发布消息
	err = c.redisClient.Publish(c.ctx, channel, msgJSON).Err()
	if err != nil {
		return fmt.Errorf("发布交易消息失败: %w", err)
	}

	logger.Infof("已添加延迟交易消息到队列，通道: %s，延迟: %d毫秒，消息: %s",
		channel, delayMilliseconds, string(msgJSON))
	return nil
}

// PublishDelayedClosePositionMessage 发布延迟平仓消息
func (c *TradeSubscriptionClient) PublishDelayedCPMessage(channel string, message types.ClosePositionMessage, delayMilliseconds int) error {
	// 验证平台
	if !ValidatePlatform(message.Platform) {
		return fmt.Errorf("不支持的交易平台: %s", message.Platform)
	}

	// 序列化消息
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化平仓消息失败: %w", err)
	}
	time.Sleep(time.Duration(delayMilliseconds) * time.Millisecond)
	// 发布消息
	err = c.redisClient.Publish(c.ctx, channel, msgJSON).Err()
	if err != nil {
		return fmt.Errorf("发布平仓消息失败: %w", err)
	}

	logger.Infof("已添加延迟平仓消息到队列，通道: %s，延迟: %d毫秒，消息: %s",
		channel, delayMilliseconds, string(msgJSON))
	return nil
}

// SubscribeToChannel 订阅交易消息通道
func (c *TradeSubscriptionClient) SubscribeToChannel(channel string, handler func(types.TradeMessage)) error {
	pubsub := c.redisClient.Subscribe(c.ctx, channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	logger.Infof("已订阅交易消息通道: %s", channel)

	for {
		select {
		case <-c.ctx.Done():
			logger.Infof("关闭订阅: %s", channel)
			return nil
		case msg := <-ch:
			// 解析消息
			var message types.TradeMessage
			err := json.Unmarshal([]byte(msg.Payload), &message)
			if err != nil {
				logger.Errorf("解析交易消息失败: %v", err)
				continue
			}

			// 验证平台
			if !ValidatePlatform(message.Platform) {
				logger.Errorf("不支持的交易平台: %s", message.Platform)
				continue
			}

			// 调用处理函数
			go handler(message)
		}
	}
}

// ProcessDelayedMessages 处理延迟消息队列
// 此方法应在单独的goroutine中运行
func (c *TradeSubscriptionClient) ProcessDelayedMessages() {
	logger.Infof("开始处理延迟消息队列")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// 获取当前时间
		now := float64(time.Now().Unix())

		// 查找所有应该执行的消息
		keys, err := c.redisClient.ZRangeByScore(c.ctx, "delayed_queue", &redis.ZRangeBy{
			Min: "0",
			Max: fmt.Sprintf("%f", now),
		}).Result()

		if err != nil {
			logger.Errorf("查询延迟消息失败: %v", err)
			continue
		}

		// 处理每个消息
		for _, key := range keys {
			// 获取消息内容
			msgJSON, err := c.redisClient.Get(c.ctx, key).Result()
			if err != nil {
				logger.Errorf("获取延迟消息内容失败: %v", err)
				continue
			}

			// 解析消息
			var message types.TradeMessage
			err = json.Unmarshal([]byte(msgJSON), &message)
			if err != nil {
				logger.Errorf("解析延迟消息失败: %v", err)
				continue
			}

			// 提取通道名称
			parts := splitKey(key)
			if len(parts) < 2 {
				logger.Errorf("无效的延迟消息键: %s", key)
				continue
			}
			channel := parts[1]

			// 发布消息
			err = c.redisClient.Publish(c.ctx, channel, msgJSON).Err()
			if err != nil {
				logger.Errorf("发布延迟消息失败: %v", err)
				continue
			}

			logger.Infof("已发布延迟消息到通道 %s: %s", channel, msgJSON)

			// 删除已处理的消息
			pipe := c.redisClient.Pipeline()
			pipe.ZRem(c.ctx, "delayed_queue", key)
			pipe.Del(c.ctx, key)
			_, err = pipe.Exec(c.ctx)
			if err != nil {
				logger.Errorf("删除已处理的延迟消息失败: %v", err)
			}
		}
	}
}

// splitKey 分割延迟消息键
func splitKey(key string) []string {
	var result []string
	var current string

	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			result = append(result, current)
			current = ""
		} else {
			current += string(key[i])
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// ValidatePlatform 验证交易平台是否有效
func ValidatePlatform(platform string) bool {
	validPlatforms := []string{"pumpfun", "pumpamm", "cpmm", "launchpad", "raydiumV4", "unknown"}
	return contains(validPlatforms, platform)
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
