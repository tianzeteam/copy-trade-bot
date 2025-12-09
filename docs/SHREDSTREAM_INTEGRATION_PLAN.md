# ShredStream 集成方案 (方案 B: Go 原生实现)

> **范围说明**: 本版本仅支持 **Pump.fun (内盘)** 平台，其他平台 (PumpAmm, Raydium 等) 将在后续版本中添加。

> **兼容性原则**: 新方案通过配置文件开关控制，**默认关闭**，不影响现有逻辑。用户可随时通过配置切换回原有 gRPC 模式。

## 背景

当前 `copy-trade-bot` 使用 Yellowstone gRPC 订阅链上交易数据，虽然已经是毫秒级响应，但在高竞争场景下（如 Pump.fun 新币狙击），仍存在延迟瓶颈。

**ShredStream** 直接从 Solana 验证者的 Shred 数据流解析交易，可以进一步降低 50-200ms 延迟。

## 实现原则

### 兼容性保证

1. **默认关闭**: `shred_stream.enabled` 默认值为 `false`，不改变任何现有行为
2. **零侵入**: 原有代码路径完全保留，新代码通过条件判断隔离
3. **静默降级**: ShredStream 连接失败时自动降级到纯 gRPC 模式
4. **热切换**: 修改配置文件后重启即可切换模式，无需重新编译

### 代码修改规范

```go
// 所有新增代码块使用注释标记，便于识别和回滚
// ========== 新增: ShredStream 相关 ==========
if sm.shredEnabled {
    // 新功能代码
}
// ========== 新增代码结束 ==========

// 原有代码保持不变
```

---

## 方案选择

采用 **方案 B: Go 原生实现**

| 优点 | 说明 |
|------|------|
| 单进程架构 | 无需额外的 Rust 服务，部署简单 |
| 无 IPC 开销 | 直接在内存中处理，无 Redis 中间层延迟 |
| 统一代码库 | 易于维护和调试 |
| 复用现有解析器 | 已有的 pump/pumpamm/raydium 解析逻辑可直接复用 |

---

## 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Solana Network                              │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────┐
│   ShredStream Server    │     │   Yellowstone gRPC      │
│   (Jito/自建节点)        │     │   (现有订阅)             │
│   Port: 10800           │     │                         │
└─────────────────────────┘     └─────────────────────────┘
              │                               │
              │                               │
              ▼                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      copy-trade-bot (Go)                         │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    StreamManager                         │    │
│  │  ┌───────────────────┐    ┌───────────────────┐         │    │
│  │  │ ShredStreamClient │    │ YellowstoneClient │         │    │
│  │  │ (新增)            │    │ (现有)            │         │    │
│  │  └─────────┬─────────┘    └─────────┬─────────┘         │    │
│  │            │                        │                    │    │
│  │            └───────────┬────────────┘                    │    │
│  │                        ▼                                 │    │
│  │            ┌───────────────────────┐                     │    │
│  │            │    EventMerger        │                     │    │
│  │            │  (去重 & 优先级)       │                     │    │
│  │            └───────────┬───────────┘                     │    │
│  └────────────────────────┼─────────────────────────────────┘    │
│                           ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              TransactionMonitor                          │    │
│  │  - ProcessTransaction (Pump内盘)                         │    │
│  │  - ProcessAmmTransaction (Pump外盘)                      │    │
│  │  - ProcessRaydiumV4Transaction                           │    │
│  │  - ...                                                   │    │
│  └─────────────────────────────────────────────────────────┘    │
│                           │                                      │
│                           ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                   Bot 交易执行                            │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

---

## 核心组件设计

### 1. ShredStream gRPC Proto 定义

需要根据 ShredStream 服务的 proto 文件生成 Go 代码。

**文件**: `shredstream/proto/shredstream.proto`

