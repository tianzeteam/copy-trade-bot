package wallet

import (
	"context"
	"copytrade/asset"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/raydium/layouts"
	"time"

	"copytrade/bot"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
)

// 交易监控器
type TransactionMonitor struct {
	TradeBot      *bot.Bot
	WalletMonitor *WalletMonitor   // 钱包监控器
	BotWallet     solana.PublicKey // 机器人钱包地址
	BuySolAmount  float64          // 购买金额（SOL）
}

// 创建新的交易监控器
func NewTransactionMonitor(tradeBot *bot.Bot, walletMonitor *WalletMonitor, botWallet solana.PublicKey, buySolAmount float64) *TransactionMonitor {
	return &TransactionMonitor{
		TradeBot:      tradeBot,
		WalletMonitor: walletMonitor,
		BotWallet:     botWallet,
		BuySolAmount:  buySolAmount,
	}
}

// 处理pump内盘交易事件
func (m *TransactionMonitor) ProcessTransaction(tradeInfo *global.TradeEventInfo) {
	// 获取交易者钱包地址
	copyAddress, err := solana.PublicKeyFromBase58(tradeInfo.UserAddress)
	if err != nil {
		logger.Errorf("解析用户地址失败: %v", err)
		return
	}

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		// 不是我们监控的钱包，忽略
		return
	}
	specailScheme, err := m.WalletMonitor.GetParamSchemeForWallet(copyAddress.String())
	if err != nil {
		logger.Errorf("获取参数模板失败: %v", err)
		return
	}

	if float64(tradeInfo.SolAmount) < (specailScheme.MinFollowTriggerAmount*1e9) || float64(tradeInfo.SolAmount) > (specailScheme.MaxFollowTriggerAmount*1e9) {
		//不在跟卖金额范围内，忽略
		return
	}
	if float64(tradeInfo.RealSolReserves) < (specailScheme.MinPoolSize*1e9) || float64(tradeInfo.RealSolReserves) > (specailScheme.MaxPoolSize*1e9) {
		//不在跟卖池子大小范围内，忽略
		return
	}

	logger.Infof("检测到监控钱包交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 记录跟单钱包的交易资产数据持仓
	holdingsManager := asset.GetHoldingsManager()
	holdingsManager.RecordTokenTransaction(tradeInfo)

	// 执行跟单交易
	m.executeCopyTrade(tradeInfo, copyAddress, specailScheme)
}

// 处理pump内盘聪明钱包卖出交易事件
func (m *TransactionMonitor) ProcessSellTransaction(tradeInfo *global.TradeEventInfo) {
	// 获取交易者钱包地址
	copyAddress, err := solana.PublicKeyFromBase58(tradeInfo.UserAddress)
	if err != nil {
		logger.Errorf("解析用户地址失败: %v", err)
		return
	}

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		// 不是我们监控的钱包，忽略
		return
	}
	if !walletSettings.IsFollowSell {
		// 当前钱包不跟卖，忽略
		return
	}

	logger.Infof("检测到监控钱包卖出交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	assetInfo, ok := global.GetPendingHoldAsset(tradeInfo.MintAddress)
	if !ok {
		logger.Debugf("读取买入资产信息发生异常 ：%s", tradeInfo.MintAddress)
		return
	}
	// 处理跟单钱包的交易资产卖出
	holdingsManager := asset.GetHoldingsManager()
	holdingsManager.RecordTokenTransaction(tradeInfo)

	// 计算跟单的同比例卖出
	// 1. 获取监控钱包的总买入量（多笔买入累计，不减去卖出量）
	// 从资产持仓管理器中获取监控钱包的总买入量
	// 注意：这里需要使用资产持仓管理器中的数据
	// 如果没有记录，则使用当前卖出量作为估计值
	walletHoldings := holdingsManager.GetWalletHoldings(tradeInfo.UserAddress)
	monitoredWalletTotalBuyAmount := uint64(0)
	if tokenHolding, exists := walletHoldings.GetTokenHolding(tradeInfo.MintAddress); exists {
		// 使用记录的总买入量
		monitoredWalletTotalBuyAmount = tokenHolding.TotalBuyAmount
	} else {
		// 如果没有记录，则使用当前卖出量作为估计值
		monitoredWalletTotalBuyAmount = tradeInfo.TokenAmount
		logger.Warnf("无法获取监控钱包 %s 的代币 %s 总买入量，使用当前卖出量作为估计", tradeInfo.UserAddress, tradeInfo.MintAddress)
	}

	if monitoredWalletTotalBuyAmount == 0 {
		logger.Warnf("监控钱包 %s 的代币 %s 总买入量为0，无法计算卖出比例", tradeInfo.UserAddress, tradeInfo.MintAddress)
		return
	}

	// 2. 计算卖出比例
	sellRatio := float64(tradeInfo.TokenAmount) / float64(monitoredWalletTotalBuyAmount)
	logger.Infof("监控钱包 %s 卖出比例: %.2f%%", tradeInfo.UserAddress, sellRatio*100)

	// 3. 根据比例计算我们需要卖出的数量
	amountToSell := uint64(float64(assetInfo.Amount) * sellRatio)
	if amountToSell == 0 {
		logger.Warnf("计算得到的卖出数量为0，使用最小卖出单位1")
		return
	}
	if amountToSell > assetInfo.Amount {
		amountToSell = assetInfo.Amount // 确保不超过我们的持仓量
	}

	logger.Infof("跟单卖出: 代币=%s, 原始持仓=%d, 卖出数量=%d, 卖出比例=%.2f%%",
		assetInfo.MintAddress, assetInfo.Amount, amountToSell, sellRatio*100)

	// 执行跟单卖出
	go queue.SendTokenToSellChannel(0, assetInfo, amountToSell, tradeInfo.CurrentPrice, "pumpfun", walletSettings.Name, "follow_sell")

}

