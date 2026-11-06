package global

import (
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
)

// PumpCurveSnapshot 是 Pump 内盘交易所需的曲线状态快照。
// 事件处理阶段负责更新，买入/卖出组装阶段只读取，不访问 RPC。
type PumpCurveSnapshot struct {
	Mint                 solana.PublicKey
	BondingCurve         solana.PublicKey
	Creator              solana.PublicKey
	QuoteMint            solana.PublicKey
	BaseTokenProgram     solana.PublicKey
	VirtualTokenReserves uint64
	VirtualQuoteReserves uint64
	RealTokenReserves    uint64
	RealQuoteReserves    uint64
	TokenTotalSupply     uint64
	CreatorFeeBps        uint64
	IsMayhemMode         bool
	IsHolderReward       bool
	IsCashbackCoin       bool
	UpdatedAt            time.Time
}

const pumpCurveCacheMaxEntries = 8192

var pumpCurveCache = struct {
	sync.RWMutex
	values map[string]PumpCurveSnapshot
}{
	values: make(map[string]PumpCurveSnapshot),
}

// GetPumpCurveSnapshot 返回指定 mint 的曲线快照。
func GetPumpCurveSnapshot(mint solana.PublicKey) (PumpCurveSnapshot, bool) {
	if mint.IsZero() {
		return PumpCurveSnapshot{}, false
	}
	key := mint.String()
	pumpCurveCache.RLock()
	snapshot, ok := pumpCurveCache.values[key]
	pumpCurveCache.RUnlock()
	return snapshot, ok
}

// SetPumpCurveSnapshot 写入完整曲线快照。
func SetPumpCurveSnapshot(snapshot PumpCurveSnapshot) {
	if snapshot.Mint.IsZero() {
		return
	}
	key := snapshot.Mint.String()
	snapshot.UpdatedAt = time.Now()
	pumpCurveCache.Lock()
	if _, exists := pumpCurveCache.values[key]; !exists {
		prunePumpCurveCacheLocked()
	}
	pumpCurveCache.values[key] = snapshot
	pumpCurveCache.Unlock()
}

// UpdatePumpCurveSnapshot 原子更新指定 mint 的曲线快照。
// update 应只修改事件明确提供的字段，保留其它已缓存字段。
func UpdatePumpCurveSnapshot(mint solana.PublicKey, update func(*PumpCurveSnapshot)) {
	if mint.IsZero() || update == nil {
		return
	}
	key := mint.String()
	pumpCurveCache.Lock()
	defer pumpCurveCache.Unlock()

	snapshot, exists := pumpCurveCache.values[key]
	if !exists {
		snapshot.Mint = mint
		prunePumpCurveCacheLocked()
	}
	update(&snapshot)
	snapshot.Mint = mint
	snapshot.UpdatedAt = time.Now()
	pumpCurveCache.values[key] = snapshot
}

func prunePumpCurveCacheLocked() {
	if len(pumpCurveCache.values) < pumpCurveCacheMaxEntries {
		return
	}
	var oldestKey string
	var oldest time.Time
	for key, snapshot := range pumpCurveCache.values {
		if oldestKey == "" || snapshot.UpdatedAt.Before(oldest) {
			oldestKey = key
			oldest = snapshot.UpdatedAt
		}
	}
	if oldestKey != "" {
		delete(pumpCurveCache.values, oldestKey)
	}
}