```protobuf
syntax = "proto3";

package shredstream;

option go_package = "copytrade/shredstream/pb";

service ShredstreamService {
  rpc SubscribeEntries(SubscribeEntriesRequest) returns (stream SubscribeEntriesResponse);
}

message SubscribeEntriesRequest {
  // 可选过滤参数
}

message SubscribeEntriesResponse {
  uint64 slot = 1;
  bytes entries = 2;  // bincode 序列化的 Vec<Entry>
}
```

### 2. Entry 数据结构

Solana Entry 是 Shred 重组后的数据单元，包含多笔交易。

**文件**: `shredstream/types.go`

```go
package shredstream

import (
    "github.com/gagliardetto/solana-go"
)

// Entry Solana Entry 结构 (简化版，匹配 bincode 序列化)
type Entry struct {
    NumHashes     uint64                      // PoH 哈希数
    Hash          solana.Hash                 // Entry 哈希
    Transactions  []solana.Transaction        // 交易列表
}

// ShredEvent 从 ShredStream 解析出的事件
type ShredEvent struct {
    Source       string                       // "shredstream"
    Slot         uint64                       // Slot 号
    Signature    string                       // 交易签名
    ReceivedAt   time.Time                    // 接收时间
    Transaction  *solana.Transaction          // 原始交易
}
```

### 3. ShredStreamClient

**文件**: `shredstream/client.go`

```go
package shredstream

import (
    "context"
    "sync"
    
    pb "copytrade/shredstream/pb"
    "google.golang.org/grpc"
)

type ShredStreamClient struct {
    endpoint      string
    conn          *grpc.ClientConn
    client        pb.ShredstreamServiceClient
    
    ctx           context.Context
    cancel        context.CancelFunc
    
    mu            sync.RWMutex
    isRunning     bool
    
    // 事件通道
    EventChan     chan *ShredEvent
}

// New 创建 ShredStream 客户端
func New(endpoint string) (*ShredStreamClient, error)

// Start 启动订阅
func (c *ShredStreamClient) Start() error

// Stop 停止订阅
func (c *ShredStreamClient) Stop()

// parseEntries 解析 bincode 序列化的 Entry 数据
func (c *ShredStreamClient) parseEntries(data []byte, slot uint64) ([]*ShredEvent, error)
```

### 4. Entry Bincode 解码器

Solana 使用 bincode 序列化 Entry，需要实现 Go 版本的解码器。

**文件**: `shredstream/decoder.go`

