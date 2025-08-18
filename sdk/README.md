# 交易订阅 SDK

这个 SDK 专注于交易订阅功能，提供了一种简单的方式来发布和订阅交易消息，支持即时消息和延迟消息发布。

## 功能特点

- 专注交易订阅功能
- 即时消息发布
- 延迟消息发布与处理
- 简单的 API 接口
- 支持多种交易平台
- 提供快速发布方法，无需维护客户端实例

## 安装

```bash
# 确保项目依赖已安装
go get github.com/go-redis/redis/v8
```

## 使用方法

### 1. 创建交易订阅客户端

```go
import "copytrade/sdk"

// 创建交易订阅客户端
client, err := sdk.NewTradeSubscriptionClient("localhost:6379", 0, "")
if err != nil {
    // 处理错误
}
defer client.Close()
```

### 2. 发布即时消息

```go
// 创建交易消息
message := types.TradeMessage{
    Action:   "buy",
    Mint:     "89wKsFhfN4Hb5UYTrkcU3XVhxASSq7vpxdsmKXvsZe37",
    Platform: "pump",
}

// 发布消息
err = client.PublishTradeMessage("flashtrade:trade", message)
if err != nil {
    // 处理错误
}
```

### 3. 发布延迟消息

```go
// 创建交易消息
message := types.TradeMessage{
    Action:   "buy",
    Mint:     "89wKsFhfN4Hb5UYTrkcU3XVhxASSq7vpxdsmKXvsZe37",
    Platform: "pump",
}

// 发布延迟消息 (延迟10秒)
err = client.PublishDelayedTradeMessage("flashtrade:trade", message, 10)
if err != nil {
    // 处理错误
}

// 创建平仓消息
closeMessage := types.ClosePositionMessage{
    MintAddress:  "89wKsFhfN4Hb5UYTrkcU3XVhxASSq7vpxdsmKXvsZe37",
    CurrentPrice: 1.25,
    Platform:     "launchpad",
    MessageType:  "delay_sell",
    Reason:       "延迟卖出",
}

// 发布延迟平仓消息 (延迟30秒)
err = client.PublishDelayedClosePositionMessage("flashtrade:trade", closeMessage, 30)
if err != nil {
    // 处理错误
}
```

### 4. 订阅交易消息

```go
// 订阅交易消息通道
client.SubscribeToChannel("flashtrade:trade", func(message types.TradeMessage) {
    // 处理收到的消息
    fmt.Printf("收到交易消息: %+v\n", message)
    
    // 根据消息执行操作
    if message.Action == "buy" {
        // 处理购买操作
    }
})
```

### 5. 处理延迟消息队列

```go
// 启动延迟消息处理器 (在单独的 goroutine 中运行)
go client.ProcessDelayedMessages()
```

### 5. 快速发布方法 (无需维护客户端实例)

```go
// 快速发布即时消息
err := sdk.QuickPublish("localhost:6379", 0, "flashtrade:trade", message)
if err != nil {
    // 处理错误
}

// 快速发布延迟消息
err := sdk.QuickPublishDelayed("localhost:6379", 0, "flashtrade:trade", message, 10)
if err != nil {
    // 处理错误
}
```

## 完整示例

参见 `examples/redis_sdk_example.go` 文件，其中包含了完整的使用示例。

### 发布消息示例

```bash
# 运行示例 (即时发送)
go run examples/redis_sdk_example.go -mint 89wKsFhfN4Hb5UYTrkcU3XVhxASSq7vpxdsmKXvsZe37 -platform pump

# 运行示例 (延迟发送)
go run examples/redis_sdk_example.go -mint 89wKsFhfN4Hb5UYTrkcU3XVhxASSq7vpxdsmKXvsZe37 -platform pump -delay 10
```

### 订阅消息示例

```bash
# 运行订阅模式，监听交易消息
go run examples/redis_sdk_example.go -subscribe
```

订阅模式下，程序将持续监听指定通道的交易消息，并在收到消息时进行处理。按 Ctrl+C 可以退出订阅模式。

## API 参考

### TradeSubscriptionClient

```go
// 创建新的交易订阅客户端
NewTradeSubscriptionClient(addr string, db int, password string) (*TradeSubscriptionClient, error)

// 关闭交易订阅客户端连接
(*TradeSubscriptionClient) Close() error

// 发布交易消息
(*TradeSubscriptionClient) PublishTradeMessage(channel string, message types.TradeMessage) error

// 发布延迟交易消息
(*TradeSubscriptionClient) PublishDelayedTradeMessage(channel string, message types.TradeMessage, delaySeconds int) error

// 发布延迟平仓消息
(*TradeSubscriptionClient) PublishDelayedClosePositionMessage(channel string, message types.ClosePositionMessage, delaySeconds int) error

// 订阅交易消息通道
(*TradeSubscriptionClient) SubscribeToChannel(channel string, handler func(types.TradeMessage)) error

// 处理延迟消息队列
(*TradeSubscriptionClient) ProcessDelayedMessages()
```

### 快速发布函数

```go
// 快速发布交易消息
QuickPublish(addr string, db int, channel string, message types.TradeMessage) error

// 快速发布延迟交易消息
QuickPublishDelayed(addr string, db int, channel string, message types.TradeMessage, delaySeconds int) error
```

### 辅助函数

```go
// 验证交易平台是否有效
ValidatePlatform(platform string) bool
```

## 支持的交易平台

- `pump`: Pump.fun 平台
- `pumpamm`: Pump AMM 平台
- `cpmm`: Raydium CPMM 平台
- `launchpad`: Raydium Launchpad 平台

## 延迟消息实现原理

延迟消息使用 Redis 有序集合 (Sorted Set) 实现：

1. 消息内容存储在 Redis 键值对中
2. 消息键添加到有序集合，分数为预期执行时间
3. 处理器定期检查有序集合，发布到期的消息
4. 发布后从有序集合和键值存储中删除消息

这种实现方式不依赖于 Redis 的发布/订阅机制，而是利用 Redis 的数据结构实现延迟功能。