// ProcessAmmTransaction 处理AMM交易事件
func (m *TransactionMonitor) ProcessRaydiumV4Transaction(tradeInfo *types.AmmRaydiumV4TradeInfo, marketLayout *types.MarketState, poolLayout *layouts.LIQUIDITY_STATE_LAYOUT_V4) {
	// 只处理买入交易

	// 获取聪明钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		// 不是我们监控的钱包，忽略
		return
	}
	if !walletSettings.IsFollowSell {
		// 当前钱包不跟卖，忽略
		return
	}
	specailScheme, err := m.WalletMonitor.GetParamSchemeForWallet(copyAddress.String())
	if err != nil {
		logger.Errorf("获取参数模板失败: %v", err)
		return
	}
	if float64(tradeInfo.SolAmount) < (specailScheme.MinFollowTriggerAmount*1e9) || float64(tradeInfo.SolAmount) > (specailScheme.MaxFollowTriggerAmount*1e9) {
		//不在跟卖金额范围内，忽略
		return
	}
	if float64(tradeInfo.SolReserve) < (specailScheme.MinPoolSize*1e9) || float64(tradeInfo.SolReserve) > (specailScheme.MaxPoolSize*1e9) {
		//不在跟卖池子大小范围内，忽略
		return
	}
	logger.Infof("检测到监控钱包Raydium V4 交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 执行跟单交易

	m.executeRaydiumV4CopyTrade(tradeInfo, marketLayout, poolLayout, copyAddress, tradeInfo.TokenAmount, specailScheme)
}
func (m *TransactionMonitor) ProcessRaydiumV4SellTransaction(tradeInfo *types.AmmRaydiumV4TradeInfo, marketLayout *types.MarketState, poolLayout *layouts.LIQUIDITY_STATE_LAYOUT_V4) {
	// 只处理卖出交易

	// 获取交易者钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		// 不是我们监控的钱包，忽略
		return
	}
	if !walletSettings.IsFollowSell {
		// 当前钱包不跟卖，忽略
		return
	}

	logger.Infof("检测到监控钱包 Raydium V4 卖出交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 获取代币信息
	mintAddressStr := tradeInfo.MintAddress.String()
	assetInfo, ok := global.GetPendingHoldAsset(mintAddressStr)
	if !ok {
		logger.Debugf("读取买入资产信息发生异常 ：%s", mintAddressStr)
		return
	}

	// 处理跟单钱包的交易资产卖出
	holdingsManager := asset.GetHoldingsManager()

	// 创建一个与 ProcessSellTransaction 兼容的 TradeEventInfo 结构
	tradeEventInfo := &global.TradeEventInfo{
		UserAddress:  copyAddress.String(),
		MintAddress:  mintAddressStr,
		TokenAmount:  tradeInfo.TokenAmount,
		IsBuy:        false,
		CurrentPrice: tradeInfo.CurrentPrice,
	}

	holdingsManager.RecordTokenTransaction(tradeEventInfo)

	// 计算跟单的同比例卖出
	// 1. 获取监控钱包的总买入量（多笔买入累计，不减去卖出量）
	walletHoldings := holdingsManager.GetWalletHoldings(copyAddress.String())
	monitoredWalletTotalBuyAmount := uint64(0)
	if tokenHolding, exists := walletHoldings.GetTokenHolding(mintAddressStr); exists {
		// 使用记录的总买入量
		monitoredWalletTotalBuyAmount = tokenHolding.TotalBuyAmount
	} else {
		// 如果没有记录，则使用当前卖出量作为估计值
		monitoredWalletTotalBuyAmount = tradeInfo.TokenAmount
		logger.Warnf("无法获取监控钱包 %s 的代币 %s 总买入量，使用当前卖出量作为估计", copyAddress.String(), mintAddressStr)
	}

	if monitoredWalletTotalBuyAmount == 0 {
		logger.Warnf("监控钱包 %s 的代币 %s 总买入量为0，无法计算卖出比例", copyAddress.String(), mintAddressStr)
		return
	}

	// 2. 计算卖出比例
	sellRatio := float64(tradeInfo.TokenAmount) / float64(monitoredWalletTotalBuyAmount)
	logger.Infof("监控钱包 %s 卖出比例: %.2f%%", copyAddress.String(), sellRatio*100)

	// 3. 根据比例计算我们需要卖出的数量
	amountToSell := uint64(float64(assetInfo.Amount) * sellRatio)
	if amountToSell == 0 {
		logger.Warnf("计算得到的卖出数量为0，取消卖出")
		return
	}
	if amountToSell > assetInfo.Amount {
		amountToSell = assetInfo.Amount // 确保不超过我们的持仓量
	}

	logger.Infof("Raydium V4 跟单卖出: 代币=%s, 原始持仓=%d, 卖出数量=%d, 卖出比例=%.2f%%",
		mintAddressStr, assetInfo.Amount, amountToSell, sellRatio*100)

	// 执行跟单卖出
	// 注意：这里需要实现适合 Raydium V4 的卖出逻辑
	// 由于我们直接使用 SendTokenToSellChannel，不需要获取参数模板

	// 调用卖出方法，使用计算出的卖出数量
	go queue.SendTokenToSellChannel(0, assetInfo, amountToSell, tradeInfo.CurrentPrice, "raydiumV4", walletSettings.Name, "follow_sell")
}

