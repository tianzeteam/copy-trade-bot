package global

import (
	"sync"
	"time"

	"copytrade/pumpV3"
)

// Pump.fun 动态费率配置的进程级缓存。
//
// FeeConfig（pump_fees PDA）决定按市值分档的 protocol / creator / lp 费率，
// Global 提供 creator_fee_configurable 开关与 FeeConfig 缺失时的回退费率。
// 两者都由管理员手动更新，变化极少：启动时预取一次，之后定时刷新即可。
//
// 读取方（金额计算器）拿不到配置时应按保守费率兜底并告警，
// 不能因为费率缓存缺失而阻塞交易。

var (
	pumpFeeMu        sync.RWMutex
	pumpFeeConfig    *pumpV3.FeeConfig
	pumpGlobalConfig *pumpV3.Global
	pumpFeeUpdatedAt time.Time
)

// SetPumpFeeConfig 写入最新的链上费率配置。任一参数可为 nil（拉取失败时保留旧值由调用方决定）。
func SetPumpFeeConfig(feeConfig *pumpV3.FeeConfig, globalConfig *pumpV3.Global) {
	pumpFeeMu.Lock()
	defer pumpFeeMu.Unlock()
	if feeConfig != nil {
		pumpFeeConfig = feeConfig
	}
	if globalConfig != nil {
		pumpGlobalConfig = globalConfig
	}
	pumpFeeUpdatedAt = time.Now()
}

// GetPumpFeeConfig 返回缓存的费率配置，未拉取成功时为 nil。
func GetPumpFeeConfig() (feeConfig *pumpV3.FeeConfig, globalConfig *pumpV3.Global) {
	pumpFeeMu.RLock()
	defer pumpFeeMu.RUnlock()
	return pumpFeeConfig, pumpGlobalConfig
}

// PumpFeeConfigUpdatedAt 返回最近一次成功写入缓存的时间（零值表示从未成功）。
func PumpFeeConfigUpdatedAt() time.Time {
	pumpFeeMu.RLock()
	defer pumpFeeMu.RUnlock()
	return pumpFeeUpdatedAt
}
