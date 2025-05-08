package calculator

import (
	"context"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

// AmountsCalculator 金额计算器
type AmountsCalculator struct {
	client *rpc.Client
}

func NewAmountsCalculator(endpoint string) *AmountsCalculator {
	return &AmountsCalculator{
		client: rpc.New(endpoint),
	}
}

// CalculateBuyAmounts 计算购买所需的金额参数
func (c *AmountsCalculator) CalculateBuyAmounts(ctx context.Context, bondingCurve solana.PublicKey, solAmount float64, virtualSolReserves uint64, slippage float64) (*BuyAmounts, error) {

	// 计算bonding curve
	base := float64(1073000191)
	coef := float64(32190005730)
	oneBillion := float64(1000000000)

	virtualSol := float64(virtualSolReserves)
	virtualSol2 := virtualSol + (solAmount * oneBillion)

	pay1 := base - (coef / (virtualSol / oneBillion))
	pay2 := base - (coef / (virtualSol2 / oneBillion))

	tokenAmount := pay2 - pay1
	tokenAmountWithDecimals := uint64(tokenAmount * 1e6)

	// 计算基础金额
	solAmountWithDecimals := uint64(solAmount * 1e9)

	// 计算最大花费（考虑滑点）
	maxCost := uint64(float64(solAmountWithDecimals) * (1 + slippage))

	return &BuyAmounts{
		TokenAmount: tokenAmountWithDecimals,
		SolAmount:   solAmountWithDecimals,
		MaxCost:     maxCost,
	}, nil
}

// BuyAmounts 存储购买相关的金额
type BuyAmounts struct {
	TokenAmount uint64 // 获得的token数量
	SolAmount   uint64 // 支付的SOL数量
	MaxCost     uint64 // 最大花费（含滑点）
}

// CalculateSellAmounts 计算卖出所需的金额参数
func (c *AmountsCalculator) CalculateSellAmounts(ctx context.Context, bondingCurve solana.PublicKey, tokenAmount float64, virtualTokenReserves uint64, virtualSolReserves uint64, slippage float64) (*SellAmounts, error) {

	// 计算价格
	virtualSol := decimal.NewFromInt(int64(virtualSolReserves)).Div(decimal.NewFromInt(1e9))
	virtualToken := decimal.NewFromInt(int64(virtualTokenReserves)).Div(decimal.NewFromInt(1e6))
	price := virtualSol.Div(virtualToken).Round(12)

	// 计算token数量
	tokenAmountWithDecimals := uint64(tokenAmount)

	// 计算预期获得的SOL数量
	expectedSol, _ := price.Float64()
	solAmount := expectedSol * (tokenAmount / 1e6) // 将token数量转换为原始单位
	solAmountWithDecimals := uint64(solAmount * 1e9)

	// 计算最小获得数量（考虑滑点）
	minOut := uint64(float64(solAmountWithDecimals) * (1 - slippage))

	return &SellAmounts{
		TokenAmount: tokenAmountWithDecimals,
		SolAmount:   solAmountWithDecimals,
		MinOut:      minOut,
	}, nil
}

// SellAmounts 存储卖出相关的金额
type SellAmounts struct {
	TokenAmount uint64 // 卖出的token数量
	SolAmount   uint64 // 预期获得的SOL数量
	MinOut      uint64 // 最小获得数量（考虑滑点）
}

// TokensForSol 计算SOL兑换代币的数量
func (c *AmountsCalculator) CalculateAmmBuyAmounts(solAmount float64, baseReserve, quoteReserve uint64, slippage float64) (*SellAmounts, error) {
	// 使用恒定乘积公式计算
	// (x + Δx) * (y - Δy) = x * y
	// 解出 Δy

	// 转换为big.Int进行精确计算
	quoteAmountInBig := new(big.Int).SetUint64(uint64(solAmount * 1e9))
	baseReserveBig := new(big.Int).SetUint64(baseReserve)
	quoteReserveBig := new(big.Int).SetUint64(quoteReserve)

	// 计算 x * y
	product := new(big.Int).Mul(baseReserveBig, quoteReserveBig)

	// 计算 y + Δy
	newQuoteReserve := new(big.Int).Add(quoteReserveBig, quoteAmountInBig)

	// 计算 (x * y) / (y + Δy)
	newBaseReserve := new(big.Int).Div(product, newQuoteReserve)

	// 计算 x - ((x * y) / (y + Δy))
	baseAmountOut := new(big.Int).Sub(baseReserveBig, newBaseReserve)

	// 计算最小获得数量（考虑滑点）
	minOut := uint64(float64(baseAmountOut.Uint64()) * (1 - slippage))

	return &SellAmounts{
		TokenAmount: baseAmountOut.Uint64(),
		SolAmount:   uint64(solAmount * 1e9),
		MinOut:      minOut,
	}, nil
}

// TokensForSol 计算代币兑换SOL的数量
func (c *AmountsCalculator) CalculateAmmSellAmounts(baseAmountIn, baseReserve, quoteReserve uint64, slippage float64) (*SellAmounts, error) {
	// 使用恒定乘积公式计算
	// (x + Δx) * (y - Δy) = x * y
	// 解出 Δy

	// 转换为big.Int进行精确计算
	baseAmountInBig := new(big.Int).SetUint64(baseAmountIn)
	baseReserveBig := new(big.Int).SetUint64(baseReserve)
	quoteReserveBig := new(big.Int).SetUint64(quoteReserve)

	// 计算 x * y
	product := new(big.Int).Mul(baseReserveBig, quoteReserveBig)

	// 计算 x + Δx
	newBaseReserve := new(big.Int).Add(baseReserveBig, baseAmountInBig)

	// 计算 (x * y) / (x + Δx)
	newQuoteReserve := new(big.Int).Div(product, newBaseReserve)

	// 计算 y - ((x * y) / (x + Δx))
	quoteAmountOut := new(big.Int).Sub(quoteReserveBig, newQuoteReserve)

	// 计算最小获得数量（考虑滑点）
	minOut := uint64(float64(quoteAmountOut.Uint64()) * (1 - slippage))

	return &SellAmounts{
		TokenAmount: baseAmountIn,
		SolAmount:   quoteAmountOut.Uint64(),
		MinOut:      minOut,
	}, nil
}