// ProcessAmmTransaction 处理AMM交易事件
func (m *TransactionMonitor) ProcessAmmTransaction(tradeInfo *global.AmmTradeEventInfo) {
	// 只处理买入交易

	// 获取交易者钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		logger.Debugf("不监控钱包%s", copyAddress.String())
		// 不是我们监控的钱包，或者钱包未启用，忽略
		return
	}
	specailScheme, err := m.WalletMonitor.GetParamSchemeForWallet(copyAddress.String())
	if err != nil {
		logger.Errorf("获取参数模板失败: %v", err)
		return
	}
	if float64(tradeInfo.SolAmount) < (specailScheme.MinFollowTriggerAmount*1e9) || float64(tradeInfo.SolAmount) > (specailScheme.MaxFollowTriggerAmount*1e9) {
		//不在跟卖金额范围内，忽略
		return
	}
	if float64(tradeInfo.SolReserve) < (specailScheme.MinPoolSize*1e9) || float64(tradeInfo.SolReserve) > (specailScheme.MaxPoolSize*1e9) {
		//不在跟卖池子大小范围内，忽略
		logger.Infof("不在跟卖池子大小范围内，忽略%s %.2f", tradeInfo.MintAddress, float64(tradeInfo.SolReserve)/1e9)
		return
	}
	logger.Infof("检测到监控钱包AMM交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 执行跟单交易
	m.executeAmmCopyTrade(tradeInfo, copyAddress, tradeInfo.TokenAmount, specailScheme)
}

func (m *TransactionMonitor) ProcessAmmSellTransaction(tradeInfo *global.AmmTradeEventInfo) {
	// 只处理卖出交易

	// 获取交易者钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		// 不是我们监控的钱包，忽略
		return
	}
	if !walletSettings.IsFollowSell {
		// 当前钱包不跟卖，忽略
		return
	}

	logger.Infof("检测到监控钱包 AMM 卖出交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 获取代币信息
	mintAddressStr := tradeInfo.MintAddress.String()
	assetInfo, ok := global.GetPendingHoldAsset(mintAddressStr)
	if !ok {
		logger.Debugf("读取买入资产信息发生异常 ：%s", mintAddressStr)
		return
	}

	// 处理跟单钱包的交易资产卖出
	holdingsManager := asset.GetHoldingsManager()

	// 创建一个与 ProcessSellTransaction 兼容的 TradeEventInfo 结构
	tradeEventInfo := &global.TradeEventInfo{
		UserAddress:  copyAddress.String(),
		MintAddress:  mintAddressStr,
		TokenAmount:  tradeInfo.TokenAmount,
		IsBuy:        false,
		CurrentPrice: tradeInfo.CurrentPrice,
	}

	holdingsManager.RecordTokenTransaction(tradeEventInfo)

	// 计算跟单的同比例卖出
	// 1. 获取监控钱包的总买入量（多笔买入累计，不减去卖出量）
	walletHoldings := holdingsManager.GetWalletHoldings(copyAddress.String())
	monitoredWalletTotalBuyAmount := uint64(0)
	if tokenHolding, exists := walletHoldings.GetTokenHolding(mintAddressStr); exists {
		// 使用记录的总买入量
		monitoredWalletTotalBuyAmount = tokenHolding.TotalBuyAmount
	} else {
		// 如果没有记录，则使用当前卖出量作为估计值
		monitoredWalletTotalBuyAmount = tradeInfo.TokenAmount
		logger.Warnf("无法获取监控钱包 %s 的代币 %s 总买入量，使用当前卖出量作为估计", copyAddress.String(), mintAddressStr)
	}

	if monitoredWalletTotalBuyAmount == 0 {
		logger.Warnf("监控钱包 %s 的代币 %s 总买入量为0，无法计算卖出比例", copyAddress.String(), mintAddressStr)
		return
	}

	// 2. 计算卖出比例
	sellRatio := float64(tradeInfo.TokenAmount) / float64(monitoredWalletTotalBuyAmount)
	logger.Infof("监控钱包 %s 卖出比例: %.2f%%", copyAddress.String(), sellRatio*100)

	// 3. 根据比例计算我们需要卖出的数量
	amountToSell := uint64(float64(assetInfo.Amount) * sellRatio)
	if amountToSell == 0 {
		logger.Warnf("计算得到的卖出数量为0，取消卖出")
		return
	}
	if amountToSell > assetInfo.Amount {
		amountToSell = assetInfo.Amount // 确保不超过我们的持仓量
	}

	logger.Infof("AMM 跟单卖出: 代币=%s, 原始持仓=%d, 卖出数量=%d, 卖出比例=%.2f%%",
		mintAddressStr, assetInfo.Amount, amountToSell, sellRatio*100)

	// 执行跟单卖出
	go queue.SendTokenToSellChannel(0, assetInfo, amountToSell, tradeInfo.CurrentPrice, tradeInfo.Router, walletSettings.Name, "follow_sell")
}

