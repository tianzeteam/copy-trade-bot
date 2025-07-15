package queue

import (
	"time"

	"copytrade/global"
	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
)

// TokenInfo 代币信息结构体
// 用于在通道中传递代币信息
type TokenInfo struct {
	Pool                   solana.PublicKey // 代币池地址
	PoolBaseTokenAccount   solana.PublicKey // 代币池基础代币账户
	PoolQuoteTokenAccount  solana.PublicKey // 代币池报价代币账户
	PoolBaseTokenReserves  uint64           // 代币池基础代币储备
	PoolQuoteTokenReserves uint64           // 代币池报价代币储备
	Mint                   solana.PublicKey // 代币Mint地址
	Creator                solana.PublicKey // 代币创建者
	Owner                  solana.PublicKey // 代币所有者
	Amount                 uint64           // 代币数量
	User                   solana.PublicKey // 用户地址
	UpdateTime             time.Time        // 更新时间
	EntryPrice             float64
	CurrentPrice           float64
	Platform               string
	StrategyName           string // 策略名称
}

var (
	// 全局代币信息通道
	WaitingSellChan = make(chan TokenInfo, 1) // 导出通道供其他包使用
	TokenSellChan   = make(chan TokenInfo, 1) // 导出通道供其他包使用
)

// sendTokenToSellChannel 创建 TokenInfo 并将其发送到 WaitingSellChan
// 它还记录了尝试发送和结果。
func SendTokenToSellChannel(delayTime int, assetInfo *global.MyBuyMap, currentPrice float64, platform, strategyName string, reason string) {
	tokenInfo := TokenInfo{
		Mint:         solana.MustPublicKeyFromBase58(assetInfo.MintAddress),
		Owner:        solana.MustPublicKeyFromBase58(assetInfo.UserAddress),
		Amount:       assetInfo.Amount,
		Creator:      assetInfo.Creator,
		User:         solana.MustPublicKeyFromBase58(assetInfo.UserAddress),
		UpdateTime:   time.Now(),
		EntryPrice:   assetInfo.EntryPrice,
		CurrentPrice: currentPrice,
		Platform:     platform,
		StrategyName: strategyName,
	}

	logger.Debugf("[%s] 准备发送代币到卖出通道: Mint=%s, 余额=%d, 入场价=%.8f, 策略=%s,触发原因=%s",
		assetInfo.UserAddress, tokenInfo.Mint, tokenInfo.Amount, assetInfo.EntryPrice, strategyName, reason)
	time.Sleep(time.Duration(delayTime) * time.Millisecond)

	// 尝试发送到通道，使用select避免阻塞
	select {
	case WaitingSellChan <- tokenInfo:
		logger.Infof("[%s] 代币信息已成功发送到卖出通道: Mint=%s, 策略=%s", assetInfo.UserAddress, tokenInfo.Mint, strategyName)
	default:
		logger.Warnf("[%s] 卖出通道已满，无法发送代币信息: Mint=%s, 策略=%s", assetInfo.UserAddress, tokenInfo.Mint, strategyName)
	}
}
