package pumpV3

import (
	"bytes"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// TradeAccounts 是 buy_v2 / sell_v2 / buy_exact_quote_in_v2 共用的账户集合。
// 字段顺序即链上指令的账户顺序（官方 IDL 固定布局）。
//
// GlobalVolumeAccumulator 只有 buy 系指令使用，sell_v2 不包含该账户。
type TradeAccounts struct {
	Global                             solana.PublicKey // [0] 只读，pump ["global"] PDA
	BaseMint                           solana.PublicKey // [1] 只读，基础代币 mint
	QuoteMint                          solana.PublicKey // [2] 只读，SOL 配对传 wSOL，USDC 配对传 USDC
	BaseTokenProgram                   solana.PublicKey // [3] 只读，SPL Token 或 Token-2022
	QuoteTokenProgram                  solana.PublicKey // [4] 只读，wSOL/USDC 均为 SPL Token
	AssociatedTokenProgram             solana.PublicKey // [5] 只读，SPL ATA 程序
	FeeRecipient                       solana.PublicKey // [6] 可写，普通币任选 Normal，mayhem 币必须 Reserved
	AssociatedQuoteFeeRecipient        solana.PublicKey // [7] 可写，fee_recipient 的 quote ATA
	BuybackFeeRecipient                solana.PublicKey // [8] 可写，任选 BuybackFeeRecipients
	AssociatedQuoteBuybackFeeRecipient solana.PublicKey // [9] 可写，buyback recipient 的 quote ATA
	BondingCurve                       solana.PublicKey // [10] 可写，["bonding-curve", base_mint] PDA
	AssociatedBaseBondingCurve         solana.PublicKey // [11] 可写，曲线的 base ATA
	AssociatedQuoteBondingCurve        solana.PublicKey // [12] 可写，曲线的 quote ATA（SOL 配对也要求传）
	User                               solana.PublicKey // [13] 可写+签名，交易发起者
	AssociatedBaseUser                 solana.PublicKey // [14] 可写，用户的 base ATA（买入前需存在或预创建）
	AssociatedQuoteUser                solana.PublicKey // [15] 可写，用户的 quote ATA；SOL 配对仅做 seed 约束，无需初始化
	CreatorVault                       solana.PublicKey // [16] 可写，["creator-vault", bonding_curve.creator] PDA
	AssociatedCreatorVault             solana.PublicKey // [17] 可写，creator vault 的 quote ATA
	SharingConfig                      solana.PublicKey // [18] 只读，pump_fees ["sharing-config", base_mint] PDA，可未初始化
	GlobalVolumeAccumulator            solana.PublicKey // [19] 只读，仅 buy 系指令
	UserVolumeAccumulator              solana.PublicKey // buy[20]/sell[19] 可写，["user_volume_accumulator", user] PDA
	AssociatedUserVolumeAccumulator    solana.PublicKey // buy[21]/sell[20] 可写，UVA 的 quote ATA
	FeeConfig                          solana.PublicKey // buy[22]/sell[21] 只读，pump_fees ["fee_config", pump_program] PDA
	FeeProgram                         solana.PublicKey // buy[23]/sell[22] 只读，pump_fees 程序地址
	SystemProgram                      solana.PublicKey // buy[24]/sell[23] 只读
	EventAuthority                     solana.PublicKey // buy[25]/sell[24] 只读，["__event_authority"] PDA
	Program                            solana.PublicKey // buy[26]/sell[25] 只读，pump 程序地址
}

// Validate 检查账户集合中的必填项是否齐备。
func (acc TradeAccounts) Validate() error {
	required := []struct {
		name string
		key  solana.PublicKey
	}{
		{"Global", acc.Global},
		{"BaseMint", acc.BaseMint},
		{"QuoteMint", acc.QuoteMint},
		{"BaseTokenProgram", acc.BaseTokenProgram},
		{"QuoteTokenProgram", acc.QuoteTokenProgram},
		{"AssociatedTokenProgram", acc.AssociatedTokenProgram},
		{"FeeRecipient", acc.FeeRecipient},
		{"AssociatedQuoteFeeRecipient", acc.AssociatedQuoteFeeRecipient},
		{"BuybackFeeRecipient", acc.BuybackFeeRecipient},
		{"AssociatedQuoteBuybackFeeRecipient", acc.AssociatedQuoteBuybackFeeRecipient},
		{"BondingCurve", acc.BondingCurve},
		{"AssociatedBaseBondingCurve", acc.AssociatedBaseBondingCurve},
		{"AssociatedQuoteBondingCurve", acc.AssociatedQuoteBondingCurve},
		{"User", acc.User},
		{"AssociatedBaseUser", acc.AssociatedBaseUser},
		{"AssociatedQuoteUser", acc.AssociatedQuoteUser},
		{"CreatorVault", acc.CreatorVault},
		{"AssociatedCreatorVault", acc.AssociatedCreatorVault},
		{"SharingConfig", acc.SharingConfig},
		{"UserVolumeAccumulator", acc.UserVolumeAccumulator},
		{"AssociatedUserVolumeAccumulator", acc.AssociatedUserVolumeAccumulator},
		{"FeeConfig", acc.FeeConfig},
		{"FeeProgram", acc.FeeProgram},
		{"EventAuthority", acc.EventAuthority},
		{"Program", acc.Program},
	}
	if !acc.SystemProgram.Equals(solana.SystemProgramID) {
		return fmt.Errorf("pumpV3: SystemProgram 无效")
	}
	for _, r := range required {
		if r.key.IsZero() {
			return fmt.Errorf("pumpV3: 账户 %s 未设置", r.name)
		}
	}
	return nil
}

// encodeTradeArgs 编码「判别码 + 两个 u64」的 borsh 指令数据。
func encodeTradeArgs(disc [8]byte, a, b uint64) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := bin.NewBorshEncoder(buf)
	if err := enc.WriteBytes(disc[:], false); err != nil {
		return nil, fmt.Errorf("写入判别码失败: %w", err)
	}
	if err := enc.Encode(a); err != nil {
		return nil, fmt.Errorf("编码第一个参数失败: %w", err)
	}
	if err := enc.Encode(b); err != nil {
		return nil, fmt.Errorf("编码第二个参数失败: %w", err)
	}
	return buf.Bytes(), nil
}

