# 买单活跃度监控功能 - 实施文档

## 功能概述

**需求**: 在 pumpfun 内盘自己买入后，如果 X 秒内没有新买单，就卖出 X%

**实现**: 创建了 `BuyActivityMonitor` 监控器，专门监控 pumpfun 平台的买单活跃度

## 文件变更清单

### 新增文件
- ✅ `bot/buy_activity_monitor.go` - 买单活跃度监控器核心实现

### 修改文件
- ✅ `utils/config.go` - 添加 `BuyActivityMonitorConfig` 配置结构
- ✅ `config/config.yaml` - 添加 `buy_activity_monitor` 配置项
- ✅ `bot/structs.go` - 集成监控器到 Bot 结构体
- ✅ `bot/pump-sniper.go` - 在买入成功后启动监控

## 配置说明

### config.yaml 配置项

```yaml
buy_activity_monitor:
    enable: false              # 是否启用买单活跃度监控
    check_interval: 5          # 检查间隔（秒）
    timeout: 30                # 无新买单超时时间（秒）
    sell_percent: 50           # 超时卖出比例（%）
    min_buy_amount: 0.01       # 最小买单金额（SOL），低于此金额不算活跃买单
```

### 配置参数详解

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enable` | bool | false | 是否启用监控功能 |
| `check_interval` | int | 5 | 每隔多少秒检查一次是否有新买单 |
| `timeout` | int | 30 | 多少秒内无新买单则触发卖出 |
| `sell_percent` | int | 50 | 触发后卖出的比例（百分比） |
| `min_buy_amount` | float64 | 0.01 | 最小买单金额（SOL），低于此金额的买单不计入活跃度 |

## 工作流程

```
Bot 在 pumpfun 买入成功
    ↓
启动买单活跃度监控 (goroutine)
    ↓
每 5 秒检查一次是否有新买单
    ├─ 有新买单 (金额 ≥ 0.01 SOL) → 重置计时器 → 继续监控
    └─ 30 秒内无新买单 → 卖出 50% → 停止监控
```

## 核心功能特性

### 1. 智能过滤机制
- ✅ **平台限定**: 仅支持 pumpfun 内盘
- ✅ **金额过滤**: 只统计 ≥ 0.01 SOL 的买单
- ✅ **自我过滤**: 自动过滤掉 bot 自己的买单
- ✅ **时间判断**: 只检测新的买单（基于 `ReceivedAt` 时间戳）

### 2. 并发安全
- ✅ 使用 `sync.RWMutex` 保护任务映射表
- ✅ 使用 `goroutine` + `channel` 实现异步监控
- ✅ 使用 `CancelChan` 实现优雅取消

### 3. 资源管理
- ✅ 自动清理已完成的监控任务
- ✅ 支持取消监控
- ✅ 防止重复监控同一代币

## 日志输出示例

### 启动监控
```
🔍 启动买单活跃度监控: 代币=ABC123..., 超时=30s, 卖出比例=50%, 最小买单=0.0100 SOL
```

### 检测到新买单
```
💰 检测到有效新买单: 代币=ABC123..., 用户=DEF456..., 金额=0.0500 SOL, 时间=10:30:15.123
✅ 检测到新买单: 代币=ABC123..., 重置监控计时器
```

### 超时触发卖出
```
⏰ 买单活跃度超时: 代币=ABC123..., 已等待30.0s无新买单
⚠️ 买单活跃度不足: 代币=ABC123..., 30s内无新买单，执行卖出50%
📤 准备卖出(买单活跃度不足): 代币=ABC123..., 数量=50000 (50%), 买入价=0.00001234, 当前价=0.00001456
✅ 已发送卖出请求到通道(买单活跃度): 代币=ABC123..., 数量=50000
```

### 过滤日志（Debug级别）
```
检测到买单但金额过小: 代币=ABC123..., 金额=0.0050 SOL < 最小要求0.0100 SOL
过滤掉bot自己的买单: 代币=ABC123...
```

## 使用方法

### 1. 启用监控

编辑 `config/config.yaml`:
```yaml
buy_activity_monitor:
    enable: true           # 启用监控
    check_interval: 5      # 每5秒检查一次
    timeout: 30            # 30秒无新买单则触发
    sell_percent: 50       # 卖出50%
    min_buy_amount: 0.01   # 最小买单0.01 SOL
```

### 2. 重启程序

```bash
# 停止当前运行的程序
pkill -f copy-trade-bot

# 重新编译
cd /usr/local/server/copy-trade-bot
go build

