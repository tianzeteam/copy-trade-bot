package bot

import (
	"context"
	"sort"
	"sync"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/types"
	"copytrade/utils"

	"github.com/gagliardetto/solana-go"
)

// SellStrategy 定义代币卖出策略的接口
type SellStrategy interface {
	// CalculateSellBatches 计算如何将代币拆分为多个卖出批次
	CalculateSellBatches(tokenInfo queue.TokenInfo) []SellBatch
	// Name 返回策略名称
	Name() string
}

// SellBatch 表示多批次卖出策略中的单个批次
type SellBatch struct {
	Amount       uint64  // 本批次卖出的代币数量
	PricePercent float64 // 价格阈值，以入场价格的百分比表示
}

// BatchSellStrategy 实现了将代币分批卖出的策略
type BatchSellStrategy struct {
	NumBatches int       // 分批数量
	Thresholds []float64 // 每批的价格阈值倍数
}

// MultiLevelSellStrategy 实现了多级卖出策略
type MultiLevelSellStrategy struct {
	Levels        []utils.MultiSellLevel  // 多级卖出配置
	SoldLevelsMap map[string]map[int]bool // 记录每个代币的已卖出档位
	Lock          sync.RWMutex            // 用于保护 SoldLevelsMap
}

// NewDefaultBatchSellStrategy 创建一个默认的批量卖出策略
func NewDefaultBatchSellStrategy() SellStrategy {
	// 获取配置
	config := GetConfig()

	// 检查是否使用多级卖出模式
	if config != nil && config.SellMode == "multi_sell" && len(config.MultiSellLevels) > 0 {
		// 使用多级卖出模式
		return NewMultiLevelSellStrategy(config.MultiSellLevels)
	}

	// 默认批量卖出策略值
	defaultNumBatches := 5
	defaultThresholds := []float64{1.2, 1.5, 2.0, 3.0, 5.0} // 价格阈值（百分比）：1.2x, 1.5x, 2x, 3x, 5x

	// 如果配置文件中有设置，则使用配置文件中的值
	if config != nil && config.BatchSellConfig.NumBatches > 0 {
		defaultNumBatches = config.BatchSellConfig.NumBatches
		// 如果配置了阈值，则使用配置的阈值
		if len(config.BatchSellConfig.Thresholds) > 0 {
			defaultThresholds = make([]float64, len(config.BatchSellConfig.Thresholds))
			for i, threshold := range config.BatchSellConfig.Thresholds {
				defaultThresholds[i] = float64(threshold) / 100.0
			}
		}
	}

	return &BatchSellStrategy{
		NumBatches: defaultNumBatches,
		Thresholds: defaultThresholds,
	}
}

// Name 返回策略名称
func (s *BatchSellStrategy) Name() string {
	return "批量卖出策略"
}

// 全局配置和锁
var (
	config     *utils.Config
	configLock sync.RWMutex
)

// SetConfig 设置全局配置
func SetConfig(cfg *utils.Config) {
	configLock.Lock()
	defer configLock.Unlock()
	config = cfg
}

// GetConfig 获取全局配置
func GetConfig() *utils.Config {
	configLock.RLock()
	defer configLock.RUnlock()
	return config
}

// CalculateSellBatches 实现 SellStrategy 接口
func (s *BatchSellStrategy) CalculateSellBatches(tokenInfo queue.TokenInfo) []SellBatch {
	var batches []SellBatch

	// 计算每批数量
	totalAmount := tokenInfo.Amount
	batchSize := totalAmount / uint64(s.NumBatches)

	// 生成前 N-1 批
	for i := 0; i < s.NumBatches-1; i++ {
		batches = append(batches, SellBatch{
			Amount:       batchSize,
			PricePercent: s.Thresholds[i],
		})
	}

	// 最后一批使用剩余的所有代币
	remainingAmount := totalAmount - (batchSize * uint64(s.NumBatches-1))
	batches = append(batches, SellBatch{
		Amount:       remainingAmount,
		PricePercent: s.Thresholds[s.NumBatches-1],
	})

	return batches
}

// NewMultiLevelSellStrategy 创建一个多级卖出策略
func NewMultiLevelSellStrategy(levels []utils.MultiSellLevel) SellStrategy {
	return &MultiLevelSellStrategy{
		Levels:        levels,
		SoldLevelsMap: make(map[string]map[int]bool),
		Lock:          sync.RWMutex{},
	}
}