```go
package shredstream

import (
    "bytes"
    "encoding/binary"
    "fmt"
    
    "github.com/gagliardetto/solana-go"
)

// DecodeEntries 解码 bincode 序列化的 Vec<Entry>
func DecodeEntries(data []byte) ([]Entry, error) {
    reader := bytes.NewReader(data)
    
    // 读取 Vec 长度 (u64)
    var vecLen uint64
    if err := binary.Read(reader, binary.LittleEndian, &vecLen); err != nil {
        return nil, fmt.Errorf("read vec length: %w", err)
    }
    
    entries := make([]Entry, 0, vecLen)
    for i := uint64(0); i < vecLen; i++ {
        entry, err := decodeEntry(reader)
        if err != nil {
            return nil, fmt.Errorf("decode entry %d: %w", i, err)
        }
        entries = append(entries, entry)
    }
    
    return entries, nil
}

// decodeEntry 解码单个 Entry
func decodeEntry(reader *bytes.Reader) (Entry, error) {
    var entry Entry
    
    // 1. num_hashes (u64)
    if err := binary.Read(reader, binary.LittleEndian, &entry.NumHashes); err != nil {
        return entry, fmt.Errorf("read num_hashes: %w", err)
    }
    
    // 2. hash (32 bytes)
    hashBytes := make([]byte, 32)
    if _, err := reader.Read(hashBytes); err != nil {
        return entry, fmt.Errorf("read hash: %w", err)
    }
    entry.Hash = solana.HashFromBytes(hashBytes)
    
    // 3. transactions (Vec<Transaction>)
    var txCount uint64
    if err := binary.Read(reader, binary.LittleEndian, &txCount); err != nil {
        return entry, fmt.Errorf("read tx count: %w", err)
    }
    
    entry.Transactions = make([]solana.Transaction, 0, txCount)
    for j := uint64(0); j < txCount; j++ {
        tx, err := decodeTransaction(reader)
        if err != nil {
            return entry, fmt.Errorf("decode transaction %d: %w", j, err)
        }
        entry.Transactions = append(entry.Transactions, tx)
    }
    
    return entry, nil
}

// decodeTransaction 解码单个 Transaction
func decodeTransaction(reader *bytes.Reader) (solana.Transaction, error) {
    // Solana Transaction bincode 格式:
    // 1. signatures: Vec<Signature>
    // 2. message: Message
    
    var tx solana.Transaction
    
    // 读取签名数量
    var sigCount uint64
    if err := binary.Read(reader, binary.LittleEndian, &sigCount); err != nil {
        return tx, err
    }
    
    // 读取签名
    tx.Signatures = make([]solana.Signature, sigCount)
    for i := uint64(0); i < sigCount; i++ {
        sigBytes := make([]byte, 64)
        if _, err := reader.Read(sigBytes); err != nil {
            return tx, err
        }
        copy(tx.Signatures[i][:], sigBytes)
    }
    
    // 读取 Message
    msg, err := decodeMessage(reader)
    if err != nil {
        return tx, err
    }
    tx.Message = msg
    
    return tx, nil
}

// decodeMessage 解码 Message
func decodeMessage(reader *bytes.Reader) (solana.Message, error) {
    // Message 格式:
    // 1. header: MessageHeader (3 bytes)
    // 2. account_keys: Vec<Pubkey>
    // 3. recent_blockhash: Hash
    // 4. instructions: Vec<CompiledInstruction>
    
    var msg solana.Message
    
    // Header
    header := make([]byte, 3)
    if _, err := reader.Read(header); err != nil {
        return msg, err
    }
    msg.Header.NumRequiredSignatures = header[0]
    msg.Header.NumReadonlySignedAccounts = header[1]
    msg.Header.NumReadonlyUnsignedAccounts = header[2]
    
    // Account Keys
    var keyCount uint64
    if err := binary.Read(reader, binary.LittleEndian, &keyCount); err != nil {
        return msg, err
    }
    
    // 注意：这里用 compact-u16 编码，简化处理
    // 实际可能需要处理 compact-u16
    msg.AccountKeys = make([]solana.PublicKey, keyCount)
    for i := uint64(0); i < keyCount; i++ {
        keyBytes := make([]byte, 32)
        if _, err := reader.Read(keyBytes); err != nil {
            return msg, err
        }
        msg.AccountKeys[i] = solana.PublicKeyFromBytes(keyBytes)
    }
    
    // Recent Blockhash
    hashBytes := make([]byte, 32)
    if _, err := reader.Read(hashBytes); err != nil {
        return msg, err
    }
    msg.RecentBlockhash = solana.HashFromBytes(hashBytes)
    
    // Instructions
    var ixCount uint64
    if err := binary.Read(reader, binary.LittleEndian, &ixCount); err != nil {
        return msg, err
    }
    
    msg.Instructions = make([]solana.CompiledInstruction, ixCount)
    for i := uint64(0); i < ixCount; i++ {
        ix, err := decodeCompiledInstruction(reader)
        if err != nil {
            return msg, err
        }
        msg.Instructions[i] = ix
    }
    
    return msg, nil
}
```

### 5. EventMerger (事件去重)

**文件**: `core/event_merger.go`