// buyV2AccountMetas 组装 buy 系指令（buy_v2 / buy_exact_quote_in_v2）的 27 个账户。
func buyV2AccountMetas(acc TradeAccounts) []*solana.AccountMeta {
	return []*solana.AccountMeta{
		solana.NewAccountMeta(acc.Global, false, false),
		solana.NewAccountMeta(acc.BaseMint, false, false),
		solana.NewAccountMeta(acc.QuoteMint, false, false),
		solana.NewAccountMeta(acc.BaseTokenProgram, false, false),
		solana.NewAccountMeta(acc.QuoteTokenProgram, false, false),
		solana.NewAccountMeta(acc.AssociatedTokenProgram, false, false),
		solana.NewAccountMeta(acc.FeeRecipient, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteFeeRecipient, true, false),
		solana.NewAccountMeta(acc.BuybackFeeRecipient, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteBuybackFeeRecipient, true, false),
		solana.NewAccountMeta(acc.BondingCurve, true, false),
		solana.NewAccountMeta(acc.AssociatedBaseBondingCurve, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteBondingCurve, true, false),
		solana.NewAccountMeta(acc.User, true, true),
		solana.NewAccountMeta(acc.AssociatedBaseUser, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteUser, true, false),
		solana.NewAccountMeta(acc.CreatorVault, true, false),
		solana.NewAccountMeta(acc.AssociatedCreatorVault, true, false),
		solana.NewAccountMeta(acc.SharingConfig, false, false),
		solana.NewAccountMeta(acc.GlobalVolumeAccumulator, false, false),
		solana.NewAccountMeta(acc.UserVolumeAccumulator, true, false),
		solana.NewAccountMeta(acc.AssociatedUserVolumeAccumulator, true, false),
		solana.NewAccountMeta(acc.FeeConfig, false, false),
		solana.NewAccountMeta(acc.FeeProgram, false, false),
		solana.NewAccountMeta(acc.SystemProgram, false, false),
		solana.NewAccountMeta(acc.EventAuthority, false, false),
		solana.NewAccountMeta(acc.Program, false, false),
	}
}

