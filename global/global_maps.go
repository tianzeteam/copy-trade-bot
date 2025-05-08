package global

import (
	"copytrade/logger"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
)

// TradeEventInfo 存储交易事件信息
type TradeEventInfo struct {
	UserAddress          string    // 用户地址
	SolAmount            uint64    // Sol数量
	TokenAmount          uint64    // Token数量
	IsBuy                bool      // 是否买入
	MintAddress          string    // Mint地址
	TimeStamp            int64     // 时间戳
	VirtualSolReserves   uint64    // 虚拟Sol储备
	VirtualTokenReserves uint64    // 虚拟Token储备
	RealSolReserves      uint64    // 实际Sol储备
	RealTokenReserves    uint64    // 实际Token储备
	CurrentPrice         float64   // 当前价格
	TradeTime            string    // 交易时间
	Milliseconds         int64     // 毫秒时间戳
	ReceivedAt           time.Time // 接收时间
}

// MyBuyMap 存储买入信息
type MyBuyMap struct {
	UserAddress string
	MintAddress string
	EntryPrice  float64
	Amount      uint64
	SoldLevels  map[int]bool // 记录已经卖出的价格档位
	TimeStamp   int64
}

// CreateEventInfo 存储代币创建事件信息
type CreateEventInfo struct {
	Name         string
	Symbol       string
	Uri          string
	MintAddress  string
	BondingCurve string
	Creator      string
	CreatedAt    time.Time
}

// AmmTradeEventInfo 存储AMM交易事件信息
type AmmTradeEventInfo struct {
	UserAddress  solana.PublicKey // 用户地址
	SolAmount    uint64           // Sol数量
	TokenAmount  uint64           // Token数量
	IsBuy        bool             // 是否买入
	MintAddress  solana.PublicKey // Mint地址
	AmmPool      solana.PublicKey // AMM池地址
	TimeStamp    int64            // 时间戳
	BaseReserve  uint64           // 基础代币储备
	QuoteReserve uint64           // 报价代币储备
	CurrentPrice float64          // 当前价格
	TradeTime    string           // 交易时间
	ReceivedAt   time.Time        // 接收时间
	TxHash       string           // 交易哈希
	Slot         uint64           // 区块槽位
}

// AmmPoolInfo 存储AMM池信息
type AmmPoolInfo struct {
	AmmPool               solana.PublicKey // AMM池地址
	BaseMint              solana.PublicKey // 基础代币地址
	QuoteMint             solana.PublicKey // 报价代币地址
	PoolBaseTokenAccount  solana.PublicKey // 池中基础代币账户
	PoolQuoteTokenAccount solana.PublicKey // 池中报价代币账户
	LastUpdated           time.Time        // 最后更新时间
}

// TokenSellLimiter 代币卖出限制器，用于防止短时间内重复卖出同一个代币
type TokenSellLimiter struct {
	LastSellTime time.Time
}

// 全局交易信息映射表，键为Mint地址
var (
	// TradeInfoMap 存储PumpFun交易信息，键为Mint地址
	TradeInfoMap     = make(map[string]*TradeEventInfo)
	TradeInfoMapLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// PendingHoldAsset 全局等待持有资产映射表
	PendingHoldAsset     = make(map[string]*MyBuyMap)
	PendingHoldAssetLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// CreateInfoMap 全局代币创建信息映射表，键为Mint地址
	CreateInfoMap     = make(map[string]*CreateEventInfo)
	CreateInfoMapLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// AmmTradeInfoMap 全局AMM交易信息映射表，键为Mint地址
	AmmTradeInfoMap     = make(map[string]*AmmTradeEventInfo)
	AmmTradeInfoMapLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// AmmPoolInfoMap 全局AMM池信息映射表，键为Mint地址
	AmmPoolInfoMap     = make(map[string]*AmmPoolInfo)
	AmmPoolInfoMapLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// TokenSellLimiters 代币卖出限制器映射表，键为Mint地址
	TokenSellLimiters     = make(map[string]*TokenSellLimiter)
	TokenSellLimitersLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// 代币卖出时间间隔限制，默认5秒
	SellIntervalLimit = 5 * time.Second
)

// GetTradeInfo 获取指定代币的交易信息
func GetTradeInfo(mintAddress string) *TradeEventInfo {
	TradeInfoMapLock.RLock()
	defer TradeInfoMapLock.RUnlock()
	return TradeInfoMap[mintAddress]
}

