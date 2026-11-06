package pumpV3

import (
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
)

// 本文件复刻官方 @pump-fun/pump-sdk 的 fees.ts / bondingCurve.ts 计算逻辑，
// 全部使用整数运算（math/big），与链上程序一致。
//
// 费用模型（关键结论，已与官方 TS / Rust SDK 双向核对）：
//
//	用户买入总成本 = 曲线成本 + 协议费 + creator 费
//	用户卖出净收入 = 曲线产出 - 协议费 - creator 费
//
// buyback 费不再另行叠加到用户成本上（它从协议费内部分账给 buyback recipient），
// 因此 max_sol_cost / min_sol_output 只需覆盖「曲线 + 协议 + creator」三项。
// LP 费只存在于 PumpSwap 外盘，内盘不收。

// OneBillionSupply 是非 mayhem 代币计算市值时使用的固定供应量（10 亿 × 10^6）。
// mayhem 代币必须使用 BondingCurve.TokenTotalSupply。
var OneBillionSupply = uint64(1_000_000_000_000_000)

// WSOLMint2022 是 Token-2022 程序下的 wrapped SOL。
// create_v2 拒绝以它作为 quote，但费率程序把它归类为 SOL 系。
var WSOLMint2022 = solana.MustPublicKeyFromBase58("9pan9bMn5HatX4EJdBwg9VgCa7Uz5HL8N1m5D3NdXejP")

// IsSolLikeQuoteMint 判断 quote 是否按 SOL 档费率计费：
// 零值（历史 SOL 曲线）、wSOL、Token-2022 wSOL 三者都算。
func IsSolLikeQuoteMint(quoteMint solana.PublicKey) bool {
	return quoteMint.IsZero() || quoteMint.Equals(WSOLMint) || quoteMint.Equals(WSOLMint2022)
}

// IsStableQuoteMint 判断 quote 是否按稳定币档费率计费（目前只有主网 USDC）。
func IsStableQuoteMint(quoteMint solana.PublicKey) bool {
	return quoteMint.Equals(USDCMint)
}

// FeeBps 是一笔交易实际适用的费率（基点）。
type FeeBps struct {
	Protocol uint64
	Creator  uint64
}

// Total 返回总费率。买入时用于从预算中扣除费用，
// 注意：只有当 creator 费确实收取时才能把 Creator 计入（见 GetFee）。
func (f FeeBps) Total() uint64 {
	return f.Protocol + f.Creator
}

// FeeForAmount 计算单项费用：ceil(amount × bps / 10000)，与链上 ceil_div 一致。
func FeeForAmount(amount, bps uint64) uint64 {
	if bps == 0 || amount == 0 {
		return 0
	}
	a := new(big.Int).SetUint64(amount)
	a.Mul(a, new(big.Int).SetUint64(bps))
	a.Add(a, big.NewInt(9999))
	a.Quo(a, big.NewInt(10000))
	return a.Uint64()
}

// GetFee 计算一笔金额上的总费用 = 协议费 + creator 费。
// creator 费仅在收取时计入：新建曲线或曲线设置了 creator（creator 非零地址）。
// 历史 V1 币的 BondingCurve.creator 为零，不收 creator 费。
func GetFee(amount uint64, bps FeeBps, creator solana.PublicKey) uint64 {
	feeAmount := FeeForAmount(amount, bps.Protocol)
	if !creator.IsZero() {
		feeAmount += FeeForAmount(amount, bps.Creator)
	}
	return feeAmount
}

// MarketCap 计算市值（以 quote 最小单位计）：
//
//	marketCap = virtualQuoteReserves × mintSupply / virtualTokenReserves
func MarketCap(virtualQuoteReserves, mintSupply, virtualTokenReserves uint64) (*big.Int, error) {
	if virtualTokenReserves == 0 {
		return nil, fmt.Errorf("virtualTokenReserves 为 0，无法计算市值（曲线可能已迁移）")
	}
	out := new(big.Int).SetUint64(virtualQuoteReserves)
	out.Mul(out, new(big.Int).SetUint64(mintSupply))
	out.Quo(out, new(big.Int).SetUint64(virtualTokenReserves))
	return out, nil
}

