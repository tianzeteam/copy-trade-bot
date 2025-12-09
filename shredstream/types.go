package shredstream

import (
	"time"

	"github.com/gagliardetto/solana-go"
)

// ShredEvent 从 ShredStream 解析出的交易事件
type ShredEvent struct {
	Source      string              // 数据来源标识 "shredstream"
	Slot        uint64              // Slot 号
	Signature   string              // 交易签名
	ReceivedAt  time.Time           // 接收时间
	Transaction *solana.Transaction // 原始交易
}

// Entry Solana Entry 结构 (匹配 bincode 序列化格式)
type Entry struct {
	NumHashes    uint64                // PoH 哈希数
	Hash         solana.Hash           // Entry 哈希
	Transactions []solana.Transaction  // 交易列表
}

// ShredStreamConfig ShredStream 配置
type ShredStreamConfig struct {
	Enabled           bool   `yaml:"enabled"`            // 是否启用
	Endpoint          string `yaml:"endpoint"`           // 服务端点
	ReconnectInterval int    `yaml:"reconnect_interval"` // 重连间隔（秒）
	MaxRetries        int    `yaml:"max_retries"`        // 最大重试次数
}

// EventDedupConfig 事件去重配置
type EventDedupConfig struct {
	CacheSize  int `yaml:"cache_size"`   // LRU 缓存大小
	TTLSeconds int `yaml:"ttl_seconds"`  // 缓存过期时间（秒）
}