// TODO executeCopyTrade 执行跟单交易
func (m *TransactionMonitor) executeCopyTrade(tradeInfo *global.TradeEventInfo, copyAddress solana.PublicKey, specailScheme *types.ParamScheme) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress]; exists && !global.GetConfig().RepeatTradeEnable {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}

	// 发送买入请求
	logger.Infof("执行跟单买入: 代币=%s, 金额=%.6f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress, copyAddress, tradeInfo.CurrentPrice, tradeInfo.TokenAmount) // 假设买入了1000000个代币
	m.WalletMonitor.Mux.RUnlock()

	ctx := context.Background()

	// Convert mint address string to solana.PublicKey
	mintAddr, err := solana.PublicKeyFromBase58(tradeInfo.MintAddress)
	if err != nil {
		logger.Errorf("Failed to parse mint address: %v", err)
		return
	}
	creator := tradeInfo.Creator
	pumpAccounts, err := m.TradeBot.Trader.AccountsCalc.CalculatePumpAccounts(
		ctx,
		creator,
		mintAddr,
		m.BotWallet,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}

	// Prepare transaction parameters
	accounts := core.TradeAccounts{
		Mint:                    mintAddr,
		User:                    m.BotWallet,
		BondingCurve:            pumpAccounts.BondingCurve,
		AssociatedBondingCurve:  pumpAccounts.AssociatedBondingCurve,
		AssociatedTokenAccount:  pumpAccounts.AssociatedTokenAccount,
		CreatorVault:            pumpAccounts.CreatorVaultPda,
		GlobalVolumeAccumulator: pumpAccounts.GlobalVolumeAccumulator,
		UserVolumeAccumulator:   pumpAccounts.UserVolumeAccumulator,
	}

	// Get trade information using the thread-safe getter function
	TradeEventInfo := global.GetTradeInfo(mintAddr.String())

	// If trade information isn't available yet, use default values
	var virtualSolReserves uint64
	if TradeEventInfo == nil {
		logger.Debugf("TradeInfoMap not found for mint address: %s", mintAddr.String())
		virtualSolReserves = 30_000_000_000 // 30 SOL in lamports

	} else {
		virtualSolReserves = TradeEventInfo.VirtualSolReserves
	}
	// Calculate buy amounts
	amounts, err := m.TradeBot.Trader.AmountsCalc.CalculateBuyAmounts(
		ctx,
		pumpAccounts.BondingCurve,
		specailScheme.BuySolAmount,
		virtualSolReserves,
		global.GetConfig().BuySlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}

	// Build transaction parameters
	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:                    accounts.User,
			Mint:                    accounts.Mint,
			BondingCurve:            accounts.BondingCurve,
			AssociatedBondingCurve:  accounts.AssociatedBondingCurve,
			AssociatedTokenAccount:  accounts.AssociatedTokenAccount,
			CreatorVault:            accounts.CreatorVault,
			GlobalVolumeAccumulator: accounts.GlobalVolumeAccumulator,
			UserVolumeAccumulator:   accounts.UserVolumeAccumulator,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MaxCost,
		Blockhash: *m.TradeBot.Blockhash,
		FeeConfig: types.FeeConfig{
			ComputeUnits: m.TradeBot.Config.ComputeUnits,
			Slippage:     specailScheme.BuySlippage,
			PriorityFee:  uint64(specailScheme.BuyPriorityFee * 1e9),
		},
	}

	// Build complete transaction parameters
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		BlockrazorParams: &types.BlockrazorParams{
			Enable:         m.TradeBot.Config.Blockrazor.Enable,
			RpcUrl:         m.TradeBot.Config.Blockrazor.RpcUrl,
			Mode:           m.TradeBot.Config.Blockrazor.Mode,
			HealthEndpoint: m.TradeBot.Config.Blockrazor.HealthEndpoint,
			ApiKey:         m.TradeBot.Config.Blockrazor.ApiKeys[0],
			TipAmount:      uint64(specailScheme.BuyTipFee * 1e9),
		},
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    m.TradeBot.Config.Zeroslot.Enable,
			RpcUrl:    m.TradeBot.Config.Zeroslot.RpcUrl,
			BundleUrl: m.TradeBot.Config.Zeroslot.RpcUrl,
			ApiKeys:   m.TradeBot.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		JitoParams: &types.JitoParams{
			Enable:    m.TradeBot.Config.Jito.Enable,
			BundleUrl: m.TradeBot.Config.Jito.RpcUrl,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    m.TradeBot.Config.Temporal.Enable,
			BundleUrl: m.TradeBot.Config.Temporal.RpcUrl,
			ApiKey:    m.TradeBot.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    m.TradeBot.Config.BloXroute.Enable,
			RpcUrl:    m.TradeBot.Config.BloXroute.RpcUrl,
			ApiKeys:   m.TradeBot.Config.BloXroute.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
	}

	logger.Debugf("Executing buy transaction for token: %s\n", mintAddr.String())

	go m.TradeBot.Trader.Buy(ctx, orderParams)
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress, copyAddress, m.BotWallet, tradeInfo.CurrentPrice)
}

