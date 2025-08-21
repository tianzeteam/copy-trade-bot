package global

import (
	"copytrade/logger"
	"copytrade/raydium/layouts"
	"copytrade/types"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
)

// TradeEventInfo 存储交易事件信息
type TradeEventInfo struct {
	UserAddress          string // 用户地址
	SolAmount            uint64 // Sol数量
	TokenAmount          uint64 // Token数量
	IsBuy                bool   // 是否买入
	MintAddress          string // Mint地址
	Creator              solana.PublicKey
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
	Decimal       uint32
	UserAddress   string
	MintAddress   string
	Creator       solana.PublicKey
	EntryPrice    float64
	InitBuyAmount uint64
	Amount        uint64
	SoldLevels    map[int]bool // 记录已经卖出的价格档位
	TimeStamp     int64
}

// CreateEventInfo 存储代币创建事件信息
type CreateEventInfo struct {
	Name         string
	Symbol       string
	Uri          string
	MintAddress  string
	BondingCurve string
	Creator      solana.PublicKey
	CreatedAt    time.Time
}

// AmmTradeEventInfo 存储AMM交易事件信息
type AmmTradeEventInfo struct {
	UserAddress  solana.PublicKey // 用户地址
	SolAmount    uint64           // Sol数量
	TokenAmount  uint64           // Token数量
	IsBuy        bool             // 是否买入
	MintAddress  solana.PublicKey // Mint地址
	Creator      solana.PublicKey // 创建者地址
	AmmPool      solana.PublicKey // AMM池地址
	TimeStamp    int64            // 时间戳
	SolReserve   uint64           // SOL储备
	BaseReserve  uint64           // 基础代币储备
	QuoteReserve uint64           // 报价代币储备
	CurrentPrice float64          // 当前价格
	TradeTime    string           // 交易时间
	ReceivedAt   time.Time        // 接收时间
	TxHash       string           // 交易哈希
	Slot         uint64           // 区块槽位
	CoinCreator  solana.PublicKey
	Router       string
}

// LaunchpadTradeEventInfo 存储Launchpad交易事件信息
type LaunchpadTradeEventInfo struct {
	UserAddress  solana.PublicKey // 用户地址
	SolAmount    uint64           // Sol数量
	TokenAmount  uint64           // Token数量
	IsBuy        bool             // 是否买入
	MintAddress  solana.PublicKey // Mint地址
	PoolState    solana.PublicKey // 池状态地址
	SolReserve   uint64           // SOL储备
	BaseReserve  uint64           // 基础代币储备
	QuoteReserve uint64           // 报价代币储备
	ReceivedAt   time.Time        // 接收时间
	CurrentPrice float64          // 当前价格
	Router       string
}

