package core

import (
	"sync"
	"time"
)

// EventSource 事件来源
type EventSource string

const (
	SourceShredStream EventSource = "shredstream"
	SourceGRPC        EventSource = "grpc"
)

// ProcessedEvent 已处理的事件记录
type ProcessedEvent struct {
	Source    EventSource
	Timestamp time.Time
}

// EventMerger 事件去重合并器
// 用于在 ShredStream + gRPC 双通道模式下去重相同交易
type EventMerger struct {
	cache     map[string]*ProcessedEvent
	mu        sync.RWMutex
	cacheSize int
	ttl       time.Duration

	// 统计信息
	stats struct {
		shredStreamFirst uint64 // ShredStream 先到的次数
		grpcFirst        uint64 // gRPC 先到的次数
		duplicates       uint64 // 去重的次数
	}
}

// NewEventMerger 创建事件去重器
func NewEventMerger(cacheSize int, ttlSeconds int) *EventMerger {
	if cacheSize <= 0 {
		cacheSize = 100000
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}

	em := &EventMerger{
		cache:     make(map[string]*ProcessedEvent, cacheSize),
		cacheSize: cacheSize,
		ttl:       time.Duration(ttlSeconds) * time.Second,
	}

	// 启动定期清理过期条目的 goroutine
	go em.cleanupLoop()

	return em
}

// ShouldProcess 检查事件是否应该处理
// 返回 true 表示应该处理（首次出现），false 表示已处理过应跳过
func (m *EventMerger) ShouldProcess(signature string, source EventSource) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 检查是否已存在
	if existing, ok := m.cache[signature]; ok {
		// 已存在，检查是否过期
		if now.Sub(existing.Timestamp) < m.ttl {
			// 未过期，跳过
			m.stats.duplicates++
			return false
		}
		// 已过期，覆盖
	}

	// 缓存满时执行简单清理（删除最老的10%）
	if len(m.cache) >= m.cacheSize {
		m.evictOldest(m.cacheSize / 10)
	}

	// 添加到缓存
	m.cache[signature] = &ProcessedEvent{
		Source:    source,
		Timestamp: now,
	}

	// 更新统计
	if source == SourceShredStream {
		m.stats.shredStreamFirst++
	} else {
		m.stats.grpcFirst++
	}

	return true
}

// GetSource 获取事件的原始来源
func (m *EventMerger) GetSource(signature string) (EventSource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if event, ok := m.cache[signature]; ok {
		return event.Source, true
	}
	return "", false
}

// GetStats 获取统计信息
func (m *EventMerger) GetStats() map[string]uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]uint64{
		"shredstream_first": m.stats.shredStreamFirst,
		"grpc_first":        m.stats.grpcFirst,
		"duplicates":        m.stats.duplicates,
		"cache_size":        uint64(len(m.cache)),
	}
}

// cleanupLoop 定期清理过期条目
func (m *EventMerger) cleanupLoop() {
	ticker := time.NewTicker(m.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup 清理过期条目
func (m *EventMerger) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for sig, event := range m.cache {
		if now.Sub(event.Timestamp) > m.ttl {
			delete(m.cache, sig)
		}
	}
}

// evictOldest 删除最老的 n 个条目
func (m *EventMerger) evictOldest(n int) {
	if n <= 0 || len(m.cache) == 0 {
		return
	}

	// 简单实现：删除最老的条目
	// 注意：这不是最优实现，但对于高吞吐场景足够
	type entry struct {
		sig string
		ts  time.Time
	}

	entries := make([]entry, 0, len(m.cache))
	for sig, event := range m.cache {
		entries = append(entries, entry{sig: sig, ts: event.Timestamp})
	}

	// 按时间排序（简单选择最老的）
	for i := 0; i < n && i < len(entries); i++ {
		oldestIdx := i
		for j := i + 1; j < len(entries); j++ {
			if entries[j].ts.Before(entries[oldestIdx].ts) {
				oldestIdx = j
			}
		}
		if oldestIdx != i {
			entries[i], entries[oldestIdx] = entries[oldestIdx], entries[i]
		}
		delete(m.cache, entries[i].sig)
	}
}