// executeAmmCopyTrade 执行AMM跟单交易
func (m *TransactionMonitor) executeAmmCopyTrade(tradeInfo *global.AmmTradeEventInfo, originalWallet solana.PublicKey, originalAmount uint64, specailScheme *types.ParamScheme) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress.String()]; exists && !global.GetConfig().RepeatTradeEnable {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}

	// 发送买入请求
	logger.Infof("执行AMM跟单买入: 代币=%s, 金额=%f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	// 先记录原始交易者的交易
	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress.String(), originalWallet, tradeInfo.CurrentPrice, originalAmount)
	m.WalletMonitor.Mux.RUnlock()

	pumpAccounts, err := m.TradeBot.Trader.AccountsCalc.CalculateAmmAccounts(

		tradeInfo.AmmPool,
		tradeInfo.MintAddress,
		m.BotWallet,
		tradeInfo.CoinCreator,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}
	baseReserve := tradeInfo.BaseReserve
	quoteReserve := tradeInfo.QuoteReserve
	// Calculate buy amounts
	amounts, err := m.TradeBot.Trader.AmountsCalc.CalculateAmmBuyAmounts(
		specailScheme.BuySolAmount,
		baseReserve,
		quoteReserve,
		specailScheme.BuySlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}

	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			Pool:                      tradeInfo.AmmPool,
			User:                      m.BotWallet,
			Mint:                      pumpAccounts.Mint,
			BaseMint:                  pumpAccounts.Mint,
			QuoteMint:                 pumpAccounts.QuoteMint,
			UserBaseTokenAccount:      pumpAccounts.UserBaseTokenAccount,
			UserQuoteTokenAccount:     pumpAccounts.UserQuoteTokenAccount,
			PoolBaseTokenAccount:      pumpAccounts.PoolBaseTokenAccount,
			PoolQuoteTokenAccount:     pumpAccounts.PoolQuoteTokenAccount,
			CreatorVault:              pumpAccounts.CoinCreatorVaultAta,
			CoinCreatorVaultAuthority: pumpAccounts.CoinCreatorVaultAuthority,
			GlobalVolumeAccumulator:   pumpAccounts.GlobalVolumeAccumulator,
			UserVolumeAccumulator:     pumpAccounts.UserVolumeAccumulator,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MaxCost,
		Blockhash: *m.TradeBot.Blockhash,
		FeeConfig: types.FeeConfig{
			ComputeUnits: m.TradeBot.Config.ComputeUnits,
			Slippage:     specailScheme.BuySlippage,
			PriorityFee:  uint64(specailScheme.BuyPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		BlockrazorParams: &types.BlockrazorParams{
			Enable:         m.TradeBot.Config.Blockrazor.Enable,
			RpcUrl:         m.TradeBot.Config.Blockrazor.RpcUrl,
			HealthEndpoint: m.TradeBot.Config.Blockrazor.HealthEndpoint,
			Mode:           m.TradeBot.Config.Blockrazor.Mode,
			ApiKey:         m.TradeBot.Config.Blockrazor.ApiKeys[0],
			TipAmount:      uint64(specailScheme.BuyTipFee * 1e9),
		},
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    m.TradeBot.Config.Zeroslot.Enable,
			RpcUrl:    m.TradeBot.Config.Zeroslot.RpcUrl,
			ApiKeys:   m.TradeBot.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},

		JitoParams: &types.JitoParams{
			Enable:    m.TradeBot.Config.Jito.Enable,
			BundleUrl: m.TradeBot.Config.Jito.RpcUrl,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    m.TradeBot.Config.Temporal.Enable,
			BundleUrl: m.TradeBot.Config.Temporal.RpcUrl,
			ApiKey:    m.TradeBot.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    m.TradeBot.Config.BloXroute.Enable,
			RpcUrl:    m.TradeBot.Config.BloXroute.RpcUrl,
			ApiKeys:   m.TradeBot.Config.BloXroute.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
	}

	// 调用通用的卖出方法
	go m.TradeBot.Trader.PumpAmmBuy(context.Background(), orderParams)
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress.String(), originalWallet, m.BotWallet, tradeInfo.CurrentPrice)
}

// executeRaydiumV4CopyTrade
func (m *TransactionMonitor) executeRaydiumV4CopyTrade(tradeInfo *types.AmmRaydiumV4TradeInfo, marketLayout *types.MarketState, poolLayout *layouts.LIQUIDITY_STATE_LAYOUT_V4, originalWallet solana.PublicKey, originalAmount uint64, specailScheme *types.ParamScheme) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress.String()]; exists && !global.GetConfig().RepeatTradeEnable {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}

	// 发送买入请求
	logger.Infof("执行RAYDIUM POOL V4 跟单买入: 代币=%s, 金额=%f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	// 先记录原始交易者的交易
	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress.String(), originalWallet, tradeInfo.CurrentPrice, originalAmount)
	m.WalletMonitor.Mux.RUnlock()

	raydiumV4Accounts, err := m.TradeBot.Trader.AccountsCalc.CalculateV4Accounts(
		m.BotWallet,
		tradeInfo,
		marketLayout,
		poolLayout,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}

	// Calculate buy amounts
	amounts, err := m.TradeBot.Trader.AmountsCalc.CalculateRaydiumV4BuyAmounts(
		specailScheme.BuySolAmount,
		tradeInfo.CurrentPrice,
		poolLayout.BaseDecimal,
		specailScheme.BuySlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}

	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:              m.BotWallet,
			Mint:              raydiumV4Accounts.BaseMint,
			BaseMint:          raydiumV4Accounts.BaseMint,
			QuoteMint:         raydiumV4Accounts.QuoteMint,
			RaydiumV4Accounts: raydiumV4Accounts,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MaxCost,
		Blockhash: *m.TradeBot.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    specailScheme.BuySlippage,
			PriorityFee: uint64(specailScheme.BuyPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		BlockrazorParams: &types.BlockrazorParams{
			Enable:         m.TradeBot.Config.Blockrazor.Enable,
			RpcUrl:         m.TradeBot.Config.Blockrazor.RpcUrl,
			HealthEndpoint: m.TradeBot.Config.Blockrazor.HealthEndpoint,
			Mode:           m.TradeBot.Config.Blockrazor.Mode,
			ApiKey:         m.TradeBot.Config.Blockrazor.ApiKeys[0],
			TipAmount:      uint64(specailScheme.BuyTipFee * 1e9),
		},
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    m.TradeBot.Config.Zeroslot.Enable,
			RpcUrl:    m.TradeBot.Config.Zeroslot.RpcUrl,
			ApiKeys:   m.TradeBot.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},

		JitoParams: &types.JitoParams{
			Enable:    m.TradeBot.Config.Jito.Enable,
			BundleUrl: m.TradeBot.Config.Jito.RpcUrl,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    m.TradeBot.Config.Temporal.Enable,
			BundleUrl: m.TradeBot.Config.Temporal.RpcUrl,
			ApiKey:    m.TradeBot.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    m.TradeBot.Config.BloXroute.Enable,
			RpcUrl:    m.TradeBot.Config.BloXroute.RpcUrl,
			ApiKeys:   m.TradeBot.Config.BloXroute.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
	}

	// 调用通用的卖出方法
	go m.TradeBot.Trader.RaydiumV4Buy(context.Background(), orderParams)
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress.String(), originalWallet, m.BotWallet, tradeInfo.CurrentPrice)
}

