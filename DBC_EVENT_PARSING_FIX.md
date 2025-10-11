# Meteora DBC 事件解析修复说明

## 问题描述

原始的 `parseDbcSwapEvent` 方法尝试从日志消息（logMessages）中解析 DBC 事件，查找 `"Program data: "` 前缀。但是，Meteora DBC 程序使用 Anchor 框架，事件数据是通过 CPI (Cross-Program Invocation) 发出的，而不是在日志消息中。

### 实际的事件数据位置

根据 Solscan 交易分析（交易签名: 5NAMpGoWyH2fuVBTp3WS9cpUBCRz2iodBeMW2vJ9eK7ogfr5Mdc92vAEf9bVmk7smyWnQ9gWbSuDgN2XU9YKY7BJ），DBC 事件数据位于：

- **Inner Instructions #4.3 和 #4.4**: Meteora Dynamic Bonding Curve Program: anchor Self CPI Log
- 这些内部指令包含实际的事件数据

## 修复方案

### 1. 创建了新的辅助函数文件 (`dbc/events_helper.go`)

提供了从 InnerInstructions 中提取和解析 DBC 事件的工具函数：

- `DecodeEventsFromInnerInstructions()`: 从内部指令中解码事件二进制数据
- `ParseEventsFromBinaries()`: 从二进制数据解析为事件对象
- `DecodeAndParseEvents()`: 一站式解码和解析函数

### 2. 更新了 `parseDbcSwapEvent` 方法

**修改前**:
```go
func (b *Bot) parseDbcSwapEvent(logMessages []string, pool string, config string, user string, mintAddress string)
```

**修改后**:
```go
func (b *Bot) parseDbcSwapEvent(innerInstructions []*yellowstone_geyser_pb.InnerInstructions, accountKeys []string, pool string, config string, user string, mintAddress string)
```

**关键改进**:
1. 从 `logMessages` 改为 `innerInstructions` 作为数据源
2. 将 Yellowstone gRPC 格式转换为标准的 Solana 格式
3. 遍历内部指令，查找 DBC 程序发出的 CPI 调用
4. 从指令数据中提取事件（跳过前 8 字节的判别器）
5. 使用 `dbc.ParseAnyEvent()` 解析事件

### 3. 更新了 `MeteoraDbcParse` 方法

在调用 `parseDbcSwapEvent` 之前：
1. 获取账户列表 (`accountList`)
2. 获取内部指令 (`innerInstructions`)
3. 传递正确的参数

### 4. 改进了 `processEvtSwap2` 方法

添加了详细的注释说明买入和卖出时的数据流向：

**买入 (TradeDirection = 0)**:
- `SwapParameters.Amount0` = 输入的 SOL 数量
- `SwapResult.OutputAmount` = 获得的 Token 数量

**卖出 (TradeDirection = 1)**:
- `SwapParameters.Amount0` = 输入的 Token 数量
- `SwapResult.OutputAmount` = 获得的 SOL 数量

## 事件结构

### EvtSwap
```go
type EvtSwap struct {
    Pool             PublicKey
    Config           PublicKey
    TradeDirection   uint8        // 0=Buy, 1=Sell
    HasReferral      bool
    Params           SwapParameters
    SwapResult       SwapResult
    AmountIn         uint64
    CurrentTimestamp uint64
}
```

### EvtSwap2
```go
type EvtSwap2 struct {
    Pool               PublicKey
    Config             PublicKey
    TradeDirection     uint8        // 0=Buy, 1=Sell
    HasReferral        bool
    SwapParameters     SwapParameters2
    SwapResult         SwapResult2
    QuoteReserveAmount uint64
    MigrationThreshold uint64
    CurrentTimestamp   uint64
}
```

## 测试验证

使用真实交易数据验证：
- **交易**: 卖出 11,987.252589 代币换取 0.366866397 SOL
- **Pool**: Meteora DBC (大皇宫-WSOL) Market
- **User**: E2A3xE5a5PW71DtiFxVePzZvBEdf6TiE5LMec7TeMT5M

## 编译状态

✅ 编译通过，无错误

## 相关文件

- `/usr/local/server/copy-trade-bot/dbc/events_helper.go` (新建)
- `/usr/local/server/copy-trade-bot/bot/meteora-dbc.go` (修改)
- `/usr/local/server/copy-trade-bot/dbc/events.go` (使用现有)
- `/usr/local/server/copy-trade-bot/dbc/types.go` (使用现有)

## 参考

- Anchor 框架事件通过 CPI 发出
- Yellowstone gRPC InnerInstructions 结构
- Solscan 交易浏览器分析
