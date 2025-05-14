package wallet

import (
	"context"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"

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
	// 只处理买入交易
	if !tradeInfo.IsBuy {
		return
	}

	// 获取交易者钱包地址
	copyAddress, err := solana.PublicKeyFromBase58(tradeInfo.UserAddress)
	if err != nil {
		logger.Errorf("解析用户地址失败: %v", err)
		return
	}

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil {
		// 不是我们监控的钱包，忽略
		return
	}

	logger.Infof("检测到监控钱包交易: 钱包=%s (%s), 代币=%s, 数量=%d",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount)

	// 执行跟单交易
	m.executeCopyTrade(tradeInfo, copyAddress)
}

// ProcessAmmTransaction 处理AMM交易事件
func (m *TransactionMonitor) ProcessAmmTransaction(tradeInfo *global.AmmTradeEventInfo) {
	// 只处理买入交易
	if !tradeInfo.IsBuy {
		return
	}

	// 获取交易者钱包地址
	copyAddress := tradeInfo.UserAddress

	// 检查是否是我们监控的钱包
	walletSettings := m.WalletMonitor.GetWallet(copyAddress)
	if walletSettings == nil {
		// 不是我们监控的钱包，忽略
		return
	}

	logger.Infof("检测到监控钱包AMM交易: 钱包=%s (%s), 代币=%s, 数量=%d",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount)

	// 执行跟单交易
	m.executeAmmCopyTrade(tradeInfo, copyAddress, tradeInfo.TokenAmount)
}

// TODO executeCopyTrade 执行跟单交易
func (m *TransactionMonitor) executeCopyTrade(tradeInfo *global.TradeEventInfo, copyAddress solana.PublicKey) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress]; exists {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}
	m.WalletMonitor.Mux.RUnlock()
	// 发送买入请求
	logger.Infof("执行跟单买入: 代币=%s, 金额=%f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress, copyAddress, tradeInfo.CurrentPrice, tradeInfo.TokenAmount) // 假设买入了1000000个代币
	ctx := context.Background()

	// Convert mint address string to solana.PublicKey
	mintAddr, err := solana.PublicKeyFromBase58(tradeInfo.MintAddress)
	if err != nil {
		logger.Errorf("Failed to parse mint address: %v", err)
		return
	}

	pumpAccounts, err := m.TradeBot.Trader.AccountsCalc.CalculatePumpAccounts(
		ctx,
		mintAddr,
		m.BotWallet,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}

	// Prepare transaction parameters
	accounts := core.TradeAccounts{
		Mint:                   mintAddr,
		User:                   m.BotWallet,
		BondingCurve:           pumpAccounts.BondingCurve,
		AssociatedBondingCurve: pumpAccounts.AssociatedBondingCurve,
		AssociatedTokenAccount: pumpAccounts.AssociatedTokenAccount,
		CreatorVault:           pumpAccounts.CreatorVaultPda,
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
		m.BuySolAmount,
		virtualSolReserves,
		m.TradeBot.Config.BuySlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}

	// Build transaction parameters
	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:                   accounts.User,
			Mint:                   accounts.Mint,
			BondingCurve:           accounts.BondingCurve,
			AssociatedBondingCurve: accounts.AssociatedBondingCurve,
			AssociatedTokenAccount: accounts.AssociatedTokenAccount,
			CreatorVault:           accounts.CreatorVault,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MaxCost,
		Blockhash: *m.TradeBot.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    m.TradeBot.Config.BuySlippage,
			PriorityFee: uint64(m.TradeBot.Config.BuyPriorityFee * 1e9),
		},
	}

	// Build complete transaction parameters
	orderParams := types.OrderParams{
		TradeParams: tradeParams,
		JitoParams: &types.JitoParams{
			Enable:    m.TradeBot.Config.Jito.Enable,
			BundleUrl: m.TradeBot.Config.Jito.RpcUrl,
			TipAmount: uint64(m.TradeBot.Config.Jito.BuyTip * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    m.TradeBot.Config.Temporal.Enable,
			BundleUrl: m.TradeBot.Config.Temporal.RpcUrl,
			ApiKey:    m.TradeBot.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(m.TradeBot.Config.Temporal.BuyTip * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    m.TradeBot.Config.BloXroute.Enable,
			RpcUrl:    m.TradeBot.Config.BloXroute.RpcUrl,
			ApiKeys:   m.TradeBot.Config.BloXroute.ApiKeys,
			TipAmount: uint64(m.TradeBot.Config.BloXroute.BuyTip * 1e9),
		},
	}

	logger.Debugf("Executing buy transaction for token: %s\n", mintAddr.String())

	go m.TradeBot.Trader.Buy(ctx, orderParams)
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress, copyAddress, m.BotWallet, tradeInfo.CurrentPrice)
}

