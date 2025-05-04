package wallet

import (
	"time"

	"copytrade/global"
	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
)

// 交易监控器
type TransactionMonitor struct {
	WalletMonitor *WalletMonitor   // 钱包监控器
	BotWallet     solana.PublicKey // 机器人钱包地址
	BuySolAmount  float64          // 购买金额（SOL）
}

// 创建新的交易监控器
func NewTransactionMonitor(walletMonitor *WalletMonitor, botWallet solana.PublicKey, buySolAmount float64) *TransactionMonitor {
	return &TransactionMonitor{
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
	m.executeCopyTrade(tradeInfo)
}

// ProcessAmmTransaction 处理AMM交易事件
func (m *TransactionMonitor) ProcessAmmTransaction(tradeInfo *global.AmmTradeEventInfo) {
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

	logger.Infof("检测到监控钱包AMM交易: 钱包=%s (%s), 代币=%s, 数量=%d",
		copyAddress.String(), walletSettings.Name, tradeInfo.MintAddress, tradeInfo.TokenAmount)

	// 执行跟单交易
	m.executeAmmCopyTrade(tradeInfo, copyAddress, tradeInfo.TokenAmount)
}

// TODO executeCopyTrade 执行跟单交易
func (m *TransactionMonitor) executeCopyTrade(tradeInfo *global.TradeEventInfo) {
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

	// 创建交易信息 - 使用现有的 MyBuyMap 结构
	global.SetPendingHoldAsset(tradeInfo.MintAddress, &global.MyBuyMap{
		EntryPrice: tradeInfo.CurrentPrice,
		Amount:     1000000,           // 假设买入数量
		TimeStamp:  time.Now().Unix(), // 转换为Unix时间戳
	})

	// 假设买入成功
	botTokenAmount := uint64(1000000)                                                                // 假设买入了1000000个代币
	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress, m.BotWallet, tradeInfo.CurrentPrice, 1000000) // 假设买入了1000000个代币
	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress, m.BotWallet, tradeInfo.CurrentPrice, botTokenAmount)
}

// executeAmmCopyTrade 执行AMM跟单交易
func (m *TransactionMonitor) executeAmmCopyTrade(tradeInfo *global.AmmTradeEventInfo, originalWallet solana.PublicKey, originalAmount uint64) {
	// 检查是否已经交易过该代币
	m.WalletMonitor.Mux.RLock()
	if _, exists := m.WalletMonitor.TradedTokens[tradeInfo.MintAddress]; exists {
		logger.Infof("跳过重复交易: %s", tradeInfo.MintAddress)
		m.WalletMonitor.Mux.RUnlock()
		return
	}
	m.WalletMonitor.Mux.RUnlock()

	// 发送买入请求
	logger.Infof("执行AMM跟单买入: 代币=%s, 金额=%f SOL", tradeInfo.MintAddress, m.BuySolAmount)

	// 创建交易信息 - 使用现有的 AmmTradeEventInfo 结构
	global.SetAmmTradeInfo(tradeInfo.MintAddress, tradeInfo)

	// 在实际实现中，这里会调用买入函数
	// 假设买入成功
	botTokenAmount := uint64(1000000) // 假设买入了1000000个代币

	// 先记录原始交易者的交易
	m.WalletMonitor.RecordTrade(tradeInfo.MintAddress, originalWallet, tradeInfo.CurrentPrice, originalAmount)

	// 然后记录机器人的跟单交易
	m.WalletMonitor.RecordBotTrade(tradeInfo.MintAddress, m.BotWallet, tradeInfo.CurrentPrice, botTokenAmount)
}