// CPMMTradeEventInfo 存储CPMM交易事件信息
type CPMMTradeEventInfo struct {
	AmmConfig    string
	UserAddress  solana.PublicKey // 用户地址
	SolAmount    uint64           // Sol数量
	TokenAmount  uint64           // Token数量
	IsBuy        bool             // 是否买入
	MintAddress  solana.PublicKey // Mint地址
	PoolState    solana.PublicKey // 池状态地址
	SolReserve   uint64           // SOL储备
	BaseReserve  uint64           // 基础代币储备
	QuoteReserve uint64           // 报价代币储备
	ReceivedAt   time.Time        // 接收时间
	CurrentPrice float64          // 当前价格
	Router       string
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

// NewMarketStateFromLayout creates a new MarketState from a MarketStateLayoutV3
func NewMarketStateFromLayout(marketID solana.PublicKey, layout layouts.MarketStateLayoutV3) *types.MarketState {
	return &types.MarketState{
		MarketID:         marketID,
		Layout:           layout,
		BaseMint:         layout.BaseMint,
		QuoteMint:        layout.QuoteMint,
		BaseVault:        layout.BaseVault,
		QuoteVault:       layout.QuoteVault,
		Bids:             layout.Bids,
		Asks:             layout.Asks,
		BaseLotSize:      layout.BaseLotSize,
		QuoteLotSize:     layout.QuoteLotSize,
		FeeRateBps:       layout.FeeRateBps,
		MarketEventQueue: layout.EventQueue,
	}
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

	// AmmRaydiumV4 全局AMM交易信息映射表，键为Mint地址
	AmmRaydiumV4TradeMap     = make(map[string]*types.AmmRaydiumV4TradeInfo)
	AmmRaydiumV4TradeMapLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	CpmmTradeInfoMap     = make(map[string]*CPMMTradeEventInfo)
	CpmmTradeInfoMapLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// LaunchpadTradeEventMap 存储Launchpad交易事件信息，键为池地址
	LaunchpadTradeEventMap     = make(map[string]*LaunchpadTradeEventInfo)
	LaunchpadTradeEventMapLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// TokenSellLimiters 代币卖出限制器映射表，键为Mint地址
	TokenSellLimiters     = make(map[string]*TokenSellLimiter)
	TokenSellLimitersLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// OpenBookMarketStates 存储OpenBook市场状态信息，键为市场ID
	OpenBookMarketStates     = make(map[string]*types.MarketState)
	OpenBookMarketStatesLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// RaydiumPoolLayouts 存储Raydium池布局信息，键为池地址
	RaydiumPoolLayouts     = make(map[string]*layouts.LIQUIDITY_STATE_LAYOUT_V4)
	RaydiumPoolLayoutsLock = &sync.RWMutex{} // 用于保护映射表的读写锁

	// 代币卖出时间间隔限制，默认2秒

	SellIntervalLimit = 2 * time.Second
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

}

// GetPendingHoldAsset 获取指定代币的持有资产信息
func GetPendingHoldAsset(mintAddress string) (*MyBuyMap, bool) {
	PendingHoldAssetLock.RLock()
	defer PendingHoldAssetLock.RUnlock()
	asset, ok := PendingHoldAsset[mintAddress]
	return asset, ok
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

// DelPendingHoldAsseWithAmount 根据指定数量减少持有资产，进行仓位管理
// 如果减少后数量为0，则完全删除该资产；否则更新剩余数量
func DelPendingHoldAssetWithAmount(mintAddress string, amount uint64) {
	PendingHoldAssetLock.Lock()
	defer PendingHoldAssetLock.Unlock()

	asset, exists := PendingHoldAsset[mintAddress]
	if !exists {
		logger.Warnf("尝试减少不存在的持仓资产: %s", mintAddress)
		return
	}

	// 如果要减少的数量大于等于当前持有量，则完全删除
	if amount >= asset.Amount {
		logger.Infof("完全删除持仓资产: %s, 原数量: %d", mintAddress, asset.Amount)
		delete(PendingHoldAsset, mintAddress)
		return
	}

	// 否则减少持有量
	asset.Amount -= amount
	logger.Infof("部分减少持仓资产: %s, 减少数量: %d, 剩余数量: %d", mintAddress, amount, asset.Amount)
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

// SetCpmmTradeInfo sets the CPMM trade info for a given mint address.
func SetCpmmTradeInfo(mintAddress string, info *CPMMTradeEventInfo) {
	CpmmTradeInfoMapLock.Lock()
	defer CpmmTradeInfoMapLock.Unlock()
	CpmmTradeInfoMap[mintAddress] = info
}

// GetCpmmTradeInfo gets the CPMM trade info for a given mint address.
func GetCpmmTradeInfo(mintAddress string) *CPMMTradeEventInfo {
	CpmmTradeInfoMapLock.RLock()
	defer CpmmTradeInfoMapLock.RUnlock()
	return CpmmTradeInfoMap[mintAddress]
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

// SetRaydiumV4TradeInfo 设置指定代币的AMM交易信息
func SetRaydiumV4TradeInfo(mintAddress string, info *types.AmmRaydiumV4TradeInfo) {
	AmmRaydiumV4TradeMapLock.Lock()
	defer AmmRaydiumV4TradeMapLock.Unlock()
	AmmRaydiumV4TradeMap[mintAddress] = info
}

// SetLaunchpadTradeInfo 设置指定代币的Launchpad交易信息
func SetLaunchpadTradeInfo(mintAddress string, info *LaunchpadTradeEventInfo) {
	LaunchpadTradeEventMapLock.Lock()
	defer LaunchpadTradeEventMapLock.Unlock()
	LaunchpadTradeEventMap[mintAddress] = info
}

// GetLaunchpadTradeInfo 获取指定代币的Launchpad交易信息
func GetLaunchpadTradeInfo(mintAddress string) *LaunchpadTradeEventInfo {
	LaunchpadTradeEventMapLock.RLock()
	defer LaunchpadTradeEventMapLock.RUnlock()
	return LaunchpadTradeEventMap[mintAddress]
}

// GetAmmRaydiuV4TradeInfo 获取指定代币的AMM交易信息
func GetAmmRaydiuV4TradeInfo(mintAddress string) *types.AmmRaydiumV4TradeInfo {
	AmmRaydiumV4TradeMapLock.Lock()
	defer AmmRaydiumV4TradeMapLock.Unlock()
	return AmmRaydiumV4TradeMap[mintAddress]
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

	return elapsed > time.Duration(GetConfig().SellIntervalLimit)*time.Second
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

// GetOpenBookMarketState 获取指定市场ID的OpenBook市场状态
func GetOpenBookMarketState(marketID string) *types.MarketState {
	OpenBookMarketStatesLock.RLock()
	defer OpenBookMarketStatesLock.RUnlock()
	return OpenBookMarketStates[marketID]
}

// SetOpenBookMarketState 设置指定市场ID的OpenBook市场状态
func SetOpenBookMarketState(marketID string, state *types.MarketState) {
	OpenBookMarketStatesLock.Lock()
	defer OpenBookMarketStatesLock.Unlock()
	OpenBookMarketStates[marketID] = state

}

// GetRaydiumPoolLayout 获取指定池地址的Raydium池布局
func GetRaydiumPoolLayout(poolAddress string) *layouts.LIQUIDITY_STATE_LAYOUT_V4 {
	RaydiumPoolLayoutsLock.RLock()
	defer RaydiumPoolLayoutsLock.RUnlock()
	return RaydiumPoolLayouts[poolAddress]
}

// SetRaydiumPoolLayout 设置指定池地址的Raydium池布局
func SetRaydiumPoolLayout(poolAddress string, layout *layouts.LIQUIDITY_STATE_LAYOUT_V4) {
	RaydiumPoolLayoutsLock.Lock()
	defer RaydiumPoolLayoutsLock.Unlock()
	RaydiumPoolLayouts[poolAddress] = layout
}

// CreateTokenAssetFromAccount 从代币账户数据创建资产条目
func CreateTokenAssetFromAccount(mint solana.PublicKey, owner solana.PublicKey, amount uint64) *MyBuyMap {
	return &MyBuyMap{
		UserAddress: owner.String(),
		MintAddress: mint.String(),
		EntryPrice:  0,
		Amount:      amount,
		SoldLevels:  make(map[int]bool),
		TimeStamp:   time.Now().Unix(),
	}
}

// AddOrUpdatePendingAsset 添加或更新待处理资产
// 如果代币余额大于阈值，则添加到待处理资产列表，否则从列表中删除
func AddOrUpdatePendingAsset(mint solana.PublicKey, owner solana.PublicKey, amount uint64, threshold uint64) {
	PendingHoldAssetLock.Lock()
	defer PendingHoldAssetLock.Unlock()

	mintStr := mint.String()
	if amount > threshold {
		// 添加或更新资产
		PendingHoldAsset[mintStr] = CreateTokenAssetFromAccount(mint, owner, amount)
		logger.Infof("添加代币到待处理资产: Mint=%s, 所有者=%s, 余额=%d", mint, owner, amount)
	} else {
		// 删除资产
		delete(PendingHoldAsset, mintStr)
		logger.Infof("从待处理资产中删除代币: Mint=%s, 余额=%d", mint, amount)
	}
}

// SetTradeAsset 设置交易资产信息，如果已存在则更新
// 如果资产已存在，则累加数量并计算新的平均成本价
func SetTradeAsset(mintAddress string, creator solana.PublicKey, userAddress string, _entryPrice float64, amount uint64, decimal uint32) bool {
	PendingHoldAssetLock.Lock()
	defer PendingHoldAssetLock.Unlock()

	// 检查资产是否已存在
	existingAsset, exists := PendingHoldAsset[mintAddress]
	if exists {

		// 计算新的总成本和总数量
		totalCost := existingAsset.EntryPrice*float64(existingAsset.Amount) + _entryPrice*float64(amount)
		totalAmount := existingAsset.Amount + amount
		initBuyAmount := existingAsset.InitBuyAmount + amount

		// 更新资产信息
		existingAsset.Amount = totalAmount
		existingAsset.InitBuyAmount = initBuyAmount
		if totalAmount > 0 {
			existingAsset.EntryPrice = totalCost / float64(totalAmount)
		}
		existingAsset.TimeStamp = time.Now().Unix()
		existingAsset.SoldLevels = make(map[int]bool) // 重置卖出记录

		logger.Infof("持仓资产更新： %s 再次买入, 新平均价格=%.8f, 新数量=%d", mintAddress, existingAsset.EntryPrice, existingAsset.Amount)
		return true

	}

	// 创建新的交易资产
	PendingHoldAsset[mintAddress] = &MyBuyMap{
		Decimal:       decimal,
		UserAddress:   userAddress,
		MintAddress:   mintAddress,
		Creator:       creator,
		EntryPrice:    _entryPrice,
		InitBuyAmount: amount,
		Amount:        amount,
		SoldLevels:    make(map[int]bool),
		TimeStamp:     time.Now().Unix(),
	}

	logger.Infof("持仓资产新增： %s 的交易资产, 价格=%.8f, 数量=%d", mintAddress, _entryPrice, amount)
	return true
}
