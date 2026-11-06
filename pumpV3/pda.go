package pumpV3

import (
	"copytrade/helpers/associatedtokenaccount"

	"github.com/gagliardetto/solana-go"
)

// 本文件集中 buy_v2 / sell_v2 所需的全部 PDA 与 ATA 派生。
// 种子与所属程序均取自官方 IDL（idl/pump.json、idl/pump_fees.json），
// 修改前请先比对。

// BondingCurvePDA 派生内盘曲线账户，seeds = ["bonding-curve", mint]，program = pump。
func BondingCurvePDA(mint solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte(SeedBondingCurve), mint.Bytes()},
		ProgramID,
	)
	return addr, err
}

// CreatorVaultPDA 派生创建者金库，seeds = ["creator-vault", creator]，program = pump。
// 注意种子是 BondingCurve.creator（creator 钱包地址），不是 creator vault 自身。
func CreatorVaultPDA(creator solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte(SeedCreatorVault), creator.Bytes()},
		ProgramID,
	)
	return addr, err
}

// SharingConfigPDA 派生 creator fee 分账配置，seeds = ["sharing-config", base_mint]，
// program = pump_fees。该账户可能未初始化，指令里仅做 seed 约束，传 PDA 地址即可。
func SharingConfigPDA(baseMint solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte(SeedSharingConfig), baseMint.Bytes()},
		FeeProgramID,
	)
	return addr, err
}

// FeeConfigPDA 派生动态费率配置账户，seeds = ["fee_config", pump_program_id]，
// program = pump_fees。第二个种子是 pump 程序地址的 32 字节原始值。
func FeeConfigPDA() (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte(SeedFeeConfig), ProgramID.Bytes()},
		FeeProgramID,
	)
	return addr, err
}

// GlobalVolumeAccumulatorPDA 派生全局成交量累加器，
// seeds = ["global_volume_accumulator"]，program = pump。
func GlobalVolumeAccumulatorPDA() (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte(SeedGlobalVolumeAccumulator)},
		ProgramID,
	)
	return addr, err
}

// UserVolumeAccumulatorPDA 派生用户成交量累加器，
// seeds = ["user_volume_accumulator", user]，program = pump。
// 首次交易时程序会 init_if_needed 创建（租金约 0.0018444 SOL）。
func UserVolumeAccumulatorPDA(user solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := solana.FindProgramAddress(
		[][]byte{[]byte(SeedUserVolumeAccumulator), user.Bytes()},
		ProgramID,
	)
	return addr, err
}

// AssociatedTokenAccount 按指定 token program 派生 ATA。
// base 代币用其所属 token program（SPL Token 或 Token-2022），
// quote 侧（wSOL/USDC）固定用 SPL Token 程序。
func AssociatedTokenAccount(owner, mint, tokenProgram solana.PublicKey) (solana.PublicKey, error) {
	addr, _, err := associatedtokenaccount.FindAssociatedTokenAddressWithProgram(owner, mint, tokenProgram)
	return addr, err
}

// QuoteTokenProgram 返回 quote mint 对应的 token program。
// 目前协议只支持 wSOL 与 USDC 两种 quote，均运行在标准 SPL Token 程序上。
func QuoteTokenProgram(quoteMint solana.PublicKey) solana.PublicKey {
	return solana.TokenProgramID
}