```go
package core

import (
    "sync"
    "time"
    
    lru "github.com/hashicorp/golang-lru/v2"
)

type EventSource string

const (
    SourceShredStream EventSource = "shredstream"
    SourceGRPC        EventSource = "grpc"
)

type ProcessedEvent struct {
    Source    EventSource
    Timestamp time.Time
}

type EventMerger struct {
    cache     *lru.Cache[string, *ProcessedEvent]
    mu        sync.RWMutex
    cacheSize int
    ttl       time.Duration
}

func NewEventMerger(cacheSize int, ttl time.Duration) (*EventMerger, error) {
    cache, err := lru.New[string, *ProcessedEvent](cacheSize)
    if err != nil {
        return nil, err
    }
    
    return &EventMerger{
        cache:     cache,
        cacheSize: cacheSize,
        ttl:       ttl,
    }, nil
}

// ShouldProcess 检查事件是否应该处理
// 返回 true 表示应该处理，false 表示已处理过应跳过
func (m *EventMerger) ShouldProcess(signature string, source EventSource) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 检查是否已存在
    if existing, ok := m.cache.Get(signature); ok {
        // 已存在，检查是否过期
        if time.Since(existing.Timestamp) < m.ttl {
            return false  // 未过期，跳过
        }
    }
    
    // 添加到缓存
    m.cache.Add(signature, &ProcessedEvent{
        Source:    source,
        Timestamp: time.Now(),
    })
    
    return true
}

// GetSource 获取事件的原始来源
func (m *EventMerger) GetSource(signature string) (EventSource, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    if event, ok := m.cache.Get(signature); ok {
        return event.Source, true
    }
    return "", false
}
```

### 6. StreamManager 集成

**修改文件**: `stream/manager.go`

```go
// StreamManager 新增字段
type StreamManager struct {
    // ... 现有字段 ...
    
    // ShredStream 相关
    shredClient    *shredstream.ShredStreamClient
    shredEnabled   bool
    
    // 事件去重
    eventMerger    *core.EventMerger
}

// NewStreamManager 修改
func NewStreamManager(...) *StreamManager {
    // ... 现有逻辑 ...
    
    // 初始化事件去重器
    eventMerger, _ := core.NewEventMerger(100000, 60*time.Second)
    
    sm := &StreamManager{
        // ... 现有字段 ...
        eventMerger: eventMerger,
    }
    
    return sm
}

// Start 修改 (保持原有逻辑不变，仅在配置启用时添加新功能)
func (sm *StreamManager) Start() error {
    // ========== 原有 gRPC 启动逻辑 (保持不变) ==========
    // ... 现有代码完全保留 ...
    
    // ========== 新增: ShredStream 启动 (仅在配置启用时执行) ==========
    if sm.shredEnabled {
        if err := sm.startShredStream(); err != nil {
            logger.Warnf("Failed to start ShredStream: %v, falling back to gRPC only", err)
            // 失败时静默降级，不影响原有 gRPC 功能
        } else {
            go sm.handleShredStream()
            logger.Info("ShredStream enabled, running in dual-channel mode")
        }
    }
    // ========== 新增代码结束 ==========
    
    return nil
}

// handleShredStream 处理 ShredStream 事件
func (sm *StreamManager) handleShredStream() {
    for event := range sm.shredClient.EventChan {
        // 去重检查
        if !sm.eventMerger.ShouldProcess(event.Signature, core.SourceShredStream) {
            continue
        }
        
        // 解析交易并处理
        sm.processShredEvent(event)
    }
}

// processShredEvent 处理单个 ShredStream 事件
func (sm *StreamManager) processShredEvent(event *shredstream.ShredEvent) {
    tx := event.Transaction
    
    // 获取账户列表
    accountKeys := tx.Message.AccountKeys
    accountsAddress := make([]string, len(accountKeys))
    for i, key := range accountKeys {
        accountsAddress[i] = key.String()
    }
    
    // 检查是否包含目标程序
    // PumpFun (内盘) - 当前版本仅支持
    if containsProgram(accountsAddress, core.PUMP_PROGRAM_ID.String()) {
        sm.processShredPumpFun(event, accountsAddress)
        return
    }
    
    // TODO: 后续版本支持
    // - PumpAMM (外盘)
    // - Raydium V4
    // - Raydium CPMM
    // - 其他 DEX
}

// processTransactionUpdate 修改 (仅在启用 ShredStream 时添加去重检查)
func (sm *StreamManager) processTransactionUpdate(out *yellowstone_geyser_pb.SubscribeUpdate) {
    signature := base58.Encode(out.GetTransaction().Transaction.Signature)
    
    // ========== 新增: 去重检查 (仅在 ShredStream 启用时执行) ==========
    if sm.shredEnabled && sm.eventMerger != nil {
        if !sm.eventMerger.ShouldProcess(signature, core.SourceGRPC) {
            logger.Debugf("Skipping duplicate event from gRPC: %s", signature)
            return
        }
    }
    // ========== 新增代码结束 ==========
    
    // ... 原有处理逻辑完全保留 ...
}
```

