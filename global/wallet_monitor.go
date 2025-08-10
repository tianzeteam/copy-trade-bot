package global

import (
	"sync"
)

var (
	// walletMonitor 全局钱包监控器实例
	walletMonitor interface{}
	// walletMonitorMutex 钱包监控器读写锁
	walletMonitorMutex sync.RWMutex
)

// SetWalletMonitor 设置全局钱包监控器
func SetWalletMonitor(monitor interface{}) {
	walletMonitorMutex.Lock()
	defer walletMonitorMutex.Unlock()
	walletMonitor = monitor
}

// GetWalletMonitor 获取全局钱包监控器
func GetWalletMonitor() interface{} {
	walletMonitorMutex.RLock()
	defer walletMonitorMutex.RUnlock()
	return walletMonitor
}
