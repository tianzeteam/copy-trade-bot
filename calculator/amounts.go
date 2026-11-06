package calculator

import (
	"fmt"
	"math"
	"math/big"

	"copytrade/global"
	"copytrade/logger"
	pumpV3 "copytrade/pumpV3"

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

// CalculatePumpV3BuyAmounts 按已缓存的曲线快照计算买入金额（fee-aware）。
func (c *AmountsCalculator) CalculatePumpV3BuyAmounts(snapshot global.PumpCurveSnapshot, quoteBudget uint64, slippage float64) (*BuyAmounts, error) {
	if snapshot.Mint.IsZero() {
		return nil, fmt.Errorf("Pump 曲线快照为空")
	}
	if quoteBudget == 0 {
		return nil, fmt.Errorf("买入预算为 0")
	}
	if slippage < 0 || slippage >= 1 {
		return nil, fmt.Errorf("滑点必须在 [0, 1) 区间")
	}
	if snapshot.IsMayhemMode && snapshot.TokenTotalSupply == 0 {
		return nil, fmt.Errorf("mayhem 曲线供应量尚未缓存")
	}

	bps, err := c.pumpFeeBps(snapshot)
	if err != nil {
		return nil, err
	}
	chargedCreatorBps := bps.Creator
	if snapshot.Creator.IsZero() {
		chargedCreatorBps = 0
	}

	tokens, err := pumpV3.BuyTokenAmountFromQuoteBudget(
		quoteBudget,
		snapshot.VirtualQuoteReserves,
		snapshot.VirtualTokenReserves,
		snapshot.RealTokenReserves,
		bps.Protocol+chargedCreatorBps,
	)
	if err != nil {
		return nil, err
	}
	if tokens == 0 {
		return nil, fmt.Errorf("预算 %d 过小，无法买到任何代币", quoteBudget)
	}

	maxCost := uint64(float64(quoteBudget) * (1 + slippage))
	return &BuyAmounts{
		TokenAmount: tokens,
		SolAmount:   quoteBudget,
		MaxCost:     maxCost,
	}, nil
}

// CalculatePumpV3SellAmounts 按已缓存的曲线快照计算卖出金额（fee-aware）。
func (c *AmountsCalculator) CalculatePumpV3SellAmounts(snapshot global.PumpCurveSnapshot, tokenAmount uint64, slippage float64) (*SellAmounts, error) {
	if snapshot.Mint.IsZero() {
		return nil, fmt.Errorf("Pump 曲线快照为空")
	}
	if tokenAmount == 0 {
		return nil, fmt.Errorf("卖出数量为 0")
	}
	if slippage < 0 || slippage >= 1 {
		return nil, fmt.Errorf("滑点必须在 [0, 1) 区间")
	}
	if snapshot.IsMayhemMode && snapshot.TokenTotalSupply == 0 {
		return nil, fmt.Errorf("mayhem 曲线供应量尚未缓存")
	}

	bps, err := c.pumpFeeBps(snapshot)
	if err != nil {
		return nil, err
	}
	net, err := pumpV3.SellQuoteOutputFromTokenAmount(
		tokenAmount,
		snapshot.VirtualQuoteReserves,
		snapshot.VirtualTokenReserves,
		bps,
		snapshot.Creator,
	)
	if err != nil {
		return nil, err
	}

	minOut := uint64(float64(net) * (1 - slippage))
	return &SellAmounts{
		TokenAmount: tokenAmount,
		SolAmount:   net,
		MinOut:      minOut,
	}, nil
}

func (c *AmountsCalculator) pumpFeeBps(snapshot global.PumpCurveSnapshot) (pumpV3.FeeBps, error) {
	feeConfig, globalConfig := global.GetPumpFeeConfig()
	if feeConfig == nil && globalConfig == nil {
		logger.Warnf("泵费率配置不可用，按保守费率 200bps 计算")
		return pumpV3.FeeBps{Protocol: 100, Creator: 100}, nil
	}

	mintSupply := snapshot.TokenTotalSupply
	if mintSupply == 0 {
		mintSupply = pumpV3.OneBillionSupply
	}
	return pumpV3.ComputeFeeBps(
		feeConfig,
		globalConfig,
		snapshot.QuoteMint,
		snapshot.CreatorFeeBps,
		mintSupply,
		snapshot.VirtualQuoteReserves,
		snapshot.VirtualTokenReserves,
	)
}

// BuyAmounts 存储购买相关的金额
type BuyAmounts struct {
	TokenAmount  uint64 // 获得的token数量
	SolAmount    uint64 // 支付的SOL数量
	MaxSolAmount uint64 // 最大支付的SOL数量
	MaxCost      uint64 // 最大花费（含滑点）
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

	// 计算最大花费（考虑滑点）
	quoteAmountInFloat, _ := quoteAmountInBig.Float64()
	maxCost := uint64(quoteAmountInFloat * (1 + slippage))

	return &BuyAmounts{
		TokenAmount: baseAmountOut.Uint64(),
		SolAmount:   uint64(solAmount * 1e9),
		MaxCost:     maxCost,
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

// TokensForSol 计算SOL兑换代币的数量
func (c *AmountsCalculator) CalculateRaydiumLaunchpadBuyAmounts(solAmount float64, buyTipFee float64, prePrice float64, tokenDecimals uint64, slippage float64) (*BuyAmounts, error) {
	// 使用最近的交易价格预估实时买入数量
	// (solAmount / prePrice) * (1-slippage)

	// 检查价格和精度是否有效
	if prePrice <= 0 {
		return nil, fmt.Errorf("invalid pre-trade price: %f", prePrice)
	}

	// 计算SOL金额（以lamports为单位）
	// 1 SOL = 1e9 lamports
	tipFeeLamports := buyTipFee * 1e9
	solAmountLamports := solAmount * 1e9

	// 计算最大支付的SOL数量
	maxSolAmountLamports := (solAmountLamports + tipFeeLamports) * (1 + 0.2)

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
	logger.Infof("Raydium Launchpad Buy Calculation: SOL=%f, Price=%f, TokenDecimals=%d, Slippage=%f, ExpectedTokens=%f, MinTokens=%f",
		solAmount, prePrice, tokenDecimals, slippage, expectedTokenAmountWithDecimals, minTokenAmountWithDecimals)

	return &BuyAmounts{
		TokenAmount:  tokenAmountUint64,
		SolAmount:    uint64(solAmountLamports),
		MaxSolAmount: uint64(maxSolAmountLamports),
		MaxCost:      minOutUint64,
	}, nil
}

// TokensForSol 计算SOL兑换代币的数量
func (c *AmountsCalculator) CalculateRaydiumCPMMBuyAmounts(solAmount float64, buyTipFee float64, prePrice float64, tokenDecimals uint64, slippage float64) (*BuyAmounts, error) {
	// 使用最近的交易价格预估实时买入数量
	// (solAmount / prePrice) * (1-slippage)

	// 检查价格和精度是否有效
	if prePrice <= 0 {
		return nil, fmt.Errorf("invalid pre-trade price: %f", prePrice)
	}

	// 计算SOL金额（以lamports为单位）
	// 1 SOL = 1e9 lamports
	tipFeeLamports := buyTipFee * 1e9
	solAmountLamports := solAmount * 1e9

	// 计算最大支付的SOL数量
	maxSolAmountLamports := (solAmountLamports + tipFeeLamports) * (1 + 0.2)

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
	logger.Infof("Raydium CPMM Buy Calculation: SOL=%f, Price=%.8f, TokenDecimals=%d, Slippage=%f, ExpectedTokens=%f, MinTokens=%f",
		solAmount, prePrice, tokenDecimals, slippage, expectedTokenAmountWithDecimals, minTokenAmountWithDecimals)

	return &BuyAmounts{
		TokenAmount:  tokenAmountUint64,
		MaxSolAmount: uint64(maxSolAmountLamports),
		SolAmount:    uint64(solAmountLamports),
		MaxCost:      minOutUint64,
	}, nil
}

// TokensForSol 计算SOL兑换代币的数量
func (c *AmountsCalculator) CalculateMeteoraDbcBuyAmounts(solAmount float64, buyTipFee float64, prePrice float64, tokenDecimals uint64, slippage float64) (*BuyAmounts, error) {
	// 使用最近的交易价格预估实时买入数量
	// (solAmount / prePrice) * (1-slippage)

	// 检查价格和精度是否有效
	if prePrice <= 0 {
		return nil, fmt.Errorf("invalid pre-trade price: %f", prePrice)
	}

	// 计算SOL金额（以lamports为单位）
	// 1 SOL = 1e9 lamports
	tipFeeLamports := buyTipFee * 1e9
	solAmountLamports := solAmount * 1e9

	// 计算最大支付的SOL数量
	maxSolAmountLamports := (solAmountLamports + tipFeeLamports) * (1 + 0.2)

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
	logger.Infof("Meteora dbc Buy Calculation: SOL=%f, Price=%f, TokenDecimals=%d, Slippage=%f, ExpectedTokens=%f, MinTokens=%f",
		solAmount, prePrice, tokenDecimals, slippage, expectedTokenAmountWithDecimals, minTokenAmountWithDecimals)

	return &BuyAmounts{
		TokenAmount:  tokenAmountUint64,
		MaxSolAmount: uint64(maxSolAmountLamports),
		SolAmount:    uint64(solAmountLamports),
		MaxCost:      minOutUint64,
	}, nil
}

// Sol to Token 计算代币兑换SOL的数量
func (c *AmountsCalculator) CalculateMeteoraDbcSellAmounts(amountInWithDecimals float64, prePrice float64, tokenDecimals uint64, slippage float64) (*SellAmounts, error) {
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