// SetTradeInfo 设置指定代币的交易信息
func SetTradeInfo(mintAddress string, info *TradeEventInfo) {
	TradeInfoMapLock.Lock()
	defer TradeInfoMapLock.Unlock()
	TradeInfoMap[mintAddress] = info

	// 打印当前设置的交易信息
	logger.Debugf("设置交易信息: Mint=%s, 价格=%.8f, 用户=%s",
		mintAddress, info.CurrentPrice, info.UserAddress)

	// 打印TradeInfoMap中的所有元素
	logger.Debugf("当前TradeInfoMap包含 %d 个元素", len(TradeInfoMap))
	for mint, tradeInfo := range TradeInfoMap {
		logger.Debugf("TradeInfoMap[%s]: 价格=%.8f, 用户=%s, 时间=%s",
			mint, tradeInfo.CurrentPrice, tradeInfo.UserAddress, tradeInfo.TradeTime)
	}
}

// GetPendingHoldAsset 获取指定代币的持有资产信息
func GetPendingHoldAsset(mintAddress string) *MyBuyMap {
	PendingHoldAssetLock.RLock()
	defer PendingHoldAssetLock.RUnlock()
	return PendingHoldAsset[mintAddress]
}

// SetPendingHoldAsset 设置指定代币的持有资产信息
func SetPendingHoldAsset(mintAddress string, info *MyBuyMap) {
	PendingHoldAssetLock.Lock()
	defer PendingHoldAssetLock.Unlock()
	PendingHoldAsset[mintAddress] = info
}

// DeletePendingHoldAsset 删除指定代币的持有资产信息
func DelPendingHoldAsset(mintAddress string) {
	PendingHoldAssetLock.Lock()
	defer PendingHoldAssetLock.Unlock()
	delete(PendingHoldAsset, mintAddress)
}

// GetCreateInfo 获取指定代币的创建信息
func GetCreateInfo(mintAddress string) *CreateEventInfo {
	CreateInfoMapLock.RLock()
	defer CreateInfoMapLock.RUnlock()
	return CreateInfoMap[mintAddress]
}

// SetCreateInfo 设置指定代币的创建信息
func SetCreateInfo(mintAddress string, info *CreateEventInfo) {
	CreateInfoMapLock.Lock()
	defer CreateInfoMapLock.Unlock()
	CreateInfoMap[mintAddress] = info
}

// GetAmmTradeInfo 获取指定代币的AMM交易信息
func GetAmmTradeInfo(mintAddress string) *AmmTradeEventInfo {
	AmmTradeInfoMapLock.RLock()
	defer AmmTradeInfoMapLock.RUnlock()
	return AmmTradeInfoMap[mintAddress]
}

// SetAmmTradeInfo 设置指定代币的AMM交易信息
func SetAmmTradeInfo(mintAddress string, info *AmmTradeEventInfo) {
	AmmTradeInfoMapLock.Lock()
	defer AmmTradeInfoMapLock.Unlock()
	AmmTradeInfoMap[mintAddress] = info
}

// GetAmmPoolInfo 获取指定代币的AMM池信息
func GetAmmPoolInfo(mintAddress string) *AmmPoolInfo {
	AmmPoolInfoMapLock.RLock()
	defer AmmPoolInfoMapLock.RUnlock()
	return AmmPoolInfoMap[mintAddress]
}

// SetAmmPoolInfo 设置指定代币的AMM池信息
func SetAmmPoolInfo(mintAddress string, info *AmmPoolInfo) {
	AmmPoolInfoMapLock.Lock()
	defer AmmPoolInfoMapLock.Unlock()
	AmmPoolInfoMap[mintAddress] = info
}

// CanSellToken 检查是否可以卖出代币（5秒内同一个币最多卖出一次）
func CanSellToken(mintAddress string) bool {
	TokenSellLimitersLock.RLock()
	defer TokenSellLimitersLock.RUnlock()
	limiter, exists := TokenSellLimiters[mintAddress]

	if !exists {
		// 如果没有该代币的限制记录，可以卖出
		return true
	}

	// 检查上次卖出时间是否超过限制时间
	elapsed := time.Since(limiter.LastSellTime)

	return elapsed > SellIntervalLimit
}

// UpdateTokenSellTime 更新代币卖出时间
func UpdateTokenSellTime(mintAddress string) {
	TokenSellLimitersLock.Lock()
	defer TokenSellLimitersLock.Unlock()

	// 更新或创建限制器
	TokenSellLimiters[mintAddress] = &TokenSellLimiter{
		LastSellTime: time.Now(),
	}
	logger.Infof("更新代币 %s 卖出时间限制，下次可卖出时间: %v", mintAddress, time.Now().Add(SellIntervalLimit).Format("15:04:05.000"))
}