// sellV2AccountMetas 组装 sell_v2 的 26 个账户（不含 global_volume_accumulator）。
func sellV2AccountMetas(acc TradeAccounts) []*solana.AccountMeta {
	return []*solana.AccountMeta{
		solana.NewAccountMeta(acc.Global, false, false),
		solana.NewAccountMeta(acc.BaseMint, false, false),
		solana.NewAccountMeta(acc.QuoteMint, false, false),
		solana.NewAccountMeta(acc.BaseTokenProgram, false, false),
		solana.NewAccountMeta(acc.QuoteTokenProgram, false, false),
		solana.NewAccountMeta(acc.AssociatedTokenProgram, false, false),
		solana.NewAccountMeta(acc.FeeRecipient, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteFeeRecipient, true, false),
		solana.NewAccountMeta(acc.BuybackFeeRecipient, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteBuybackFeeRecipient, true, false),
		solana.NewAccountMeta(acc.BondingCurve, true, false),
		solana.NewAccountMeta(acc.AssociatedBaseBondingCurve, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteBondingCurve, true, false),
		solana.NewAccountMeta(acc.User, true, true),
		solana.NewAccountMeta(acc.AssociatedBaseUser, true, false),
		solana.NewAccountMeta(acc.AssociatedQuoteUser, true, false),
		solana.NewAccountMeta(acc.CreatorVault, true, false),
		solana.NewAccountMeta(acc.AssociatedCreatorVault, true, false),
		solana.NewAccountMeta(acc.SharingConfig, false, false),
		solana.NewAccountMeta(acc.UserVolumeAccumulator, true, false),
		solana.NewAccountMeta(acc.AssociatedUserVolumeAccumulator, true, false),
		solana.NewAccountMeta(acc.FeeConfig, false, false),
		solana.NewAccountMeta(acc.FeeProgram, false, false),
		solana.NewAccountMeta(acc.SystemProgram, false, false),
		solana.NewAccountMeta(acc.EventAuthority, false, false),
		solana.NewAccountMeta(acc.Program, false, false),
	}
}

// NewBuyV2Instruction 构造 buy_v2 指令。
//
//	amount       要购买的 base 代币数量（最小单位，6 位小数）
//	maxSolCost   允许花费的 quote 上限 = 预算 × (1 + 滑点)，
//	             预算需已覆盖曲线成本 + 协议费 + creator 费（见 fees.go）。
func NewBuyV2Instruction(amount, maxSolCost uint64, acc TradeAccounts) (solana.Instruction, error) {
	return newBuyV2Instruction(InstructionBuyV2, amount, maxSolCost, acc)
}

// NewBuyExactQuoteInV2Instruction 构造 buy_exact_quote_in_v2 指令。
//
//	spendableQuoteIn 允许花费的 quote 上限（含协议费与 creator 费）
//	minTokensOut     允许接受的最小 base 代币数量
func NewBuyExactQuoteInV2Instruction(spendableQuoteIn, minTokensOut uint64, acc TradeAccounts) (solana.Instruction, error) {
	return newBuyV2Instruction(InstructionBuyExactQuoteInV2, spendableQuoteIn, minTokensOut, acc)
}

func newBuyV2Instruction(disc [8]byte, first, second uint64, acc TradeAccounts) (solana.Instruction, error) {
	if err := acc.Validate(); err != nil {
		return nil, err
	}
	if acc.GlobalVolumeAccumulator.IsZero() {
		return nil, fmt.Errorf("pumpV3: buy 指令需要 GlobalVolumeAccumulator 账户")
	}
	data, err := encodeTradeArgs(disc, first, second)
	if err != nil {
		return nil, err
	}
	return solana.NewInstruction(ProgramID, solana.AccountMetaSlice(buyV2AccountMetas(acc)), data), nil
}

// NewSellV2Instruction 构造 sell_v2 指令。
//
//	amount        要卖出的 base 代币数量（最小单位）
//	minSolOutput  允许收到的 quote 下限 = 扣费后预期产出 × (1 - 滑点)
func NewSellV2Instruction(amount, minSolOutput uint64, acc TradeAccounts) (solana.Instruction, error) {
	if err := acc.Validate(); err != nil {
		return nil, err
	}
	data, err := encodeTradeArgs(InstructionSellV2, amount, minSolOutput)
	if err != nil {
		return nil, err
	}
	return solana.NewInstruction(ProgramID, solana.AccountMetaSlice(sellV2AccountMetas(acc)), data), nil
}
