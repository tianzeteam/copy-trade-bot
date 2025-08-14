package asset

import (
	"copytrade/global"
	"copytrade/logger"
	"sync"
	"time"
)

// TokenHolding 代表一个代币持仓
type TokenHolding struct {
	MintAddress    string    // 代币的Mint地址
	Amount         uint64    // 当前持有数量
	TotalBuyAmount uint64    // 总买入量（多笔买入累计，不减去卖出量）
	BuyPrice       float64   // 平均买入价格（SOL）
	CurrentPrice   float64   // 当前价格（SOL）
	LastUpdated    time.Time // 最后更新时间
}

// WalletHoldings 代表一个钱包的所有代币持仓
type WalletHoldings struct {
	WalletAddress string                  // 钱包地址
	Holdings      map[string]TokenHolding // 代币持仓映射 (MintAddress -> TokenHolding)
	mutex         sync.RWMutex            // 读写锁
}

// HoldingsManager 管理所有跟单钱包的资产持仓
type HoldingsManager struct {
	walletHoldings map[string]*WalletHoldings // 钱包持仓映射 (WalletAddress -> WalletHoldings)
	mintToWallets  map[string][]string        // 代币持有钱包映射 (MintAddress -> []WalletAddress)
	mutex          sync.RWMutex               // 读写锁
}

// 全局单例实例
var (
	holdingsManager *HoldingsManager
	holdingsOnce    sync.Once
)

// NewHoldingsManager 创建资产持仓管理器
func NewHoldingsManager() *HoldingsManager {
	holdingsOnce.Do(func() {
		holdingsManager = &HoldingsManager{
			walletHoldings: make(map[string]*WalletHoldings),
			mintToWallets:  make(map[string][]string),
		}
	})
	return holdingsManager
}

// GetHoldingsManager 获取全局资产持仓管理器实例
func GetHoldingsManager() *HoldingsManager {
	if holdingsManager == nil {
		return NewHoldingsManager()
	}
	return holdingsManager
}

// GetWalletHoldings 获取指定钱包的持仓信息，如果不存在则创建
func (hm *HoldingsManager) GetWalletHoldings(walletAddress string) *WalletHoldings {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	holdings, ok := hm.walletHoldings[walletAddress]

	if !ok {
		// 创建新的钱包持仓
		holdings = &WalletHoldings{
			WalletAddress: walletAddress,
			Holdings:      make(map[string]TokenHolding),
		}

		// 存储到映射中
		hm.walletHoldings[walletAddress] = holdings
	}

	return holdings
}

// RecordTokenTransaction 记录代币交易
func (hm *HoldingsManager) RecordTokenTransaction(tradeInfo *global.TradeEventInfo) {
	// 获取钱包持仓
	walletHoldings := hm.GetWalletHoldings(tradeInfo.UserAddress)

	// 记录交易
	walletHoldings.RecordTransaction(tradeInfo)

	// 更新代币持有钱包映射
	hm.updateMintToWallets(tradeInfo.IsBuy, tradeInfo.MintAddress, tradeInfo.UserAddress)
}

// updateMintToWallets 更新代币持有钱包映射
func (hm *HoldingsManager) updateMintToWallets(isBuy bool, mintAddress string, walletAddress string) {
	if !isBuy {
		return
	}
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	wallets, ok := hm.mintToWallets[mintAddress]
	if !ok {
		hm.mintToWallets[mintAddress] = []string{walletAddress}
		return
	}

	// 检查钱包是否已经存在
	for _, wallet := range wallets {
		if wallet == walletAddress {
			return
		}
	}

	// 添加钱包到列表
	hm.mintToWallets[mintAddress] = append(wallets, walletAddress)
}

// GetWalletsByMint 获取持有特定代币的钱包列表
func (hm *HoldingsManager) GetWalletsByMint(mintAddress string) []string {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	wallets, ok := hm.mintToWallets[mintAddress]
	if !ok {
		return []string{}
	}

	// 返回副本，避免外部修改
	result := make([]string, len(wallets))
	copy(result, wallets)
	return result
}

