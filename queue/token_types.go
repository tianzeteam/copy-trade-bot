package queue

import (
	"time"

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
	Owner                  solana.PublicKey // 代币所有者
	Amount                 uint64           // 代币数量
	User                   solana.PublicKey // 用户地址
	UpdateTime             time.Time        // 更新时间
	EntryPrice             float64
	CurrentPrice           float64
	Platform               string
}

var (
	// 全局代币信息通道
	WaitingSellChan = make(chan TokenInfo, 1) // 导出通道供其他包使用
	TokenSellChan   = make(chan TokenInfo, 1) // 导出通道供其他包使用
)