---

## 配置项新增

### config.yaml 新增字段

```yaml
# ShredStream 配置 (新增，默认关闭)
shred_stream:
  enabled: false                             # 是否启用 ShredStream (默认 false，不影响原有逻辑)
  endpoint: "http://127.0.0.1:10800"         # ShredStream 服务地址
  reconnect_interval: 5                      # 重连间隔（秒）
  max_retries: 10                            # 最大重试次数

# 事件去重配置 (仅在 shred_stream.enabled=true 时生效)
event_dedup:
  cache_size: 100000                         # LRU 缓存大小
  ttl_seconds: 60                            # 缓存过期时间（秒）
```

### 配置模式说明

| 模式 | shred_stream.enabled | 行为 |
|------|---------------------|------|
| **原有模式** (默认) | `false` | 仅使用 Yellowstone gRPC，代码路径与升级前完全一致 |
| **双通道模式** | `true` | ShredStream + gRPC 并行，ShredStream 优先，gRPC 作为备份 |

### Config 结构体新增

**文件**: `utils/config.go`

```go
type Config struct {
    // ... 现有字段 ...
    
    // ShredStream 配置
    ShredStreamEnabled       bool    `yaml:"shred_stream_enabled"`
    ShredStreamEndpoint      string  `yaml:"shred_stream_endpoint"`
    ShredReconnectInterval   int     `yaml:"shred_reconnect_interval"`
    ShredMaxRetries          int     `yaml:"shred_max_retries"`
    
    // 事件去重配置
    EventDedupEnabled        bool    `yaml:"event_dedup_enabled"`
    EventDedupCacheSize      int     `yaml:"event_dedup_cache_size"`
    EventDedupTTL            int     `yaml:"event_dedup_ttl_seconds"`
}
```

---

## 文件变更清单

### 新增文件

| 文件路径 | 说明 |
|----------|------|
| `shredstream/` | ShredStream 模块目录 |
| `├── proto/shredstream.proto` | gRPC Proto 定义 |
| `├── pb/shredstream.pb.go` | 生成的 gRPC 代码 |
| `├── pb/shredstream_grpc.pb.go` | 生成的 gRPC 服务代码 |
| `├── types.go` | 数据类型定义 |
| `├── decoder.go` | Entry bincode 解码器 |
| `├── client.go` | ShredStream gRPC 客户端 |
| `└── parser.go` | 交易解析适配器 |
| `core/event_merger.go` | 事件去重合并器 |

### 修改文件

| 文件路径 | 修改内容 |
|----------|----------|
| `main.go` | 初始化 ShredStream 和 EventMerger |
| `config/config.yaml` | 新增 shred_stream 配置段 |
| `utils/config.go` | 解析新配置项 |
| `stream/manager.go` | 集成 ShredStream 和去重逻辑 |
| `go.mod` | 添加 LRU 缓存依赖 |

---

## 数据流处理流程

### ShredStream 事件处理流程

```
ShredStream Server
       │
       ▼
┌─────────────────┐
│ gRPC Stream     │
│ SubscribeEntries│
└────────┬────────┘
         │ SubscribeEntriesResponse { slot, entries }
         ▼
┌─────────────────┐
│ DecodeEntries   │  bincode -> []Entry
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ For each Entry  │
│ For each Tx     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ EventMerger     │  检查签名是否已处理
│ ShouldProcess?  │
└────────┬────────┘
         │ Yes
         ▼
┌─────────────────┐
│ Check Program   │  判断交易涉及的程序
│ PumpFun?        │
│ PumpAmm?        │
│ Raydium?        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Parse Event     │  复用现有解析逻辑
│ pump.DecodeEventsInLogs()
│ 或直接解析指令数据
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ TransactionMon  │  执行跟单逻辑
│ ProcessTransaction()
└─────────────────┘
```