// Name 返回策略名称
func (s *MultiLevelSellStrategy) Name() string {
	return "多级卖出策略"
}

// CalculateSellBatches 实现 SellStrategy 接口
func (s *MultiLevelSellStrategy) CalculateSellBatches(tokenInfo queue.TokenInfo) []SellBatch {
	// 计算价格涨幅百分比
	entryPrice := tokenInfo.EntryPrice
	currentPrice := tokenInfo.CurrentPrice
	priceIncreasePercent := (currentPrice - entryPrice) / entryPrice * 100
	logger.Infof("当前价格: %f, 入场价格: %f, 涨幅: %.2f%%", currentPrice, entryPrice, priceIncreasePercent)

	// 获取代币地址
	mintAddress := tokenInfo.Mint.String()

	// 将多级刮利水平按价格涨幅百分比从高到低排序
	sellLevels := make([]utils.MultiSellLevel, len(s.Levels))
	copy(sellLevels, s.Levels)
	sort.Slice(sellLevels, func(i, j int) bool {
		return sellLevels[i].PricePercent > sellLevels[j].PricePercent
	})

	// 检查并初始化该代币的已卖出档位记录
	s.Lock.Lock()
	if _, exists := s.SoldLevelsMap[mintAddress]; !exists {
		s.SoldLevelsMap[mintAddress] = make(map[int]bool)
	}

	// 根据配置的多级刮利水平计算应卖出的比例
	var selectedLevel *utils.MultiSellLevel = nil
	var thisSellPercent float64 = 0

	for i, level := range sellLevels {
		// 检查是否满足涨幅条件且该档位还未卖出
		if priceIncreasePercent >= float64(level.PricePercent) && !s.SoldLevelsMap[mintAddress][level.PricePercent] {
			thisSellPercent = float64(level.SellPercent)
			selectedLevel = &sellLevels[i]
			logger.Infof("满足卖出条件: 价格涨幅 %.2f%% >= %d%%, 卖出 %d%% 代币 (档位未卖出)",
				priceIncreasePercent, level.PricePercent, level.SellPercent)
			break
		} else if priceIncreasePercent >= float64(level.PricePercent) && s.SoldLevelsMap[mintAddress][level.PricePercent] {
			logger.Infof("跳过已卖出档位: 价格涨幅 %.2f%% >= %d%%, 档位已卖出", priceIncreasePercent, level.PricePercent)
		}
	}

	// 如果找到了满足条件的档位，标记为已卖出
	if selectedLevel != nil {
		s.SoldLevelsMap[mintAddress][selectedLevel.PricePercent] = true
		logger.Infof("已标记价格档位 %d%% 为已卖出", selectedLevel.PricePercent)
	}
	s.Lock.Unlock()

	// 限制最大卖出比例为100%
	if thisSellPercent > 100 {
		thisSellPercent = 100
	}

	// 如果没有满足任何卖出条件，返回空批次
	if thisSellPercent <= 0 {
		logger.Infof("没有满足任何卖出条件，不进行卖出")
		return []SellBatch{}
	}

	// 计算实际卖出数量
	sellAmount := uint64(float64(tokenInfo.Amount) * thisSellPercent / 100)
	logger.Infof("卖出比例: %.2f%%, 卖出数量: %d/%d", thisSellPercent, sellAmount, tokenInfo.Amount)

	// 如果卖出数量小于10，不进行卖出
	if sellAmount <= 10 {
		logger.Infof("计算的卖出数量小于10，不进行卖出")
		return []SellBatch{}
	}

	// 创建一个卖出批次
	return []SellBatch{
		{
			Amount:       sellAmount,
			PricePercent: float64(selectedLevel.PricePercent), // 立即以当前价格卖出
		},
	}
}

// ExecuteSellStrategy 执行卖出策略
func (b *Bot) ExecuteSellStrategy(tokenInfo queue.TokenInfo, vTokenReserves, vSolReserves uint64) {
	// 使用策略计算卖出批次
	batches := b.sellStrategy.CalculateSellBatches(tokenInfo)

	logger.Infof("使用 %s 卖出策略，共 %d 个批次", b.sellStrategy.Name(), len(batches))

	// 如果没有批次需要卖出，直接返回
	if len(batches) == 0 {
		return
	}

	// 处理每个批次
	for _, batch := range batches {
		logger.Infof("执行卖出: 数量=%d, 当前价格=%.6f", batch.Amount, tokenInfo.CurrentPrice)

		// 执行此批次的卖出订单
		sellTokenInfo := tokenInfo
		sellTokenInfo.Amount = batch.Amount
		b.ExecuteSellOrder(sellTokenInfo, vTokenReserves, vSolReserves)
	}
}

