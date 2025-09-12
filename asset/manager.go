package asset

import (
	"context"
	"copytrade/logger"
	"sync"
	"time"
)

// BalanceManager 管理资产余额信息
type BalanceManager struct {
	solBalance     float64       // SOL余额
	walletAddress  string        // 钱包地址
	rpcUrl         string        // RPC URL
	mutex          sync.RWMutex  // 读写锁
	updateInterval time.Duration // 更新间隔
	lastUpdateTime time.Time     // 上次更新时间
}

// 全局单例实例
var (
	balanceManager *BalanceManager
	once           sync.Once
)

// NewBalanceManager 创建资产管理器
func NewBalanceManager(walletAddress, rpcUrl string, updateInterval time.Duration) *BalanceManager {
	once.Do(func() {
		balanceManager = &BalanceManager{
			solBalance:     0,
			walletAddress:  walletAddress,
			rpcUrl:         rpcUrl,
			updateInterval: updateInterval,
			lastUpdateTime: time.Time{},
		}
	})
	return balanceManager
}

// GetBalanceManager 获取全局资产管理器实例
func GetBalanceManager() *BalanceManager {
	if balanceManager == nil {
		panic("BalanceManager not initialized")
	}
	return balanceManager
}

// UpdateWalletAddress 更新钱包地址
func (bm *BalanceManager) UpdateWalletAddress(walletAddress string) {
	bm.walletAddress = walletAddress
}

// InitBalanceManager 初始化资产管理器（如果尚未初始化）
func InitBalanceManager(walletAddress, rpcUrl string, updateInterval time.Duration) *BalanceManager {
	if balanceManager == nil {
		return NewBalanceManager(walletAddress, rpcUrl, updateInterval)
	}
	return balanceManager
}

// UpdateSolBalance 更新SOL余额
func (bm *BalanceManager) UpdateSolBalance() error {
	// 获取SOL余额
	balance, err := GetSolBalance(bm.walletAddress, bm.rpcUrl)
	if err != nil {
		logger.Errorf("更新SOL余额失败: %v", err)
		return err
	}

	// 更新余额
	bm.mutex.Lock()
	bm.solBalance = balance
	bm.lastUpdateTime = time.Now()
	bm.mutex.Unlock()

	logger.Infof("已更新钱包 %s 的SOL余额: %.8f", bm.walletAddress, balance)
	return nil
}

// GetSolBalanceValue 获取当前SOL余额
func (bm *BalanceManager) GetSolBalanceValue() float64 {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	return bm.solBalance
}

// GetLastUpdateTime 获取上次更新时间
func (bm *BalanceManager) GetLastUpdateTime() time.Time {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	return bm.lastUpdateTime
}

// StartAutoUpdate 启动自动更新余额的协程
func (bm *BalanceManager) StartAutoUpdate(ctx context.Context) {
	// 先执行一次更新
	err := bm.UpdateSolBalance()
	if err != nil {
		logger.Errorf("初始化SOL余额失败: %v", err)
	}

	// 启动定时更新协程
	go func() {
		ticker := time.NewTicker(bm.updateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := bm.UpdateSolBalance()
				if err != nil {
					logger.Errorf("自动更新SOL余额失败: %v", err)
				}
			case <-ctx.Done():
				logger.Infof("余额自动更新服务已停止")
				return
			}
		}
	}()

	logger.Infof("已启动SOL余额自动更新服务，更新间隔: %v (秒)", bm.updateInterval.Seconds())
}

// IsBalanceSufficient 检查余额是否足够
func (bm *BalanceManager) IsBalanceSufficient(requiredAmount float64) bool {
	bm.mutex.RLock()
	defer bm.mutex.RUnlock()
	return bm.solBalance >= requiredAmount
}
