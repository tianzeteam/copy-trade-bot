package calculator

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
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

	_virtualSol := float64(virtualSolReserves)
	_afterVirtualSol := _virtualSol + (solAmount * 1e9)

	pay1 := base - (coef / (_virtualSol / 1e9))
	pay2 := base - (coef / (_afterVirtualSol / 1e9))

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

	// 转换为 float64 类型进行计算
	virtualSolFloat := float64(virtualSolReserves) / 1e9
	virtualTokenFloat := float64(virtualTokenReserves) / 1e6
	
	// 计算价格
	priceFloat := virtualSolFloat / virtualTokenFloat
	
	// 计算token数量
	tokenAmountWithDecimals := uint64(tokenAmount)

	// 计算预期获得的SOL数量
	solAmount := priceFloat * (tokenAmount / 1e6) // 将token数量转换为原始单位
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
func (c *AmountsCalculator) CalculateAmmBuyAmounts(solAmount float64, baseReserve, quoteReserve uint64, slippage float64) (*BuyAmounts, error) {
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

	return &BuyAmounts{
		TokenAmount: baseAmountOut.Uint64(),
		SolAmount:   uint64(solAmount * 1e9),
		MaxCost:     minOut,
	}, nil
}

// TokensForSol 计算SOL兑换代币的数量
func (c *AmountsCalculator) CalculateRaydiumV4BuyAmounts(solAmount float64, prePrice float64, tokenDecimals uint64, slippage float64) (*BuyAmounts, error) {
	// 使用最近的交易价格预估实时买入数量
	// (solAmount / prePrice) * (1-slippage)

	// 检查价格和精度是否有效
	if prePrice <= 0 {
		return nil, fmt.Errorf("invalid pre-trade price: %f", prePrice)
	}

	// 计算SOL金额（以lamports为单位）
	// 1 SOL = 1e9 lamports
	solAmountLamports := solAmount * 1e9

	// 计算代币精度系数
	tokenDecimalMultiplier := math.Pow10(int(tokenDecimals))

	// 计算预期获得的原始代币数量（不考虑精度）
	// 公式: (solAmount / prePrice)
	expectedTokenAmount := solAmount / prePrice

	// 将代币数量转换为带精度的数量
	expectedTokenAmountWithDecimals := expectedTokenAmount * tokenDecimalMultiplier

	// 考虑滑点，计算最小获得数量
	// 公式: (solAmount / prePrice) * (1-slippage) * tokenDecimalMultiplier
	minTokenAmountWithDecimals := expectedTokenAmountWithDecimals * (1 - slippage)

	// 转换为uint64类型
	tokenAmountUint64 := uint64(expectedTokenAmountWithDecimals)
	minOutUint64 := uint64(minTokenAmountWithDecimals)

	// 记录详细的计算过程
	logger.Infof("Raydium V4 Buy Calculation: SOL=%f, Price=%f, TokenDecimals=%d, Slippage=%f, ExpectedTokens=%f, MinTokens=%f",
		solAmount, prePrice, tokenDecimals, slippage, expectedTokenAmountWithDecimals, minTokenAmountWithDecimals)

	return &BuyAmounts{
		TokenAmount: tokenAmountUint64,
		SolAmount:   uint64(solAmountLamports),
		MaxCost:     minOutUint64,
	}, nil
}

// Sol to Token 计算代币兑换SOL的数量
func (c *AmountsCalculator) CalculateRaydiumV4SellAmounts(amountInWithDecimals float64, prePrice float64, tokenDecimals uint64, slippage float64) (*SellAmounts, error) {
	// 使用最近的交易价格预估实时买入数量
	// (amountInWithDecimals/math.Pow10(int(tokenDecimals)) * prePrice) * (1-slippage)

	// 检查价格和精度是否有效
	if prePrice <= 0 {
		return nil, fmt.Errorf("invalid pre-trade price: %f", prePrice)
	}

	// 计算代币精度系数
	tokenDecimalMultiplier := math.Pow10(int(tokenDecimals))

	// 计算预期获得的sol数量（不考虑精度）
	// 公式: (tokenAmount * prePrice)
	expectedSolAmount := (amountInWithDecimals / tokenDecimalMultiplier) * prePrice

	// 将代币数量转换为带精度的数量
	expectedSolAmountLamports := expectedSolAmount * 1e9

	// 考虑滑点，计算最小获得数量
	// 公式: (solAmount / prePrice) * (1-slippage) * tokenDecimalMultiplier
	minSolAmountWithDecimals := expectedSolAmountLamports * (1 - slippage)

	// 转换为uint64类型
	solAmountUint64 := uint64(expectedSolAmountLamports)
	minOutUint64 := uint64(minSolAmountWithDecimals)

	// 记录详细的计算过程
	logger.Infof("Raydium V4 Sell Calculation: amountIn=%f, Price=%f, solLamport=%d, Slippage=%f, ExpectedSol=%f, MinSol=%f",
		math.Pow10(int(tokenDecimals)), prePrice, solAmountUint64, slippage, solAmountUint64, minOutUint64)

	return &SellAmounts{
		TokenAmount: uint64(amountInWithDecimals),
		SolAmount:   uint64(solAmountUint64),
		MinOut:      minOutUint64,
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
