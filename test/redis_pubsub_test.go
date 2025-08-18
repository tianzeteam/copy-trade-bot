package test

import (
	"context"
	"copytrade/types"
	"encoding/json"
	"flag"

	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func TestPublishTradeMessage(t *testing.T) {
	// 命令行参数
	redisAddr := flag.String("redis", "localhost:6379", "Redis地址")
	channel := flag.String("channel", "flashtrade:trade", "Redis通道")
	mint := flag.String("mint", "89wKsFhfN4Hb5UYTrkcU3XVhxASSq7vpxdsmKXvsZe37", "代币地址")
	platform := flag.String("platform", "pump", "交易平台 (pump, pumpamm, raydiumV4, cpmm, launchpad)")
	flag.Parse()

	// 验证必要参数
	if *mint == "" {
		fmt.Println("错误: 必须提供代币地址 (-mint)")
		flag.Usage()
		os.Exit(1)
	}

	// 验证平台参数
	validPlatforms := map[string]bool{
		"pump":      true,
		"pumpamm":   true,
		"cpmm":      true,
		"launchpad": true,
	}
	if !validPlatforms[*platform] {
		fmt.Println("错误: 无效的平台，必须是 pump, pumpamm, raydiumV4, cpmm 或 launchpad")
		flag.Usage()
		os.Exit(1)
	}

	// 连接Redis
	client := redis.NewClient(&redis.Options{
		Addr:     *redisAddr,
		Password: "", // Redis没有密码
		DB:       0,  // 使用默认DB
	})

	// 测试连接
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		fmt.Printf("连接Redis失败: %v\n", err)
		os.Exit(1)
	}

	// 创建交易消息
	message := types.TradeMessage{
		Action:   "buy",
		Mint:     *mint,
		Platform: *platform,
	}

	// 序列化消息
	msgJSON, err := json.Marshal(message)
	if err != nil {
		fmt.Printf("序列化消息失败: %v\n", err)
		os.Exit(1)
	}

	// 发布消息
	err = client.Publish(ctx, *channel, msgJSON).Err()
	if err != nil {
		fmt.Printf("发布消息失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("已发布交易消息到通道 %s: %s\n", *channel, string(msgJSON))

	// 等待一秒，确保消息发送完成
	time.Sleep(time.Second)
}