// CalculateFeeTier 按市值选择费率档位（整型比较，与 pump-fees-math::calculate_fee_tier 一致）：
// 市值低于第一档阈值 → 第一档；否则从高到低取第一个 市值 ≥ 阈值 的档位。
func CalculateFeeTier(tiers []FeeTier, marketCap *big.Int) (Fees, error) {
	if len(tiers) == 0 {
		return Fees{}, fmt.Errorf("费率档位为空")
	}
	first := tiers[0]
	if marketCap.Cmp(first.MarketCapLamportsThreshold.BigInt()) < 0 {
		return first.Fees, nil
	}
	for i := len(tiers) - 1; i >= 0; i-- {
		if marketCap.Cmp(tiers[i].MarketCapLamportsThreshold.BigInt()) >= 0 {
			return tiers[i].Fees, nil
		}
	}
	return first.Fees, nil
}

// SelectCurveFeeSchedule 按 quote mint 选择费率表（等价于 FeeConfig::fees_for_quote_mint, is_pump_pool=true）：
// SOL 系 → FeeTiers；USDC → StableFeeTiers（为空时回退 FeeTiers）；
// 其他 exotic quote → ExoticFlatFees（全零时回退 FlatFees）。
func SelectCurveFeeSchedule(cfg *FeeConfig, quoteMint solana.PublicKey, marketCap *big.Int) (Fees, error) {
	if cfg == nil {
		return Fees{}, fmt.Errorf("FeeConfig 为空")
	}
	switch {
	case IsSolLikeQuoteMint(quoteMint):
		return CalculateFeeTier(cfg.FeeTiers, marketCap)
	case IsStableQuoteMint(quoteMint):
		tiers := cfg.StableFeeTiers
		if len(tiers) == 0 {
			tiers = cfg.FeeTiers
		}
		return CalculateFeeTier(tiers, marketCap)
	default:
		if cfg.ExoticFlatFees.LpFeeBps == 0 && cfg.ExoticFlatFees.ProtocolFeeBps == 0 && cfg.ExoticFlatFees.CreatorFeeBps == 0 {
			return cfg.FlatFees, nil
		}
		return cfg.ExoticFlatFees, nil
	}
}

// ComputeFeeBps 计算一笔内盘交易适用的费率。
//
//	feeConfig        链上 FeeConfig（pump_fees PDA）；为 nil 时回退 Global 的固定费率
//	global           Global 配置（可空）：提供 creator_fee_configurable 开关与回退费率
//	quoteMint        曲线自身的 quote mint（BondingCurve.quote_mint，SOL 曲线为零值）
//	coinCreatorFeeBps 曲线自定义 creator 费率（BondingCurve.creator_fee_bps）
//	mintSupply       mayhem 币用真实供应量，其他币传 OneBillionSupply
func ComputeFeeBps(feeConfig *FeeConfig, global *Global, quoteMint solana.PublicKey, coinCreatorFeeBps, mintSupply, virtualQuoteReserves, virtualTokenReserves uint64) (FeeBps, error) {
	if feeConfig != nil {
		marketCap, err := MarketCap(virtualQuoteReserves, mintSupply, virtualTokenReserves)
		if err != nil {
			return FeeBps{}, err
		}
		schedule, err := SelectCurveFeeSchedule(feeConfig, quoteMint, marketCap)
		if err != nil {
			return FeeBps{}, err
		}
		// creator_fee_configurable 开关打开且曲线设置了自定义费率时，覆盖档位里的 creator 费率。
		// 当前链上该开关为 false；Global 缺失时也按 false 处理。
		if global != nil && global.CreatorFeeConfigurable && coinCreatorFeeBps != 0 {
			return FeeBps{Protocol: schedule.ProtocolFeeBps, Creator: coinCreatorFeeBps}, nil
		}
		return FeeBps{Protocol: schedule.ProtocolFeeBps, Creator: schedule.CreatorFeeBps}, nil
	}
	if global != nil {
		return FeeBps{Protocol: global.FeeBasisPoints, Creator: global.CreatorFeeBasisPoints}, nil
	}
	return FeeBps{}, fmt.Errorf("FeeConfig 与 Global 均不可用，无法计算费率")
}

// EffectiveMintSupply 返回费率计算应使用的供应量：
// mayhem 币用真实供应量，普通币固定 OneBillionSupply。
func EffectiveMintSupply(isMayhemMode bool, realMintSupply uint64) uint64 {
	if isMayhemMode {
		return realMintSupply
	}
	return OneBillionSupply
}

