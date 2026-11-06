package pumpV3

import (
	solana "github.com/gagliardetto/solana-go"
)

// 程序地址。
var (
	// ProgramID 是 Pump.fun 内盘（bonding curve）程序。
	ProgramID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")

	// FeeProgramID 是 Pump Fees 程序，负责动态费率配置与 creator fee 分账。
	// buy_v2 / sell_v2 必须把它作为 fee_program 账户传入。
	FeeProgramID = solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")

	// PumpAmmProgramID 是 PumpSwap（外盘 AMM）程序，内盘毕业后流动性迁移至此。
	PumpAmmProgramID = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
)

// 固定 PDA 与常用 mint。
var (
	// GlobalAccount 是唯一的 Global 配置账户，seeds = ["global"]。
	GlobalAccount = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")

	// EventAuthority 是 Anchor 事件签发 PDA，seeds = ["__event_authority"]。
	EventAuthority = solana.MustPublicKeyFromBase58("Ce6TQqeHC9p8KetsN6JsjHK7UTZk7nasjjnr7XxXp9F1")

	// WSOLMint 是 wrapped SOL。SOL 配对币在 buy_v2 / sell_v2 中
	// 必须把 quote_mint 传成本地址（而非原生 SOL 的全零地址），
	// 实际资金划转仍使用原生 SOL。
	WSOLMint = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

	// USDCMint 是 Pump.fun 支持的第二个 quote 资产。
	USDCMint = solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
)

// PDA 种子常量。
const (
	SeedGlobal                  = "global"
	SeedBondingCurve            = "bonding-curve"
	SeedCreatorVault            = "creator-vault"
	SeedSharingConfig           = "sharing-config"
	SeedFeeConfig               = "fee_config"
	SeedGlobalVolumeAccumulator = "global_volume_accumulator"
	SeedUserVolumeAccumulator   = "user_volume_accumulator"
	SeedEventAuthority          = "__event_authority"
)

// QuoteDecimals 返回 quote mint 的小数位数。
// SOL / wSOL 为 9，USDC 为 6。未知 mint 回退到 9 并应由调用方记录告警。
func QuoteDecimals(quoteMint solana.PublicKey) uint32 {
	switch {
	case quoteMint.Equals(WSOLMint), quoteMint.Equals(WSOLMint2022), quoteMint.IsZero():
		// quote_mint 为零值表示 SOL 配对币（历史数据），按 SOL 处理。
		return 9
	case quoteMint.Equals(USDCMint):
		return 6
	default:
		return 9
	}
}

// IsSupportedQuoteMint 判断当前 BOT 是否支持该 quote mint。
func IsSupportedQuoteMint(quoteMint solana.PublicKey) bool {
	return IsSolLikeQuoteMint(quoteMint) || IsStableQuoteMint(quoteMint)
}

// IsSolPaired 判断代币是否以 SOL 计价。
// BondingCurve.quote_mint 对所有 SOL 配对币都是 Pubkey::default()（全零），
// 而指令参数里的 quote_mint 则要传 wSOL，两者不能混淆。
func IsSolPaired(bondingCurveQuoteMint solana.PublicKey) bool {
	return bondingCurveQuoteMint.IsZero() || bondingCurveQuoteMint.Equals(WSOLMint)
}