// executeSellOrder 处理实际的pump 内盘卖出逻辑
func (b *Bot) ExecuteSellOrder(tokenInfo queue.TokenInfo, vTokenReserves, vSolReserves uint64) {
	// 实际的卖出逻辑
	logger.Infof("执行卖出订单: 代币=%s, 数量=%d, time: %v", tokenInfo.Mint.String(), tokenInfo.Amount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 先计算卖出金额
	pumpAccounts, err := b.Trader.AccountsCalc.CalculatePumpAccounts(context.Background(), tokenInfo.Creator, tokenInfo.Mint, tokenInfo.Owner)
	if err != nil {
		logger.Errorf("计算卖出金额失败: %v", err)
		return
	}
	//specailScheme := global.GetConfigByWallet(tokenInfo.Owner.String())
	// 计算卖出金额
	amounts, err := b.Trader.AmountsCalc.CalculateSellAmounts(
		context.Background(),
		pumpAccounts.BondingCurve,
		float64(tokenInfo.Amount),
		vTokenReserves,
		vSolReserves,
		global.GetConfig().SellSlippage,
	)
	if err != nil {
		logger.Errorf("计算卖出金额失败: %v", err)
		return
	}
	//构建交易基础参数
	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:                   tokenInfo.Owner,
			Mint:                   tokenInfo.Mint,
			BondingCurve:           pumpAccounts.BondingCurve,
			AssociatedBondingCurve: pumpAccounts.AssociatedBondingCurve,
			AssociatedTokenAccount: pumpAccounts.AssociatedTokenAccount,
			CreatorVault:           pumpAccounts.CreatorVaultPda,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MinOut, // 卖出时 Cost 表示最小获得数量
		Blockhash: *b.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    global.GetConfig().SellSlippage,
			PriorityFee: uint64(global.GetConfig().SellPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    b.Config.Zeroslot.Enable,
			RpcUrl:    b.Config.Zeroslot.RpcUrl,
			BundleUrl: b.Config.Zeroslot.RpcUrl,
			ApiKeys:   b.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		NextBlockParams: &types.NextBlockParams{
			Enable:    b.Config.NextBlock.Enable,
			BundleUrl: b.Config.NextBlock.RpcUrl,
			ApiKey:    b.Config.NextBlock.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		JitoParams: &types.JitoParams{
			Enable:    b.Config.Jito.Enable,
			BundleUrl: b.Config.Jito.RpcUrl,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    b.Config.Temporal.Enable,
			BundleUrl: b.Config.Temporal.RpcUrl,
			ApiKey:    b.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    b.Config.BloXroute.Enable,
			RpcUrl:    b.Config.BloXroute.RpcUrl,
			ApiKeys:   b.Config.BloXroute.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
	}

	// 调用交易接口执行卖出
	b.Trader.Sell(context.Background(), orderParams)
}

// ExecuteSellStrategy 执行卖出策略
func (b *Bot) ExecuteAmmSellStrategy(tokenInfo queue.TokenInfo) {
	// 使用策略计算卖出批次
	batches := b.sellStrategy.CalculateSellBatches(tokenInfo)

	logger.Infof("使用 %s 卖出策略，共 %d 个批次", b.sellStrategy.Name(), len(batches))

	// 如果没有批次需要卖出，直接返回
	if len(batches) == 0 {
		return
	}

	// 处理每个批次
	for _, batch := range batches {
		logger.Infof("执行卖出: 数量=%d, 当前价格=%.6f", batch.Amount, tokenInfo.CurrentPrice)

		// 执行此批次的卖出订单
		sellTokenInfo := tokenInfo
		sellTokenInfo.Amount = batch.Amount
		b.ExecuteAmmSellOrder(sellTokenInfo)
	}
}

// executeSellOrder 处理实际的卖出逻辑
func (b *Bot) ExecuteAmmSellOrder(tokenInfo queue.TokenInfo) {
	// 实际的卖出逻辑
	var baseAmountIn uint64 = tokenInfo.Amount
	logger.Infof("开始处理 PumpAMM 平台的代币卖出操作: %s, 数量=%d", tokenInfo.Mint.String(), baseAmountIn)
	// 获取交易所需的参数
	mintAddress := tokenInfo.Mint.String()

	// 尝试获取交易池信息
	tradeInfo := global.GetAmmTradeInfo(mintAddress)

	if tradeInfo == nil {
		logger.Errorf("未找到代币 %s 的交易信息或创建信息，无法执行卖出操作", mintAddress)
		return
	}

	ammAccounts, err := b.Trader.AccountsCalc.CalculateAmmAccounts(

		tradeInfo.AmmPool,
		tradeInfo.MintAddress,
		b.privateKey.PublicKey(),
		tradeInfo.CoinCreator,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}
	baseReserve := tradeInfo.BaseReserve
	quoteReserve := tradeInfo.QuoteReserve
	// Calculate buy amounts
	amounts, err := b.Trader.AmountsCalc.CalculateAmmSellAmounts(
		baseAmountIn,
		baseReserve,
		quoteReserve,
		global.GetConfig().SellSlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}
	global.PendingHoldAssetLock.RLock()
	myBuyMap := global.PendingHoldAsset[mintAddress]
	global.PendingHoldAssetLock.RUnlock()
	if myBuyMap == nil {
		logger.Errorf("未找到代币 %s 的买入信息，无法执行卖出操作", mintAddress)
		return
	}
	// 构建卖出参数

	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:                      tokenInfo.User,
			Mint:                      ammAccounts.Mint,
			BaseMint:                  ammAccounts.BaseMint,
			QuoteMint:                 ammAccounts.QuoteMint,
			UserBaseTokenAccount:      ammAccounts.UserBaseTokenAccount,
			UserQuoteTokenAccount:     ammAccounts.UserQuoteTokenAccount,
			PoolBaseTokenAccount:      ammAccounts.PoolBaseTokenAccount,
			PoolQuoteTokenAccount:     ammAccounts.PoolQuoteTokenAccount,
			Pool:                      ammAccounts.Pool,
			CreatorVault:              ammAccounts.CoinCreatorVaultAta,
			CoinCreatorVaultAuthority: ammAccounts.CoinCreatorVaultAuthority,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MinOut,
		Blockhash: *b.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    global.GetConfig().SellSlippage,
			PriorityFee: uint64(global.GetConfig().SellPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    b.Config.Zeroslot.Enable,
			RpcUrl:    b.Config.Zeroslot.RpcUrl,
			BundleUrl: b.Config.Zeroslot.RpcUrl,
			ApiKeys:   b.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		NextBlockParams: &types.NextBlockParams{
			Enable:    b.Config.NextBlock.Enable,
			BundleUrl: b.Config.NextBlock.RpcUrl,
			ApiKey:    b.Config.NextBlock.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		JitoParams: &types.JitoParams{
			Enable:    b.Config.Jito.Enable,
			BundleUrl: b.Config.Jito.RpcUrl,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    b.Config.Temporal.Enable,
			BundleUrl: b.Config.Temporal.RpcUrl,
			ApiKey:    b.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    b.Config.BloXroute.Enable,
			RpcUrl:    b.Config.BloXroute.RpcUrl,
			ApiKeys:   b.Config.BloXroute.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
	}

	// 调用通用的卖出方法
	b.Trader.AmmSell(context.Background(), orderParams, b.balanceNeeded)

}

// executeSellOrder 处理实际的Raydium v4 内盘卖出逻辑
func (b *Bot) ExecuteRaydiumV4SellOrder(tokenInfo queue.TokenInfo) {
	// 实际的卖出逻辑
	logger.Infof("执行卖出订单: 代币=%s, 数量=%d", tokenInfo.Mint.String(), tokenInfo.Amount)

	// 从全局缓存中获取Raydium V4交易信息
	tradeInfo := global.GetAmmRaydiuV4TradeInfo(tokenInfo.Mint.String())
	if tradeInfo == nil {
		logger.Errorf("找不到代币 %s 的Raydium V4交易信息", tokenInfo.Mint.String())
		return
	}

	// 从全局缓存中获取市场状态
	marketState := global.GetOpenBookMarketState(tradeInfo.MarketId.String())
	if marketState == nil {
		logger.Errorf("找不到市场 %s 的状态信息", tradeInfo.MarketId.String())
		return
	}

	// 从全局缓存中获取池状态
	poolLayout := global.GetRaydiumPoolLayout(tradeInfo.AmmPool.String())
	if poolLayout == nil {
		logger.Errorf("找不到池 %s 的状态信息", tradeInfo.AmmPool.String())
		return
	}

	// 计算账户信息
	raydiumV4Accounts, err := b.Trader.AccountsCalc.CalculateV4Accounts(
		b.privateKey.PublicKey(),
		tradeInfo,
		marketState,
		poolLayout,
	)
	if err != nil {
		logger.Errorf("计算卖出金额失败: %v", err)
		return
	}

	// 计算卖出金额
	amounts, err := b.Trader.AmountsCalc.CalculateRaydiumV4SellAmounts(

		float64(tokenInfo.Amount),
		tradeInfo.CurrentPrice,
		poolLayout.BaseDecimal,
		global.GetConfig().SellSlippage,
	)
	if err != nil {
		logger.Errorf("计算卖出金额失败: %v", err)
		return
	}
	//构建交易基础参数
	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:              b.privateKey.PublicKey(),
			Mint:              raydiumV4Accounts.BaseMint,
			BaseMint:          raydiumV4Accounts.BaseMint,
			QuoteMint:         raydiumV4Accounts.QuoteMint,
			RaydiumV4Accounts: raydiumV4Accounts,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MinOut,
		Blockhash: *b.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    global.GetConfig().SellSlippage,
			PriorityFee: uint64(global.GetConfig().SellPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    b.Config.Zeroslot.Enable,
			RpcUrl:    b.Config.Zeroslot.RpcUrl,
			BundleUrl: b.Config.Zeroslot.RpcUrl,
			ApiKeys:   b.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		NextBlockParams: &types.NextBlockParams{
			Enable:    b.Config.NextBlock.Enable,
			BundleUrl: b.Config.NextBlock.RpcUrl,
			ApiKey:    b.Config.NextBlock.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		JitoParams: &types.JitoParams{
			Enable:    b.Config.Jito.Enable,
			BundleUrl: b.Config.Jito.RpcUrl,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    b.Config.Temporal.Enable,
			BundleUrl: b.Config.Temporal.RpcUrl,
			ApiKey:    b.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    b.Config.BloXroute.Enable,
			RpcUrl:    b.Config.BloXroute.RpcUrl,
			ApiKeys:   b.Config.BloXroute.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
	}

	// 调用交易接口执行卖出
	b.Trader.RaydiumV4Sell(context.Background(), orderParams)
}

// 处理实际的Raydium Launchpad 内盘卖出逻辑
func (b *Bot) ExecuteRaydiumLaunchpadSellOrder(tokenInfo queue.TokenInfo) {
	// 实际的卖出逻辑
	logger.Infof("执行卖出订单: 代币=%s, 数量=%d", tokenInfo.Mint.String(), tokenInfo.Amount)

	// 从全局缓存中获取Raydium V4交易信息
	tradeInfo := global.GetLaunchpadTradeInfo(tokenInfo.Mint.String())
	if tradeInfo == nil {
		logger.Errorf("找不到代币 %s 的Raydium V4交易信息", tokenInfo.Mint.String())
		return
	}

	// 计算账户信息
	raydiumLaunchpadAccounts, err := b.Trader.AccountsCalc.CalculateLaunchpadAccounts(
		b.privateKey.PublicKey(),
		tokenInfo.Mint,
		solana.SolMint,
	)
	if err != nil {
		logger.Errorf("计算卖出金额失败: %v", err)
		return
	}

	// 计算卖出金额
	amounts, err := b.Trader.AmountsCalc.CalculateRaydiumV4SellAmounts(

		float64(tokenInfo.Amount),
		tradeInfo.CurrentPrice,
		6,
		global.GetConfig().SellSlippage,
	)
	if err != nil {
		logger.Errorf("计算卖出金额失败: %v", err)
		return
	}
	//构建交易基础参数
	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:              b.privateKey.PublicKey(),
			Mint:              raydiumLaunchpadAccounts.BaseTokenMint,
			BaseMint:          raydiumLaunchpadAccounts.BaseTokenMint,
			QuoteMint:         raydiumLaunchpadAccounts.QuoteTokenMint,
			LaunchpadAccounts: raydiumLaunchpadAccounts,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MinOut,
		Blockhash: *b.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    global.GetConfig().SellSlippage,
			PriorityFee: uint64(global.GetConfig().SellPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    b.Config.Zeroslot.Enable,
			RpcUrl:    b.Config.Zeroslot.RpcUrl,
			BundleUrl: b.Config.Zeroslot.RpcUrl,
			ApiKeys:   b.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		NextBlockParams: &types.NextBlockParams{
			Enable:    b.Config.NextBlock.Enable,
			BundleUrl: b.Config.NextBlock.RpcUrl,
			ApiKey:    b.Config.NextBlock.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		JitoParams: &types.JitoParams{
			Enable:    b.Config.Jito.Enable,
			BundleUrl: b.Config.Jito.RpcUrl,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    b.Config.Temporal.Enable,
			BundleUrl: b.Config.Temporal.RpcUrl,
			ApiKey:    b.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    b.Config.BloXroute.Enable,
			RpcUrl:    b.Config.BloXroute.RpcUrl,
			ApiKeys:   b.Config.BloXroute.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
	}

	// 调用交易接口执行卖出
	b.Trader.RaydiumLaunchpadSell(context.Background(), orderParams)
}

func (b *Bot) ExecuteCpmmSellOrder(tokenInfo queue.TokenInfo) {
	// 实际的卖出逻辑
	logger.Infof("执行CPMM卖出订单: 代币=%s, 数量=%d", tokenInfo.Mint.String(), tokenInfo.Amount)

	// 从全局缓存中获取CPMM交易信息
	tradeInfo := global.GetCpmmTradeInfo(tokenInfo.Mint.String())
	if tradeInfo == nil {
		logger.Errorf("找不到代币 %s 的CPMM交易信息", tokenInfo.Mint.String())
		return
	}

	// 计算账户信息
	raydiumCpmmAccounts, err := b.Trader.AccountsCalc.CalculateRaydiumCPMMAccounts(
		b.privateKey.PublicKey(),
		tradeInfo.AmmConfig,
		tradeInfo.PoolState,
		tokenInfo.Mint,
		solana.SolMint,
	)
	if err != nil {
		logger.Errorf("计算CPMM账户信息失败: %v", err)
		return
	}

	// 计算卖出金额
	amounts, err := b.Trader.AmountsCalc.CalculateRaydiumV4SellAmounts(

		float64(tokenInfo.Amount),
		tradeInfo.CurrentPrice,
		6,
		global.GetConfig().SellSlippage,
	)
	if err != nil {
		logger.Errorf("计算CPMM金额失败: %v", err)
		return
	}
	//构建交易基础参数
	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:                b.privateKey.PublicKey(),
			Mint:                raydiumCpmmAccounts.OutputTokenMint,
			BaseMint:            raydiumCpmmAccounts.OutputTokenMint,
			QuoteMint:           raydiumCpmmAccounts.InputTokenMint,
			RaydiumCPMMAccounts: raydiumCpmmAccounts,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MinOut,
		Blockhash: *b.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    global.GetConfig().SellSlippage,
			PriorityFee: uint64(global.GetConfig().SellPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    b.Config.Zeroslot.Enable,
			RpcUrl:    b.Config.Zeroslot.RpcUrl,
			BundleUrl: b.Config.Zeroslot.RpcUrl,
			ApiKeys:   b.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		NextBlockParams: &types.NextBlockParams{
			Enable:    b.Config.NextBlock.Enable,
			BundleUrl: b.Config.NextBlock.RpcUrl,
			ApiKey:    b.Config.NextBlock.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		JitoParams: &types.JitoParams{
			Enable:    b.Config.Jito.Enable,
			BundleUrl: b.Config.Jito.RpcUrl,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    b.Config.Temporal.Enable,
			BundleUrl: b.Config.Temporal.RpcUrl,
			ApiKey:    b.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    b.Config.BloXroute.Enable,
			RpcUrl:    b.Config.BloXroute.RpcUrl,
			ApiKeys:   b.Config.BloXroute.ApiKeys,
			TipAmount: uint64(global.GetConfig().SellTipFee * 1e9),
		},
	}

	// 调用交易接口执行卖出
	b.Trader.RaydiumCPMMSell(context.Background(), orderParams)
}