### gRPC 与 ShredStream 并行处理

```
                    ┌─────────────────┐
                    │   EventMerger   │
                    │   (LRU Cache)   │
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
         ▼                   │                   ▼
┌─────────────────┐          │          ┌─────────────────┐
│ ShredStream     │          │          │ Yellowstone     │
│ (更快, ~50ms)   │          │          │ gRPC (~200ms)   │
└────────┬────────┘          │          └────────┬────────┘
         │                   │                   │
         │ Event A (sig: xxx)│                   │ Event A (sig: xxx)
         │                   │                   │
         ▼                   │                   ▼
   ShouldProcess?            │             ShouldProcess?
   ✓ Yes (首次)              │             ✗ No (已存在)
         │                   │                   │
         ▼                   │                   X (跳过)
   Process Event             │
         │                   │
         ▼                   │
   Execute Trade             │
```

---

## 关键实现细节

### 1. Bincode 解码注意事项

Solana 的 bincode 序列化有几个特殊点：

- **Vec 长度**: 使用 `u64` 小端序
- **Compact-u16**: 部分字段使用压缩整数编码
- **Option**: 用一个字节表示 None(0) 或 Some(1)

建议参考 `solana-sdk` 的 Rust 实现：
- `solana_entry::entry::Entry`
- `solana_sdk::transaction::Transaction`

### 2. 交易解析复用

ShredStream 获取的是原始 `Transaction`，而现有的 `PumpParse` 等函数接收的是 gRPC 的 `SubscribeUpdate`。

**适配方案**：

```go
// shredstream/parser.go

// AdaptToGrpcFormat 将原始 Transaction 转换为 gRPC 格式
func AdaptToGrpcFormat(tx *solana.Transaction, slot uint64) *yellowstone_geyser_pb.SubscribeUpdate {
    // 构造兼容的 SubscribeUpdate 结构
    // 或者提取公共逻辑到独立函数
}

// 或者更简洁的方案：直接调用解析函数
func (sm *StreamManager) processShredPumpFun(event *ShredEvent, accounts []string) {
    tx := event.Transaction
    
    // 从交易的日志或指令中解析事件
    // 注意：ShredStream 拿到的是未执行的交易，没有日志
    // 需要从指令数据直接解析
    
    for _, ix := range tx.Message.Instructions {
        programID := accounts[ix.ProgramIDIndex]
        if programID != core.PUMP_PROGRAM_ID.String() {
            continue
        }
        
        // 解析指令数据
        data := ix.Data
        discriminator := data[:8]
        
        // 根据 discriminator 判断是 Buy 还是 Sell
        switch {
        case bytes.Equal(discriminator, pump.BuyDiscriminator):
            tradeInfo := parsePumpBuyInstruction(data, accounts, ix.Accounts)
            sm.transactionMonitor.ProcessTransaction(tradeInfo)
            
        case bytes.Equal(discriminator, pump.SellDiscriminator):
            tradeInfo := parsePumpSellInstruction(data, accounts, ix.Accounts)
            sm.transactionMonitor.ProcessSellTransaction(tradeInfo)
        }
    }
}
```

### 3. 无日志情况的处理

**重要差异**：
- gRPC `SubscribeUpdate` 包含交易执行后的日志 (`LogMessages`)
- ShredStream Entry 只有原始交易，**没有执行日志**

**解决方案**：
- 直接从指令数据 (`Instruction.Data`) 解析
- 参考 `pump/buy.go` 和 `pump/sell.go` 的指令结构
- 不依赖 `pump.DecodeEventsInLogs()`

---