// RecordTransaction 记录代币交易到钱包持仓
func (wh *WalletHoldings) RecordTransaction(tradeInfo *global.TradeEventInfo) {
	wh.mutex.Lock()
	defer wh.mutex.Unlock()

	mintAddress := tradeInfo.MintAddress
	currentPrice := tradeInfo.CurrentPrice

	if tradeInfo.IsBuy {
		// 买入交易
		holding, ok := wh.Holdings[mintAddress]
		if ok {
			// 已有持仓，更新平均买入价和数量
			totalAmount := holding.Amount + tradeInfo.TokenAmount
			totalValue := (float64(holding.Amount) * holding.BuyPrice) + (float64(tradeInfo.TokenAmount) * currentPrice)
			averagePrice := totalValue / float64(totalAmount)

			// 更新总买入量
			totalBuyAmount := holding.TotalBuyAmount + tradeInfo.TokenAmount

			holding.Amount = totalAmount
			holding.TotalBuyAmount = totalBuyAmount
			holding.BuyPrice = averagePrice
			holding.CurrentPrice = currentPrice
			holding.LastUpdated = time.Now()
			wh.Holdings[mintAddress] = holding

			logger.Infof("更新钱包 %s 的代币持仓: 代币=%s, 当前数量=%d, 总买入量=%d, 平均买入价=%.8f SOL",
				wh.WalletAddress, mintAddress, totalAmount, totalBuyAmount, averagePrice)
		} else {
			// 新建持仓
			wh.Holdings[mintAddress] = TokenHolding{
				MintAddress:    mintAddress,
				Amount:         tradeInfo.TokenAmount,
				TotalBuyAmount: tradeInfo.TokenAmount, // 初始总买入量等于首次买入数量
				BuyPrice:       currentPrice,
				CurrentPrice:   currentPrice,
				LastUpdated:    time.Now(),
			}

			logger.Infof("新增钱包 %s 的代币持仓: 代币=%s, 数量=%d, 买入价=%.8f SOL",
				wh.WalletAddress, mintAddress, tradeInfo.TokenAmount, currentPrice)
		}
	} else {
		// 卖出交易
		holding, ok := wh.Holdings[mintAddress]
		if !ok {
			logger.Warnf("钱包 %s 卖出了未记录持仓的代币: %s", wh.WalletAddress, mintAddress)
			return
		}

		// 更新持仓数量
		if holding.Amount <= tradeInfo.TokenAmount {
			// 全部卖出
			delete(wh.Holdings, mintAddress)
			logger.Infof("钱包 %s 已清空代币持仓: 代币=%s", wh.WalletAddress, mintAddress)
		} else {
			// 部分卖出
			holding.Amount -= tradeInfo.TokenAmount
			holding.CurrentPrice = currentPrice
			holding.LastUpdated = time.Now()
			wh.Holdings[mintAddress] = holding

			logger.Infof("减少钱包 %s 的代币持仓: 代币=%s, 剩余数量=%d, 当前价格=%.8f SOL",
				wh.WalletAddress, mintAddress, holding.Amount, currentPrice)
		}
	}
}

// GetTokenHolding 获取指定代币的持仓信息
func (wh *WalletHoldings) GetTokenHolding(mintAddress string) (TokenHolding, bool) {
	wh.mutex.RLock()
	defer wh.mutex.RUnlock()

	holding, exists := wh.Holdings[mintAddress]
	return holding, exists
}

// GetAllHoldings 获取钱包的所有代币持仓
func (wh *WalletHoldings) GetAllHoldings() map[string]TokenHolding {
	wh.mutex.RLock()
	defer wh.mutex.RUnlock()

	// 创建副本以避免并发问题
	holdings := make(map[string]TokenHolding, len(wh.Holdings))
	for k, v := range wh.Holdings {
		holdings[k] = v
	}

	return holdings
}

// UpdateTokenPrice 更新代币价格
func (wh *WalletHoldings) UpdateTokenPrice(mintAddress string, currentPrice float64) bool {
	wh.mutex.Lock()
	defer wh.mutex.Unlock()

	holding, ok := wh.Holdings[mintAddress]
	if !ok {
		return false
	}

	holding.CurrentPrice = currentPrice
	holding.LastUpdated = time.Now()
	wh.Holdings[mintAddress] = holding

	return true
}

// CalculateHoldingValue 计算持仓价值（以SOL为单位）
func (wh *WalletHoldings) CalculateHoldingValue() float64 {
	wh.mutex.RLock()
	defer wh.mutex.RUnlock()

	var totalValue float64
	for _, holding := range wh.Holdings {
		tokenValue := float64(holding.Amount) * holding.CurrentPrice
		totalValue += tokenValue
	}

	return totalValue
}

// CalculateTokenProfit 计算指定代币的盈亏（以SOL为单位）
func (wh *WalletHoldings) CalculateTokenProfit(mintAddress string) (float64, bool) {
	wh.mutex.RLock()
	defer wh.mutex.RUnlock()

	holding, ok := wh.Holdings[mintAddress]
	if !ok {
		return 0, false
	}

	// 计算盈亏
	profit := (holding.CurrentPrice - holding.BuyPrice) * float64(holding.Amount)
	return profit, true
}