// BuyTokenAmountFromQuoteBudget 给定 quote 预算（含费用），计算能买到的代币数量。
// 与官方 getBuyTokenAmountFromSolAmount 一致：
//
//	inputAmount = (budget - 1) × 10000 / (totalFeeBps + 10000)
//	tokens = inputAmount × virtualTokenReserves / (virtualQuoteReserves + inputAmount)
//	tokens = min(tokens, realTokenReserves)
//
// totalFeeBps 必须是实际收取的费率之和（不收 creator 费时不含 creator 部分）。
func BuyTokenAmountFromQuoteBudget(budget, virtualQuoteReserves, virtualTokenReserves, realTokenReserves, totalFeeBps uint64) (uint64, error) {
	if budget == 0 {
		return 0, nil
	}
	if virtualTokenReserves == 0 {
		return 0, fmt.Errorf("virtualTokenReserves 为 0（曲线可能已迁移），无法买入")
	}
	input := new(big.Int).SetUint64(budget - 1)
	input.Mul(input, big.NewInt(10000))
	input.Quo(input, new(big.Int).SetUint64(totalFeeBps+10000))

	tokens := new(big.Int).Mul(input, new(big.Int).SetUint64(virtualTokenReserves))
	denom := new(big.Int).Add(new(big.Int).SetUint64(virtualQuoteReserves), input)
	tokens.Quo(tokens, denom)

	if !tokens.IsUint64() {
		return 0, fmt.Errorf("代币数量溢出")
	}
	out := tokens.Uint64()
	if out > realTokenReserves {
		out = realTokenReserves
	}
	return out, nil
}

// BuyQuoteCostFromTokenAmount 给定代币数量，计算买入总成本（曲线成本 + 费用）。
// 与官方 getBuySolAmountFromTokenAmount 一致：
//
//	minAmount = min(amount, realTokenReserves)
//	quoteCost = minAmount × virtualQuoteReserves / (virtualTokenReserves - minAmount) + 1
//	total = quoteCost + GetFee(quoteCost)
func BuyQuoteCostFromTokenAmount(amount, virtualQuoteReserves, virtualTokenReserves, realTokenReserves uint64, bps FeeBps, creator solana.PublicKey) (uint64, error) {
	if amount == 0 {
		return 0, nil
	}
	if virtualTokenReserves == 0 {
		return 0, fmt.Errorf("virtualTokenReserves 为 0（曲线可能已迁移），无法买入")
	}
	minAmount := amount
	if minAmount > realTokenReserves {
		minAmount = realTokenReserves
	}
	denom := virtualTokenReserves - minAmount
	if denom == 0 {
		return 0, fmt.Errorf("代币数量已达曲线可售上限")
	}

	cost := new(big.Int).SetUint64(minAmount)
	cost.Mul(cost, new(big.Int).SetUint64(virtualQuoteReserves))
	cost.Quo(cost, new(big.Int).SetUint64(denom))
	cost.Add(cost, big.NewInt(1))

	if !cost.IsUint64() {
		return 0, fmt.Errorf("买入成本溢出")
	}
	quoteCost := cost.Uint64()
	return quoteCost + GetFee(quoteCost, bps, creator), nil
}

// SellQuoteOutputFromTokenAmount 给定代币数量，计算卖出净收入（曲线产出 - 费用）。
// 与官方 getSellSolAmountFromTokenAmount 一致：
//
//	quoteOut = amount × virtualQuoteReserves / (virtualTokenReserves + amount)
//	net = quoteOut - GetFee(quoteOut)
func SellQuoteOutputFromTokenAmount(amount, virtualQuoteReserves, virtualTokenReserves uint64, bps FeeBps, creator solana.PublicKey) (uint64, error) {
	if amount == 0 {
		return 0, nil
	}
	if virtualTokenReserves == 0 {
		return 0, fmt.Errorf("virtualTokenReserves 为 0（曲线可能已迁移），无法卖出")
	}

	out := new(big.Int).SetUint64(amount)
	out.Mul(out, new(big.Int).SetUint64(virtualQuoteReserves))
	out.Quo(out, new(big.Int).Add(new(big.Int).SetUint64(virtualTokenReserves), new(big.Int).SetUint64(amount)))

	if !out.IsUint64() {
		return 0, fmt.Errorf("卖出产出溢出")
	}
	quoteOut := out.Uint64()
	fee := GetFee(quoteOut, bps, creator)
	if fee >= quoteOut {
		return 0, nil
	}
	return quoteOut - fee, nil
}