## 测试计划

### 单元测试

| 测试项 | 文件 | 说明 |
|--------|------|------|
| Entry 解码 | `shredstream/decoder_test.go` | 测试 bincode 解码正确性 |
| 事件去重 | `core/event_merger_test.go` | 测试 LRU 缓存和去重逻辑 |
| 指令解析 | `shredstream/parser_test.go` | 测试从指令数据解析事件 |

### 集成测试

1. **延迟对比测试**
   - 同时启用 ShredStream 和 gRPC
   - 记录同一交易的两个来源时间差
   - 预期：ShredStream 快 50-200ms

2. **去重测试**
   - 验证同一交易不会被处理两次
   - 检查日志中的跳过记录

3. **故障恢复测试**
   - 模拟 ShredStream 断连
   - 验证 gRPC 继续正常工作
   - 验证 ShredStream 自动重连

---

## 部署流程

### 1. 确保 ShredStream Server 可用

```bash
# 测试连接
grpcurl -plaintext 127.0.0.1:10800 list
```

### 2. 更新配置

```yaml
# config/config.yaml
shred_stream:
  enabled: true
  endpoint: "http://127.0.0.1:10800"
```

### 3. 重新编译

```bash
cd /usr/local/server/copy-trade-bot
go build -o copytradeV7.1.0
```

### 4. 重启服务

```bash
supervisorctl restart copytrade
```

---

## 性能预期

| 指标 | 当前 (仅 gRPC) | 优化后 (ShredStream + gRPC) |
|------|---------------|---------------------------|
| 事件延迟 | 200-500ms | 50-150ms (ShredStream) |
| 首单优势 | 中等 | 显著提升 |
| CPU 占用 | 低 | 略有增加 (并行处理) |
| 内存占用 | 低 | 略有增加 (LRU 缓存) |
| 可靠性 | 高 | 更高 (双通道冗余) |

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| ShredStream 断连 | 延迟回退到 gRPC 级别 | gRPC 作为备份，自动重连 |
| Entry 解码错误 | 交易丢失 | 完善错误处理，记录日志 |
| 去重缓存溢出 | 内存增加或重复处理 | 合理配置 LRU 大小 |
| 指令解析不完整 | 漏掉部分事件类型 | 参考 Rust 实现，覆盖所有 discriminator |

---

## 开发计划

| 阶段 | 任务 | 预计工时 |
|------|------|----------|
| 1 | 定义 Proto 并生成 Go 代码 | 1h |
| 2 | 实现 Entry bincode 解码器 | 4h |
| 3 | 实现 ShredStreamClient | 3h |
| 4 | 实现 EventMerger 去重 | 2h |
| 5 | 实现 ShredStream 交易解析 (PumpFun 内盘) | 3h |
| 6 | 修改 StreamManager 集成 | 2h |
| 7 | 配置项和启动逻辑 | 1h |
| 8 | 单元测试 | 3h |
| 9 | 集成测试和调试 | 4h |
| **总计** | | **23h** |

### 后续版本计划

| 版本 | 新增支持 | 预计工时 |
|------|----------|----------|
| v7.2 | PumpAmm (外盘) | 3h |
| v7.3 | Raydium V4, CPMM | 4h |
| v7.4 | Raydium Launchpad, 其他 DEX | 4h |

---

## 待确认事项

1. **ShredStream Server**：
   - 是使用 Jito 服务还是自建节点？
   - 服务地址和端口？

2. **Proto 文件**：
   - 是否有现成的 ShredStream proto 定义？
   - 还是需要根据 Rust 项目反推？

3. **优先级策略**：
   - ShredStream 事件是否总是优先？
   - 还是根据具体场景动态选择？

4. **监控需求**：
   - 是否需要记录两个数据源的延迟差？
   - 是否需要告警机制？

---

**文档版本**：v2.2 (方案 B - 仅 Pump 内盘, 配置开关控制)  
**创建日期**：2025-12-09  
**最后更新**：2025-12-09  
**作者**：Droid