// ProcessLaunchpadTransaction 处理Launchpad交易事件
func (m *TransactionMonitor) ProcessLaunchpadTransaction(tradeInfo *global.LaunchpadTradeEventInfo) {
	// 只处理买入交易

	// 获取聪明钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {

		logger.Debugf("Launchpad 交易: 聪明钱包=%s, 代币=%s,Sol金额=%d, Token数量=%d, SolReserve=%d, time: %v",
			copyAddress.String(), tradeInfo.MintAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.SolReserve, time.Now().Format("2006-01-02 15:04:05.000000"))
		// 不是我们监控的钱包，忽略
		return
	}
	specailScheme := global.GetConfigByWallet(copyAddress.String())
	if float64(tradeInfo.SolAmount) < (specailScheme.MinFollowTriggerAmount*1e9) || float64(tradeInfo.SolAmount) > (specailScheme.MaxFollowTriggerAmount*1e9) {
		//不在跟卖金额范围内，忽略
		return
	}
	if float64(tradeInfo.SolReserve) < (specailScheme.MinPoolSize*1e9) || float64(tradeInfo.SolReserve) > (specailScheme.MaxPoolSize*1e9) {
		//不在跟卖池子大小范围内，忽略
		return
	}
	logger.Infof("检测到监控钱包Launchpad 交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 执行跟单交易

	m.executeLaunchpadCopyTrade(tradeInfo, copyAddress, tradeInfo.TokenAmount, specailScheme)
}
func (m *TransactionMonitor) ProcessLaunchpadSellTransaction(tradeInfo *global.LaunchpadTradeEventInfo) {

	// 获取交易者钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		// 不是我们监控的钱包，忽略
		return
	}
	if !walletSettings.IsFollowSell {
		// 当前钱包不跟卖，忽略
		return
	}

	logger.Infof("检测到监控钱包 Launchpad 卖出交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 获取代币信息
	mintAddressStr := tradeInfo.MintAddress.String()
	assetInfo, ok := global.GetPendingHoldAsset(mintAddressStr)
	if !ok {
		logger.Debugf("读取买入资产信息发生异常 ：%s", mintAddressStr)
		return
	}

	// 处理跟单钱包的交易资产卖出
	holdingsManager := asset.GetHoldingsManager()

	// 创建一个与 ProcessSellTransaction 兼容的 TradeEventInfo 结构
	tradeEventInfo := &global.TradeEventInfo{
		UserAddress:  copyAddress.String(),
		MintAddress:  mintAddressStr,
		TokenAmount:  tradeInfo.TokenAmount,
		IsBuy:        false,
		CurrentPrice: tradeInfo.CurrentPrice,
	}

	holdingsManager.RecordTokenTransaction(tradeEventInfo)

	// 计算跟单的同比例卖出
	// 1. 获取监控钱包的总买入量（多笔买入累计，不减去卖出量）
	walletHoldings := holdingsManager.GetWalletHoldings(copyAddress.String())
	monitoredWalletTotalBuyAmount := uint64(0)
	if tokenHolding, exists := walletHoldings.GetTokenHolding(mintAddressStr); exists {
		// 使用记录的总买入量
		monitoredWalletTotalBuyAmount = tokenHolding.TotalBuyAmount
	} else {
		// 如果没有记录，则使用当前卖出量作为估计值
		monitoredWalletTotalBuyAmount = tradeInfo.TokenAmount
		logger.Warnf("无法获取监控钱包 %s 的代币 %s 总买入量，使用当前卖出量作为估计", copyAddress.String(), mintAddressStr)
	}

	if monitoredWalletTotalBuyAmount == 0 {
		logger.Warnf("监控钱包 %s 的代币 %s 总买入量为0，无法计算卖出比例", copyAddress.String(), mintAddressStr)
		return
	}

	// 2. 计算卖出比例
	sellRatio := float64(tradeInfo.TokenAmount) / float64(monitoredWalletTotalBuyAmount)
	logger.Infof("监控钱包 %s 卖出比例: %.2f%%", copyAddress.String(), sellRatio*100)

	// 3. 根据比例计算我们需要卖出的数量
	amountToSell := uint64(float64(assetInfo.Amount) * sellRatio)
	if amountToSell == 0 {
		logger.Warnf("计算得到的卖出数量为0，取消卖出")
		return
	}
	if amountToSell > assetInfo.Amount {
		amountToSell = assetInfo.Amount // 确保不超过我们的持仓量
	}

	logger.Infof("Launchpad 跟单卖出: 代币=%s, 原始持仓=%d, 卖出数量=%d, 卖出比例=%.2f%%",
		mintAddressStr, assetInfo.Amount, amountToSell, sellRatio*100)

	// 执行跟单卖出
	go queue.SendTokenToSellChannel(0, assetInfo, amountToSell, tradeInfo.CurrentPrice, "launchpad", walletSettings.Name, "follow_sell")
}