# 启动程序
./copy-trade-bot
```

### 3. 观察日志

```bash
# 查看实时日志
tail -f logs/copy-trade-bot.log | grep "买单活跃度"
```

## 与盈利监控器的协同

| 特性 | ProfitTargetMonitor | BuyActivityMonitor |
|------|---------------------|-------------------|
| 监控目标 | 价格盈利是否达标 | 是否有新买单活跃度 |
| 触发条件 | 超时未达到目标盈利 | 超时无新买单 |
| 数据来源 | 价格信息 | 交易事件（IsBuy字段） |
| 卖出策略 | 部分卖出或全部清仓 | 卖出指定比例 |
| 使用场景 | 止损保护 | 流动性/热度判断 |

**两个监控器可以同时运行**，互不干扰，各自监控不同的指标。

## 测试建议

### 1. 功能测试
- ✅ 测试启用/禁用功能
- ✅ 测试不同的超时时间设置
- ✅ 测试不同的卖出比例
- ✅ 测试最小买单金额过滤

### 2. 边界测试
- ✅ 测试极短超时时间（如 5 秒）
- ✅ 测试极长超时时间（如 300 秒）
- ✅ 测试 100% 卖出
- ✅ 测试最小买单金额为 0

### 3. 并发测试
- ✅ 同时监控多个代币
- ✅ 快速买入卖出循环
- ✅ 监控器取消和重启

### 4. 集成测试
- ✅ 与盈利监控器同时运行
- ✅ 与多级卖出策略配合
- ✅ 与延迟卖出功能配合

## 性能优化建议

### 1. 调整检查间隔
- 高频交易场景：`check_interval: 3`
- 普通场景：`check_interval: 5`
- 低频场景：`check_interval: 10`

### 2. 调整超时时间
- 热门代币：`timeout: 15-30`
- 普通代币：`timeout: 30-60`
- 冷门代币：`timeout: 60-120`

### 3. 调整最小买单金额
- 高市值代币：`min_buy_amount: 0.05-0.1`
- 中市值代币：`min_buy_amount: 0.01-0.05`
- 低市值代币：`min_buy_amount: 0.001-0.01`

## 故障排查

### 问题1: 监控未启动
**症状**: 买入后没有看到监控日志
**排查**:
1. 检查 `config.yaml` 中 `enable` 是否为 `true`
2. 检查是否是 pumpfun 平台（其他平台不支持）
3. 检查日志级别是否设置为 `info` 或 `debug`

### 问题2: 监控立即触发
**症状**: 买入后立即卖出
**排查**:
1. 检查 `timeout` 设置是否过短
2. 检查是否有其他买单但被过滤（金额太小）
3. 查看 debug 日志确认过滤原因

### 问题3: 监控不触发
**症状**: 长时间无新买单但不卖出
**排查**:
1. 检查 `timeout` 设置是否过长
2. 检查是否有小额买单持续重置计时器
3. 检查监控任务是否被意外取消

## 代码架构

### 核心结构

```go
// BuyActivityTask - 监控任务
type BuyActivityTask struct {
    MintAddress   string
    Platform      string
    BuyPrice      float64
    BuyAmount     uint64
    BuyTime       time.Time
    LastBuyTime   time.Time
    Timeout       time.Duration
    SellPercent   int
    MinBuyAmount  float64
    CancelChan    chan struct{}
    Owner         solana.PublicKey
    Creator       solana.PublicKey
}

// BuyActivityMonitor - 监控器
type BuyActivityMonitor struct {
    tasks  map[string]*BuyActivityTask
    config *utils.BuyActivityMonitorConfig
    bot    *Bot
    mutex  sync.RWMutex
}
```

### 核心方法

```go
// 启动监控
func (bam *BuyActivityMonitor) StartMonitoring(task *BuyActivityTask)

// 监控循环
func (bam *BuyActivityMonitor) monitorTask(task *BuyActivityTask)

// 检测新买单
func (bam *BuyActivityMonitor) hasNewBuyOrder(task *BuyActivityTask) bool

// 处理超时
func (bam *BuyActivityMonitor) handleTimeout(task *BuyActivityTask)

// 执行卖出
func (bam *BuyActivityMonitor) executeSell(task *BuyActivityTask)

// 取消监控
func (bam *BuyActivityMonitor) CancelMonitoring(mintAddress string)
```

## 未来扩展建议

### 1. 买单质量评估
- 根据买单金额大小给予不同权重
- 统计买单数量而非仅时间
- 分析买单间隔规律

### 2. 动态参数调整
- 根据代币热度动态调整超时时间
- 根据市场情况调整卖出比例
- 根据持仓盈亏调整策略

### 3. 多平台支持
- 扩展到 pump-amm 平台
- 扩展到 raydium 平台
- 统一的监控接口

### 4. 高级策略
- 结合价格变化和买单活跃度
- 支持分批卖出策略
- 支持止盈止损联动

## 版本历史

### v1.0.0 (2025-01-05)
- ✅ 初始版本实现
- ✅ 支持 pumpfun 平台
- ✅ 基础监控功能
- ✅ 智能过滤机制
- ✅ 并发安全保证

## 技术支持

如有问题或建议，请联系开发团队。

---

**编译状态**: ✅ 编译成功  
**测试状态**: ⏳ 待测试  
**部署状态**: ⏳ 待部署