// executeAmmCopyTrade 执行AMM跟单交易
func (m *TransactionMonitor) executeAmmCopyTrade(tradeInfo *global.AmmTradeEventInfo, originalWallet solana.PublicKey, originalAmount uint64) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress.String()]; exists {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}
	m.WalletMonitor.Mux.RUnlock()

	// 发送买入请求
	logger.Infof("执行AMM跟单买入: 代币=%s, 金额=%f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	// 先记录原始交易者的交易
	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress.String(), originalWallet, tradeInfo.CurrentPrice, originalAmount)
	//
	pumpAccounts, err := m.TradeBot.Trader.AccountsCalc.CalculateAmmAccounts(
		context.Background(),
		tradeInfo.AmmPool,
		tradeInfo.MintAddress,
		m.BotWallet,
		creator,
	)
	if err != nil {
		logger.Errorf("Failed to calculate accounts: %v", err)
		return
	}
	baseReserve := tradeInfo.BaseReserve
	quoteReserve := tradeInfo.QuoteReserve
	// Calculate buy amounts
	amounts, err := m.TradeBot.Trader.AmountsCalc.CalculateAmmBuyAmounts(
		m.BuySolAmount,
		baseReserve,
		quoteReserve,
		m.TradeBot.Config.BuySlippage,
	)

	if err != nil {
		logger.Errorf("Failed to calculate buy amounts: %v", err)
		return
	}

	tradeParams := types.TradeParams{
		Accounts: types.TradeAccounts{
			User:                      m.BotWallet,
			Mint:                      pumpAccounts.Mint,
			BaseMint:                  pumpAccounts.Mint,
			QuoteMint:                 pumpAccounts.QuoteMint,
			UserBaseTokenAccount:      pumpAccounts.UserBaseTokenAccount,
			PoolBaseTokenAccount:      pumpAccounts.PoolBaseTokenAccount,
			PoolQuoteTokenAccount:     pumpAccounts.PoolQuoteTokenAccount,
			CreatorVault:              pumpAccounts.CoinCreatorVaultAta,
			CoinCreatorVaultAuthority: pumpAccounts.CoinCreatorVaultAuthority,
		},
		Amount:    amounts.TokenAmount,
		Cost:      amounts.MinOut,
		Blockhash: *m.TradeBot.Blockhash,
		FeeConfig: types.FeeConfig{
			Slippage:    m.TradeBot.Config.BuySlippage,
			PriorityFee: uint64(m.TradeBot.Config.BuyPriorityFee * 1e9),
		},
	}

	// 构建完整的订单参数
	orderParams := types.OrderParams{
		TradeParams: tradeParams,

		JitoParams: &types.JitoParams{
			Enable:    m.TradeBot.Config.Jito.Enable,
			BundleUrl: m.TradeBot.Config.Jito.RpcUrl,
			TipAmount: uint64(m.TradeBot.Config.Jito.BuyTip * 1e9),
		},
		TemporalParams: &types.TemporalParams{
			Enable:    m.TradeBot.Config.Temporal.Enable,
			BundleUrl: m.TradeBot.Config.Temporal.RpcUrl,
			ApiKey:    m.TradeBot.Config.Temporal.ApiKeys[0],
			TipAmount: uint64(m.TradeBot.Config.Temporal.BuyTip * 1e9),
		},
		BloXrouteParams: &types.BloXrouteParams{
			Enable:    m.TradeBot.Config.BloXroute.Enable,
			RpcUrl:    m.TradeBot.Config.BloXroute.RpcUrl,
			ApiKeys:   m.TradeBot.Config.BloXroute.ApiKeys,
			TipAmount: uint64(m.TradeBot.Config.BloXroute.BuyTip * 1e9),
		},
	}

	// 调用通用的卖出方法
	go m.TradeBot.Trader.PumpAmmBuy(context.Background(), orderParams)
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress.String(), originalWallet, m.BotWallet, tradeInfo.CurrentPrice)
}