// executeLaunchpadCopyTrade
func (m *TransactionMonitor) executeLaunchpadCopyTrade(tradeInfo *global.LaunchpadTradeEventInfo, originalWallet solana.PublicKey, originalAmount uint64, specailScheme *types.ParamScheme) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress.String()]; exists && !global.GetConfig().RepeatTradeEnable {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}

	// 发送买入请求
	logger.Infof("执行RAYDIUM LAUNCHPAD 跟单买入: 代币=%s, 金额=%f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	// 先记录原始交易者的交易
	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress.String(), originalWallet, tradeInfo.CurrentPrice, originalAmount)
	m.WalletMonitor.Mux.RUnlock()

	launchpadAccounts, err := m.TradeBot.Trader.AccountsCalc.CalculateLaunchpadAccounts(
		m.BotWallet,
		tradeInfo.MintAddress,
		solana.SolMint,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}

	// Calculate buy amounts
	amounts, err := m.TradeBot.Trader.AmountsCalc.CalculateRaydiumLaunchpadBuyAmounts(
		specailScheme.BuySolAmount,
		specailScheme.BuyTipFee,
		tradeInfo.CurrentPrice,
		6,
		specailScheme.BuySlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}

	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:              m.BotWallet,
			Mint:              launchpadAccounts.BaseTokenMint,
			BaseMint:          launchpadAccounts.BaseTokenMint,
			QuoteMint:         launchpadAccounts.QuoteTokenMint,
			LaunchpadAccounts: launchpadAccounts,
		},
		Amount:       amounts.SolAmount,
		MaxSolAmount: amounts.MaxSolAmount,
		Cost:         amounts.MaxCost,
		Blockhash:    *m.TradeBot.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    specailScheme.BuySlippage,
			PriorityFee: uint64(specailScheme.BuyPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		BlockrazorParams: &types.BlockrazorParams{
			Enable:         m.TradeBot.Config.Blockrazor.Enable,
			RpcUrl:         m.TradeBot.Config.Blockrazor.RpcUrl,
			HealthEndpoint: m.TradeBot.Config.Blockrazor.HealthEndpoint,
			Mode:           m.TradeBot.Config.Blockrazor.Mode,
			ApiKey:         m.TradeBot.Config.Blockrazor.ApiKeys[0],
			TipAmount:      uint64(specailScheme.BuyTipFee * 1e9),
		},
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    m.TradeBot.Config.Zeroslot.Enable,
			RpcUrl:    m.TradeBot.Config.Zeroslot.RpcUrl,
			ApiKeys:   m.TradeBot.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},

		JitoParams: &types.JitoParams{
			Enable:    m.TradeBot.Config.Jito.Enable,
			BundleUrl: m.TradeBot.Config.Jito.RpcUrl,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    m.TradeBot.Config.Temporal.Enable,
			BundleUrl: m.TradeBot.Config.Temporal.RpcUrl,
			ApiKey:    m.TradeBot.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    m.TradeBot.Config.BloXroute.Enable,
			RpcUrl:    m.TradeBot.Config.BloXroute.RpcUrl,
			ApiKeys:   m.TradeBot.Config.BloXroute.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
	}

	// 调用通用的卖出方法
	go m.TradeBot.Trader.RaydiumLaunchpadBuy(context.Background(), orderParams)
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress.String(), originalWallet, m.BotWallet, tradeInfo.CurrentPrice)
}

// ProcessCpmmTransaction 处理CPMM交易事件
func (m *TransactionMonitor) ProcessCpmmTransaction(tradeInfo *global.CPMMTradeEventInfo) {

	// 获取聪明钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {

		logger.Debugf("CPMM 交易: 聪明钱包=%s, 代币=%s,Sol金额=%d, Token数量=%d, SolReserve=%d, time: %v",
			copyAddress.String(), tradeInfo.MintAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.SolReserve, time.Now().Format("2006-01-02 15:04:05.000000"))
		// 不是我们监控的钱包，忽略
		return
	}
	specailScheme, err := m.WalletMonitor.GetParamSchemeForWallet(copyAddress.String())
	if err != nil {
		logger.Errorf("获取参数模板失败: %v", err)
		return
	}
	if float64(tradeInfo.SolAmount) < (specailScheme.MinFollowTriggerAmount*1e9) || float64(tradeInfo.SolAmount) > (specailScheme.MaxFollowTriggerAmount*1e9) {
		//不在跟卖金额范围内，忽略
		return
	}
	if float64(tradeInfo.SolReserve) < (specailScheme.MinPoolSize*1e9) || float64(tradeInfo.SolReserve) > (specailScheme.MaxPoolSize*1e9) {
		//不在跟卖池子大小范围内，忽略
		return
	}
	logger.Infof("检测到监控钱包CPMM 交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 执行跟单交易

	m.executeCpmmCopyTrade(tradeInfo, copyAddress, tradeInfo.TokenAmount, specailScheme)
}
func (m *TransactionMonitor) ProcessCpmmSellTransaction(tradeInfo *global.CPMMTradeEventInfo) {
	// 只处理卖出交易

	// 获取交易者钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil || !walletSettings.Enabled {
		// 不是我们监控的钱包，忽略
		return
	}
	if !walletSettings.IsFollowSell {
		// 当前钱包不跟卖，忽略
		return
	}

	logger.Infof("检测到监控钱包 CPMM 卖出交易: 钱包=%s (%s), 代币=%s, 数量=%d, time: %v",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 获取代币信息
	mintAddressStr := tradeInfo.MintAddress.String()
	assetInfo, ok := global.GetPendingHoldAsset(mintAddressStr)
	if !ok {
		logger.Debugf("读取买入资产信息发生异常 ：%s", mintAddressStr)
		return
	}

	// 处理跟单钱包的交易资产卖出
	holdingsManager := asset.GetHoldingsManager()

	// 创建一个与 ProcessSellTransaction 兼容的 TradeEventInfo 结构
	tradeEventInfo := &global.TradeEventInfo{
		UserAddress:  copyAddress.String(),
		MintAddress:  mintAddressStr,
		TokenAmount:  tradeInfo.TokenAmount,
		IsBuy:        false,
		CurrentPrice: tradeInfo.CurrentPrice,
	}

	holdingsManager.RecordTokenTransaction(tradeEventInfo)

	// 计算跟单的同比例卖出
	// 1. 获取监控钱包的总买入量（多笔买入累计，不减去卖出量）
	walletHoldings := holdingsManager.GetWalletHoldings(copyAddress.String())
	monitoredWalletTotalBuyAmount := uint64(0)
	if tokenHolding, exists := walletHoldings.GetTokenHolding(mintAddressStr); exists {
		// 使用记录的总买入量
		monitoredWalletTotalBuyAmount = tokenHolding.TotalBuyAmount
	} else {
		// 如果没有记录，则使用当前卖出量作为估计值
		monitoredWalletTotalBuyAmount = tradeInfo.TokenAmount
		logger.Warnf("无法获取监控钱包 %s 的代币 %s 总买入量，使用当前卖出量作为估计", copyAddress.String(), mintAddressStr)
	}

	if monitoredWalletTotalBuyAmount == 0 {
		logger.Warnf("监控钱包 %s 的代币 %s 总买入量为0，无法计算卖出比例", copyAddress.String(), mintAddressStr)
		return
	}

	// 2. 计算卖出比例
	sellRatio := float64(tradeInfo.TokenAmount) / float64(monitoredWalletTotalBuyAmount)
	logger.Infof("监控钱包 %s 卖出比例: %.2f%%", copyAddress.String(), sellRatio*100)

	// 3. 根据比例计算我们需要卖出的数量
	amountToSell := uint64(float64(assetInfo.Amount) * sellRatio)
	if amountToSell == 0 {
		logger.Warnf("计算得到的卖出数量为0，取消卖出")
		return
	}
	if amountToSell > assetInfo.Amount {
		amountToSell = assetInfo.Amount // 确保不超过我们的持仓量
	}

	logger.Infof("CPMM 跟单卖出: 代币=%s, 原始持仓=%d, 卖出数量=%d, 卖出比例=%.2f%%",
		mintAddressStr, assetInfo.Amount, amountToSell, sellRatio*100)

	// 执行跟单卖出
	go queue.SendTokenToSellChannel(0, assetInfo, amountToSell, tradeInfo.CurrentPrice, "cpmm", walletSettings.Name, "follow_sell")
}

// executeCpmmCopyTrade
func (m *TransactionMonitor) executeCpmmCopyTrade(tradeInfo *global.CPMMTradeEventInfo, originalWallet solana.PublicKey, originalAmount uint64, specailScheme *types.ParamScheme) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress.String()]; exists && !global.GetConfig().RepeatTradeEnable {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}

	// 发送买入请求
	logger.Infof("执行RAYDIUM CPMM 跟单买入: 代币=%s, 金额=%f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	// 先记录原始交易者的交易
	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress.String(), originalWallet, tradeInfo.CurrentPrice, originalAmount)
	m.WalletMonitor.Mux.RUnlock()
	cpmmAccounts, err := m.TradeBot.Trader.AccountsCalc.CalculateRaydiumCPMMAccounts(
		m.BotWallet,
		tradeInfo.AmmConfig,
		tradeInfo.PoolState,
		tradeInfo.MintAddress,
		solana.SolMint,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}

	// Calculate buy amounts
	amounts, err := m.TradeBot.Trader.AmountsCalc.CalculateRaydiumCPMMBuyAmounts(
		specailScheme.BuySolAmount,
		specailScheme.BuyTipFee,
		tradeInfo.CurrentPrice,
		6,
		specailScheme.BuySlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}

	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:                m.BotWallet,
			Mint:                cpmmAccounts.OutputTokenMint,
			BaseMint:            cpmmAccounts.OutputTokenMint,
			QuoteMint:           cpmmAccounts.InputTokenMint,
			RaydiumCPMMAccounts: cpmmAccounts,
		},
		Amount:       amounts.SolAmount,
		MaxSolAmount: amounts.MaxSolAmount,
		Cost:         amounts.MaxCost,
		Blockhash:    *m.TradeBot.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    specailScheme.BuySlippage,
			PriorityFee: uint64(specailScheme.BuyPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		BlockrazorParams: &types.BlockrazorParams{
			Enable:         m.TradeBot.Config.Blockrazor.Enable,
			RpcUrl:         m.TradeBot.Config.Blockrazor.RpcUrl,
			HealthEndpoint: m.TradeBot.Config.Blockrazor.HealthEndpoint,
			Mode:           m.TradeBot.Config.Blockrazor.Mode,
			ApiKey:         m.TradeBot.Config.Blockrazor.ApiKeys[0],
			TipAmount:      uint64(specailScheme.BuyTipFee * 1e9),
		},
		ZeroslotParams: &types.ZeroslotParams{
			Enable:    m.TradeBot.Config.Zeroslot.Enable,
			RpcUrl:    m.TradeBot.Config.Zeroslot.RpcUrl,
			ApiKeys:   m.TradeBot.Config.Zeroslot.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},

		JitoParams: &types.JitoParams{
			Enable:    m.TradeBot.Config.Jito.Enable,
			BundleUrl: m.TradeBot.Config.Jito.RpcUrl,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    m.TradeBot.Config.Temporal.Enable,
			BundleUrl: m.TradeBot.Config.Temporal.RpcUrl,
			ApiKey:    m.TradeBot.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    m.TradeBot.Config.BloXroute.Enable,
			RpcUrl:    m.TradeBot.Config.BloXroute.RpcUrl,
			ApiKeys:   m.TradeBot.Config.BloXroute.ApiKeys,
			TipAmount: uint64(specailScheme.BuyTipFee * 1e9),
		},
	}

	// 调用通用的卖出方法
	go m.TradeBot.Trader.RaydiumCPMMBuy(context.Background(), orderParams)
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress.String(), originalWallet, m.BotWallet, tradeInfo.CurrentPrice)
}
