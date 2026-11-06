package pumpV3

import (
	"encoding/binary"
	"fmt"
	"math/big"

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
)

// 本文件的解码器全部是手写的，原因有二：
//
//  1. 协议采用「追加字段」的方式演进账户与事件布局。升级前写入的账户数据比新结构短，
//     官方文档明确要求把缺失的尾部字段读作 0 / false / Pubkey::default()。
//     反射式解码遇到数据不足会直接报错，无法表达这种语义。
//  2. TradeEvent 内含变长的 shareholders 向量与 ix_name 字符串，
//     需要精确控制读取顺序，避免把后续字段读错位。
//
// 因此所有尾部可选字段都走 optXxx 系列辅助函数：数据不足时返回零值而非错误。

func decU64(dec *bin.Decoder) (uint64, error) {
	return dec.ReadUint64(binary.LittleEndian)
}

func decI64(dec *bin.Decoder) (int64, error) {
	return dec.ReadInt64(binary.LittleEndian)
}

func decBool(dec *bin.Decoder) (bool, error) {
	return dec.ReadBool()
}

func decU128(dec *bin.Decoder) (bin.Uint128, error) {
	return dec.ReadUint128(binary.LittleEndian)
}

func decPubkey(dec *bin.Decoder) (solana.PublicKey, error) {
	raw, err := dec.ReadBytes(solana.PublicKeyLength)
	if err != nil {
		return solana.PublicKey{}, err
	}
	return solana.PublicKeyFromBytes(raw), nil
}

func decString(dec *bin.Decoder) (string, error) {
	return dec.ReadString()
}

func decPubkeyArray(dec *bin.Decoder, n int) ([]solana.PublicKey, error) {
	out := make([]solana.PublicKey, n)
	for i := 0; i < n; i++ {
		pk, err := decPubkey(dec)
		if err != nil {
			return nil, err
		}
		out[i] = pk
	}
	return out, nil
}

// 以下为尾部可选字段读取：数据耗尽时返回零值。
// 协议在账户/事件末尾追加新字段时，历史数据不含这些字节，必须按缺省值处理。

func optU64(dec *bin.Decoder) (uint64, error) {
	if dec.Remaining() < 8 {
		return 0, nil
	}
	return decU64(dec)
}

func optI64(dec *bin.Decoder) (int64, error) {
	if dec.Remaining() < 8 {
		return 0, nil
	}
	return decI64(dec)
}

func optBool(dec *bin.Decoder) (bool, error) {
	if dec.Remaining() < 1 {
		return false, nil
	}
	return decBool(dec)
}

func optPubkey(dec *bin.Decoder) (solana.PublicKey, error) {
	if dec.Remaining() < solana.PublicKeyLength {
		return solana.PublicKey{}, nil
	}
	return decPubkey(dec)
}

// BondingCurve 是单个代币的内盘曲线账户，PDA seeds = ["bonding-curve", mint]。
//
// 字段顺序即 borsh 布局顺序，不可调整。带「追加」注释的字段是协议升级后新增的，
// 历史账户可能不含这些字节。
//
// 完整布局长度为 125 字节（不含 8 字节账户判别码）；
// 官方文档提到的 115 字节是 quote_mint 刚追加时的长度，
// 之后又追加了 creator_fee_bps / can_edit_creator_fee / is_holder_reward。
type BondingCurve struct {
	VirtualTokenReserves uint64
	VirtualQuoteReserves uint64 // 旧名 virtual_sol_reserves
	RealTokenReserves    uint64
	RealQuoteReserves    uint64 // 旧名 real_sol_reserves
	TokenTotalSupply     uint64
	Complete             bool

	Creator solana.PublicKey // 追加：creator fee 归属地址

	IsMayhemMode      bool             // 追加：mayhem 模式币必须使用 ReservedFeeRecipients
	IsCashbackCoin    bool             // 追加：cashback 已废弃，仅存量币为 true
	QuoteMint         solana.PublicKey // 追加：零值表示 SOL 配对，否则为 USDC 等
	CreatorFeeBps     uint64           // 追加：自定义交易对的 creator 费率，0 表示走标准费率
	CanEditCreatorFee bool             // 追加：保留字段，恒为 false
	IsHolderReward    bool             // 追加：creator fee 归持有人而非创建者钱包
}

// UnmarshalWithDecoder 解码 BondingCurve，容忍历史短账户。
func (obj *BondingCurve) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.VirtualTokenReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("BondingCurve.VirtualTokenReserves: %w", err)
	}
	if obj.VirtualQuoteReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("BondingCurve.VirtualQuoteReserves: %w", err)
	}
	if obj.RealTokenReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("BondingCurve.RealTokenReserves: %w", err)
	}
	if obj.RealQuoteReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("BondingCurve.RealQuoteReserves: %w", err)
	}
	if obj.TokenTotalSupply, err = decU64(dec); err != nil {
		return fmt.Errorf("BondingCurve.TokenTotalSupply: %w", err)
	}
	if obj.Complete, err = decBool(dec); err != nil {
		return fmt.Errorf("BondingCurve.Complete: %w", err)
	}
	if obj.Creator, err = optPubkey(dec); err != nil {
		return fmt.Errorf("BondingCurve.Creator: %w", err)
	}
	if obj.IsMayhemMode, err = optBool(dec); err != nil {
		return fmt.Errorf("BondingCurve.IsMayhemMode: %w", err)
	}
	if obj.IsCashbackCoin, err = optBool(dec); err != nil {
		return fmt.Errorf("BondingCurve.IsCashbackCoin: %w", err)
	}
	if obj.QuoteMint, err = optPubkey(dec); err != nil {
		return fmt.Errorf("BondingCurve.QuoteMint: %w", err)
	}
	if obj.CreatorFeeBps, err = optU64(dec); err != nil {
		return fmt.Errorf("BondingCurve.CreatorFeeBps: %w", err)
	}
	if obj.CanEditCreatorFee, err = optBool(dec); err != nil {
		return fmt.Errorf("BondingCurve.CanEditCreatorFee: %w", err)
	}
	if obj.IsHolderReward, err = optBool(dec); err != nil {
		return fmt.Errorf("BondingCurve.IsHolderReward: %w", err)
	}
	return nil
}

// MarshalWithEncoder 按 borsh 布局序列化（不含账户判别码）。
func (obj BondingCurve) MarshalWithEncoder(enc *bin.Encoder) error {
	for _, v := range []uint64{obj.VirtualTokenReserves, obj.VirtualQuoteReserves, obj.RealTokenReserves, obj.RealQuoteReserves, obj.TokenTotalSupply} {
		if err := enc.WriteUint64(v, binary.LittleEndian); err != nil {
			return err
		}
	}
	if err := enc.WriteBool(obj.Complete); err != nil {
		return err
	}
	if err := enc.WriteBytes(obj.Creator[:], false); err != nil {
		return err
	}
	if err := enc.WriteBool(obj.IsMayhemMode); err != nil {
		return err
	}
	if err := enc.WriteBool(obj.IsCashbackCoin); err != nil {
		return err
	}
	if err := enc.WriteBytes(obj.QuoteMint[:], false); err != nil {
		return err
	}
	if err := enc.WriteUint64(obj.CreatorFeeBps, binary.LittleEndian); err != nil {
		return err
	}
	if err := enc.WriteBool(obj.CanEditCreatorFee); err != nil {
		return err
	}
	return enc.WriteBool(obj.IsHolderReward)
}

// IsSolPaired 返回该曲线是否以 SOL 计价。
func (obj BondingCurve) IsSolPaired() bool {
	return IsSolPaired(obj.QuoteMint)
}

// QuoteDecimals 返回计价资产小数位（SOL=9，USDC=6）。
func (obj BondingCurve) QuoteDecimals() uint32 {
	return QuoteDecimals(obj.QuoteMint)
}

// MarketCapLamports 按官方公式计算以 lamports 计的市值，用于查询动态费率档位。
//
//	marketCap = virtualQuoteReserves * tokenTotalSupply / virtualTokenReserves
//
// 注意：该公式对 SOL 配对币成立。USDC 配对币算出的是「USDC 最小单位计的市值」，
// 与官方 computeFeesBps 的行为一致（官方也直接用 quote 储备计算），无需换算。
func (obj BondingCurve) MarketCapLamports() (bin.Uint128, error) {
	if obj.VirtualTokenReserves == 0 {
		return bin.Uint128{}, fmt.Errorf("virtual token reserves 为 0，无法计算市值")
	}
	mc := new(big.Int).SetUint64(obj.VirtualQuoteReserves)
	mc.Mul(mc, new(big.Int).SetUint64(obj.TokenTotalSupply))
	mc.Quo(mc, new(big.Int).SetUint64(obj.VirtualTokenReserves))
	out, err := uint128FromBig(mc)
	if err != nil {
		return bin.Uint128{}, err
	}
	return out, nil
}

// Price 返回以 quote 资产计价的单个代币价格（已按 quote 与 base 小数位归一化）。
//
// base 代币固定 6 位小数；quote 由 QuoteDecimals 决定。
// 返回值单位为「1 个代币 = ? 个 quote 资产」，可直接用于止盈止损百分比计算，
// 因为百分比对单位不敏感。绝对金额换算必须调用方自行按 quote 小数位处理。
func (obj BondingCurve) Price() float64 {
	if obj.VirtualTokenReserves == 0 {
		return 0
	}
	quote := float64(obj.VirtualQuoteReserves) / pow10(uint32(obj.QuoteDecimals()))
	token := float64(obj.VirtualTokenReserves) / 1e6
	if token == 0 {
		return 0
	}
	return quote / token
}

// Shareholder 是 creator fee 分账中的单个受益人。
type Shareholder struct {
	Address  solana.PublicKey
	ShareBps uint16
}

// UnmarshalWithDecoder 解码 Shareholder。
func (obj *Shareholder) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.Address, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.ShareBps, err = dec.ReadUint16(binary.LittleEndian); err != nil {
		return err
	}
	return nil
}

// TradeEvent 是内盘买卖产生的事件，buy / sell / buy_exact_sol_in /
// buy_v2 / sell_v2 / buy_exact_quote_in_v2 六种指令共用同一个事件判别码。
//
// 因此跟单系统只要正确解码本结构，就能同时覆盖新旧指令产生的交易。
//
// 字段顺序即 borsh 布局顺序。带「追加」注释的字段为协议升级后新增，
// 旧交易不含这些字节，解码时按零值处理。
type TradeEvent struct {
	Mint                  solana.PublicKey
	SolAmount             uint64 // 实际为 quote 资产数量，USDC 配对币时单位是 USDC 最小单位
	TokenAmount           uint64
	IsBuy                 bool
	User                  solana.PublicKey
	Timestamp             int64
	VirtualSolReserves    uint64 // 旧字段，与 VirtualQuoteReserves 等值（SOL 配对币）
	VirtualTokenReserves  uint64
	RealSolReserves       uint64 // 旧字段，与 RealQuoteReserves 等值
	RealTokenReserves     uint64
	FeeRecipient          solana.PublicKey
	FeeBasisPoints        uint64
	Fee                   uint64
	Creator               solana.PublicKey
	CreatorFeeBasisPoints uint64
	CreatorFee            uint64

	TrackVolume          bool   // 追加
	TotalUnclaimedTokens uint64 // 追加
	TotalClaimedTokens   uint64 // 追加
	CurrentSolVolume     uint64 // 追加
	LastUpdateTimestamp  int64  // 追加
	IxName               string // 追加：产生本事件的指令名，如 "buy_v2"

	MayhemMode             bool          // 追加
	CashbackFeeBasisPoints uint64        // 追加
	Cashback               uint64        // 追加
	BuybackFeeBasisPoints  uint64        // 追加
	BuybackFee             uint64        // 追加
	Shareholders           []Shareholder // 追加：变长，解码时必须严格按 u32 长度前缀读取

	QuoteMint            solana.PublicKey // 追加：零值表示 SOL 配对
	QuoteAmount          uint64           // 追加：以 quote 资产计的交易金额
	VirtualQuoteReserves uint64           // 追加
	RealQuoteReserves    uint64           // 追加

	HolderRewardsBps uint64 // 追加
	HolderRewards    uint64 // 追加
}

// UnmarshalWithDecoder 解码 TradeEvent，容忍旧版事件缺少尾部字段。
func (obj *TradeEvent) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.Mint, err = decPubkey(dec); err != nil {
		return fmt.Errorf("TradeEvent.Mint: %w", err)
	}
	if obj.SolAmount, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.SolAmount: %w", err)
	}
	if obj.TokenAmount, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.TokenAmount: %w", err)
	}
	if obj.IsBuy, err = decBool(dec); err != nil {
		return fmt.Errorf("TradeEvent.IsBuy: %w", err)
	}
	if obj.User, err = decPubkey(dec); err != nil {
		return fmt.Errorf("TradeEvent.User: %w", err)
	}
	if obj.Timestamp, err = decI64(dec); err != nil {
		return fmt.Errorf("TradeEvent.Timestamp: %w", err)
	}
	if obj.VirtualSolReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.VirtualSolReserves: %w", err)
	}
	if obj.VirtualTokenReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.VirtualTokenReserves: %w", err)
	}
	if obj.RealSolReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.RealSolReserves: %w", err)
	}
	if obj.RealTokenReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.RealTokenReserves: %w", err)
	}
	if obj.FeeRecipient, err = decPubkey(dec); err != nil {
		return fmt.Errorf("TradeEvent.FeeRecipient: %w", err)
	}
	if obj.FeeBasisPoints, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.FeeBasisPoints: %w", err)
	}
	if obj.Fee, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.Fee: %w", err)
	}
	if obj.Creator, err = decPubkey(dec); err != nil {
		return fmt.Errorf("TradeEvent.Creator: %w", err)
	}
	if obj.CreatorFeeBasisPoints, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.CreatorFeeBasisPoints: %w", err)
	}
	if obj.CreatorFee, err = decU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.CreatorFee: %w", err)
	}

	// ---- 以下为协议升级后追加的字段，旧事件不含这些字节 ----
	if obj.TrackVolume, err = optBool(dec); err != nil {
		return fmt.Errorf("TradeEvent.TrackVolume: %w", err)
	}
	if obj.TotalUnclaimedTokens, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.TotalUnclaimedTokens: %w", err)
	}
	if obj.TotalClaimedTokens, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.TotalClaimedTokens: %w", err)
	}
	if obj.CurrentSolVolume, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.CurrentSolVolume: %w", err)
	}
	if obj.LastUpdateTimestamp, err = optI64(dec); err != nil {
		return fmt.Errorf("TradeEvent.LastUpdateTimestamp: %w", err)
	}
	if dec.Remaining() >= 4 {
		if obj.IxName, err = decString(dec); err != nil {
			return fmt.Errorf("TradeEvent.IxName: %w", err)
		}
	}
	if obj.MayhemMode, err = optBool(dec); err != nil {
		return fmt.Errorf("TradeEvent.MayhemMode: %w", err)
	}
	if obj.CashbackFeeBasisPoints, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.CashbackFeeBasisPoints: %w", err)
	}
	if obj.Cashback, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.Cashback: %w", err)
	}
	if obj.BuybackFeeBasisPoints, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.BuybackFeeBasisPoints: %w", err)
	}
	if obj.BuybackFee, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.BuybackFee: %w", err)
	}
	// shareholders 是变长向量，必须显式读长度前缀；数据不足则视为空。
	if dec.Remaining() >= 4 {
		count, lenErr := dec.ReadLength()
		if lenErr != nil {
			return fmt.Errorf("TradeEvent.Shareholders length: %w", lenErr)
		}
		if count > 0 {
			obj.Shareholders = make([]Shareholder, count)
			for i := 0; i < count; i++ {
				if err = obj.Shareholders[i].UnmarshalWithDecoder(dec); err != nil {
					return fmt.Errorf("TradeEvent.Shareholders[%d]: %w", i, err)
				}
			}
		}
	}
	if obj.QuoteMint, err = optPubkey(dec); err != nil {
		return fmt.Errorf("TradeEvent.QuoteMint: %w", err)
	}
	if obj.QuoteAmount, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.QuoteAmount: %w", err)
	}
	if obj.VirtualQuoteReserves, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.VirtualQuoteReserves: %w", err)
	}
	if obj.RealQuoteReserves, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.RealQuoteReserves: %w", err)
	}
	if obj.HolderRewardsBps, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.HolderRewardsBps: %w", err)
	}
	if obj.HolderRewards, err = optU64(dec); err != nil {
		return fmt.Errorf("TradeEvent.HolderRewards: %w", err)
	}
	return nil
}

// EffectiveQuoteMint 返回本事件的计价 mint。
// 旧事件的 QuoteMint 为零值，此时按 SOL 处理。
func (obj TradeEvent) EffectiveQuoteMint() solana.PublicKey {
	if obj.QuoteMint.IsZero() {
		return WSOLMint
	}
	return obj.QuoteMint
}

// QuoteDecimals 返回计价资产小数位。
func (obj TradeEvent) QuoteDecimals() uint32 {
	return QuoteDecimals(obj.QuoteMint)
}

// IsSolPaired 返回该笔交易是否发生在 SOL 配对币上。
func (obj TradeEvent) IsSolPaired() bool {
	return IsSolPaired(obj.QuoteMint)
}

// EffectiveVirtualQuoteReserves 返回虚拟 quote 储备。
// 旧事件只有 VirtualSolReserves，新事件两者等值，优先取新字段。
func (obj TradeEvent) EffectiveVirtualQuoteReserves() uint64 {
	if obj.VirtualQuoteReserves != 0 {
		return obj.VirtualQuoteReserves
	}
	return obj.VirtualSolReserves
}

// EffectiveRealQuoteReserves 返回真实 quote 储备（即池子里实际可用的 quote 数量）。
func (obj TradeEvent) EffectiveRealQuoteReserves() uint64 {
	if obj.RealQuoteReserves != 0 {
		return obj.RealQuoteReserves
	}
	return obj.RealSolReserves
}

// EffectiveQuoteAmount 返回以 quote 资产计的交易金额。
func (obj TradeEvent) EffectiveQuoteAmount() uint64 {
	if obj.QuoteAmount != 0 {
		return obj.QuoteAmount
	}
	return obj.SolAmount
}

// CurrentPrice 返回交易后的代币价格（1 代币 = ? quote 资产）。
func (obj TradeEvent) CurrentPrice() float64 {
	if obj.VirtualTokenReserves == 0 {
		return 0
	}
	quote := float64(obj.EffectiveVirtualQuoteReserves()) / pow10(obj.QuoteDecimals())
	token := float64(obj.VirtualTokenReserves) / 1e6
	if token == 0 {
		return 0
	}
	return quote / token
}

// CreateEvent 是代币创建事件。
type CreateEvent struct {
	Name                 string
	Symbol               string
	URI                  string
	Mint                 solana.PublicKey
	BondingCurve         solana.PublicKey
	User                 solana.PublicKey
	Creator              solana.PublicKey
	Timestamp            int64
	VirtualTokenReserves uint64
	VirtualSolReserves   uint64
	RealTokenReserves    uint64
	TokenTotalSupply     uint64
	TokenProgram         solana.PublicKey

	IsMayhemMode         bool             // 追加
	IsCashbackEnabled    bool             // 追加
	QuoteMint            solana.PublicKey // 追加：零值表示 SOL 配对
	VirtualQuoteReserves uint64           // 追加
	CreatorFeeBps        uint64           // 追加
	IsHolderReward       bool             // 追加
}

// UnmarshalWithDecoder 解码 CreateEvent，容忍旧版事件缺少尾部字段。
func (obj *CreateEvent) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.Name, err = decString(dec); err != nil {
		return fmt.Errorf("CreateEvent.Name: %w", err)
	}
	if obj.Symbol, err = decString(dec); err != nil {
		return fmt.Errorf("CreateEvent.Symbol: %w", err)
	}
	if obj.URI, err = decString(dec); err != nil {
		return fmt.Errorf("CreateEvent.URI: %w", err)
	}
	if obj.Mint, err = decPubkey(dec); err != nil {
		return fmt.Errorf("CreateEvent.Mint: %w", err)
	}
	if obj.BondingCurve, err = decPubkey(dec); err != nil {
		return fmt.Errorf("CreateEvent.BondingCurve: %w", err)
	}
	if obj.User, err = decPubkey(dec); err != nil {
		return fmt.Errorf("CreateEvent.User: %w", err)
	}
	if obj.Creator, err = decPubkey(dec); err != nil {
		return fmt.Errorf("CreateEvent.Creator: %w", err)
	}
	if obj.Timestamp, err = decI64(dec); err != nil {
		return fmt.Errorf("CreateEvent.Timestamp: %w", err)
	}
	if obj.VirtualTokenReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("CreateEvent.VirtualTokenReserves: %w", err)
	}
	if obj.VirtualSolReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("CreateEvent.VirtualSolReserves: %w", err)
	}
	if obj.RealTokenReserves, err = decU64(dec); err != nil {
		return fmt.Errorf("CreateEvent.RealTokenReserves: %w", err)
	}
	if obj.TokenTotalSupply, err = decU64(dec); err != nil {
		return fmt.Errorf("CreateEvent.TokenTotalSupply: %w", err)
	}
	if obj.TokenProgram, err = decPubkey(dec); err != nil {
		return fmt.Errorf("CreateEvent.TokenProgram: %w", err)
	}
	if obj.IsMayhemMode, err = optBool(dec); err != nil {
		return fmt.Errorf("CreateEvent.IsMayhemMode: %w", err)
	}
	if obj.IsCashbackEnabled, err = optBool(dec); err != nil {
		return fmt.Errorf("CreateEvent.IsCashbackEnabled: %w", err)
	}
	if obj.QuoteMint, err = optPubkey(dec); err != nil {
		return fmt.Errorf("CreateEvent.QuoteMint: %w", err)
	}
	if obj.VirtualQuoteReserves, err = optU64(dec); err != nil {
		return fmt.Errorf("CreateEvent.VirtualQuoteReserves: %w", err)
	}
	if obj.CreatorFeeBps, err = optU64(dec); err != nil {
		return fmt.Errorf("CreateEvent.CreatorFeeBps: %w", err)
	}
	if obj.IsHolderReward, err = optBool(dec); err != nil {
		return fmt.Errorf("CreateEvent.IsHolderReward: %w", err)
	}
	return nil
}

// CompleteEvent 是内盘打满（毕业）事件。
type CompleteEvent struct {
	User         solana.PublicKey
	Mint         solana.PublicKey
	BondingCurve solana.PublicKey
	Timestamp    int64
	QuoteMint    solana.PublicKey // 追加
}

// UnmarshalWithDecoder 解码 CompleteEvent。
func (obj *CompleteEvent) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.User, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.Mint, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.BondingCurve, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.Timestamp, err = decI64(dec); err != nil {
		return err
	}
	if obj.QuoteMint, err = optPubkey(dec); err != nil {
		return err
	}
	return nil
}

// CompletePumpAmmMigrationEvent 是内盘流动性迁移到 PumpSwap 完成事件。
type CompletePumpAmmMigrationEvent struct {
	User             solana.PublicKey
	Mint             solana.PublicKey
	MintAmount       uint64
	SolAmount        uint64
	PoolMigrationFee uint64
	BondingCurve     solana.PublicKey
	Timestamp        int64
	Pool             solana.PublicKey
	QuoteMint        solana.PublicKey // 追加
}

// UnmarshalWithDecoder 解码迁移完成事件。
func (obj *CompletePumpAmmMigrationEvent) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.User, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.Mint, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.MintAmount, err = decU64(dec); err != nil {
		return err
	}
	if obj.SolAmount, err = decU64(dec); err != nil {
		return err
	}
	if obj.PoolMigrationFee, err = decU64(dec); err != nil {
		return err
	}
	if obj.BondingCurve, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.Timestamp, err = decI64(dec); err != nil {
		return err
	}
	if obj.Pool, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.QuoteMint, err = optPubkey(dec); err != nil {
		return err
	}
	return nil
}

// Fees 是某一市值档位下的三项费率（单位 bps）。
type Fees struct {
	LpFeeBps       uint64
	ProtocolFeeBps uint64
	CreatorFeeBps  uint64
}

// TotalBps 返回协议费与创建者费之和，不含 LP 费（LP 费只在 PumpSwap 收取）。
func (f Fees) TotalBps() uint64 {
	return f.ProtocolFeeBps + f.CreatorFeeBps
}

// UnmarshalWithDecoder 解码 Fees。
func (obj *Fees) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.LpFeeBps, err = decU64(dec); err != nil {
		return err
	}
	if obj.ProtocolFeeBps, err = decU64(dec); err != nil {
		return err
	}
	if obj.CreatorFeeBps, err = decU64(dec); err != nil {
		return err
	}
	return nil
}

// MarshalWithEncoder 序列化 Fees。
func (obj Fees) MarshalWithEncoder(enc *bin.Encoder) error {
	for _, v := range []uint64{obj.LpFeeBps, obj.ProtocolFeeBps, obj.CreatorFeeBps} {
		if err := enc.WriteUint64(v, binary.LittleEndian); err != nil {
			return err
		}
	}
	return nil
}

// FeeTier 是单个市值档位：市值（lamports 计）达到阈值即适用该档费率。
type FeeTier struct {
	MarketCapLamportsThreshold bin.Uint128
	Fees                       Fees
}

// UnmarshalWithDecoder 解码 FeeTier。
func (obj *FeeTier) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.MarketCapLamportsThreshold, err = decU128(dec); err != nil {
		return err
	}
	return obj.Fees.UnmarshalWithDecoder(dec)
}

// FeeConfig 是 pump_fees 程序的费率配置账户，
// PDA seeds = ["fee_config", pump_program_id]，program = pump_fees。
type FeeConfig struct {
	Bump           uint8
	Admin          solana.PublicKey
	FlatFees       Fees
	FeeTiers       []FeeTier
	StableFeeTiers []FeeTier
	ExoticFlatFees Fees
}

// UnmarshalWithDecoder 解码 FeeConfig。
func (obj *FeeConfig) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.Bump, err = dec.ReadUint8(); err != nil {
		return err
	}
	if obj.Admin, err = decPubkey(dec); err != nil {
		return err
	}
	if err = obj.FlatFees.UnmarshalWithDecoder(dec); err != nil {
		return err
	}
	if obj.FeeTiers, err = decFeeTierVec(dec); err != nil {
		return err
	}
	if obj.StableFeeTiers, err = decFeeTierVec(dec); err != nil {
		return err
	}
	return obj.ExoticFlatFees.UnmarshalWithDecoder(dec)
}

func decFeeTierVec(dec *bin.Decoder) ([]FeeTier, error) {
	count, err := dec.ReadLength()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}
	// 防御性上限：费率档位不可能成千上万，异常长度说明数据错位。
	if count > 4096 {
		return nil, fmt.Errorf("fee tier 数量异常: %d", count)
	}
	out := make([]FeeTier, count)
	for i := 0; i < count; i++ {
		if err = out[i].UnmarshalWithDecoder(dec); err != nil {
			return nil, fmt.Errorf("FeeTier[%d]: %w", i, err)
		}
	}
	return out, nil
}

// Global 是 Pump.fun 内盘的全局配置账户，seeds = ["global"]。
//
// 字段按 borsh 布局顺序排列，带「追加」注释的为升级后新增。
type Global struct {
	Initialized                 bool
	Authority                   solana.PublicKey
	FeeRecipient                solana.PublicKey
	InitialVirtualTokenReserves uint64
	InitialVirtualSolReserves   uint64
	InitialRealTokenReserves    uint64
	TokenTotalSupply            uint64
	FeeBasisPoints              uint64
	WithdrawAuthority           solana.PublicKey
	EnableMigrate               bool
	PoolMigrationFee            uint64
	CreatorFeeBasisPoints       uint64
	FeeRecipients               [7]solana.PublicKey

	SetCreatorAuthority          solana.PublicKey    // 追加
	AdminSetCreatorAuthority     solana.PublicKey    // 追加
	CreateV2Enabled              bool                // 追加
	WhitelistPda                 solana.PublicKey    // 追加
	ReservedFeeRecipient         solana.PublicKey    // 追加
	MayhemModeEnabled            bool                // 追加
	ReservedFeeRecipients        [7]solana.PublicKey // 追加
	IsCashbackEnabled            bool                // 追加
	BuybackFeeRecipients         [8]solana.PublicKey // 追加
	BuybackBasisPoints           uint64              // 追加
	InitialVirtualQuoteReserves  uint64              // 追加：非 SOL 配对币的初始虚拟 quote 储备
	WhitelistedQuoteMints        [1]solana.PublicKey // 追加
	CreatorFeeConfigurable       bool                // 追加
	MaxConfigurableCreatorFeeBps uint64              // 追加
	HolderRewardClaimAuthority   solana.PublicKey    // 追加
	IsHolderRewardEnabled        bool                // 追加
}

// UnmarshalWithDecoder 解码 Global，容忍历史短账户。
func (obj *Global) UnmarshalWithDecoder(dec *bin.Decoder) error {
	var err error
	if obj.Initialized, err = decBool(dec); err != nil {
		return err
	}
	if obj.Authority, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.FeeRecipient, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.InitialVirtualTokenReserves, err = decU64(dec); err != nil {
		return err
	}
	if obj.InitialVirtualSolReserves, err = decU64(dec); err != nil {
		return err
	}
	if obj.InitialRealTokenReserves, err = decU64(dec); err != nil {
		return err
	}
	if obj.TokenTotalSupply, err = decU64(dec); err != nil {
		return err
	}
	if obj.FeeBasisPoints, err = decU64(dec); err != nil {
		return err
	}
	if obj.WithdrawAuthority, err = decPubkey(dec); err != nil {
		return err
	}
	if obj.EnableMigrate, err = decBool(dec); err != nil {
		return err
	}
	if obj.PoolMigrationFee, err = decU64(dec); err != nil {
		return err
	}
	if obj.CreatorFeeBasisPoints, err = decU64(dec); err != nil {
		return err
	}
	if err = readPubkeyArrayInto(dec, obj.FeeRecipients[:]); err != nil {
		return err
	}

	// ---- 追加字段 ----
	if obj.SetCreatorAuthority, err = optPubkey(dec); err != nil {
		return err
	}
	if obj.AdminSetCreatorAuthority, err = optPubkey(dec); err != nil {
		return err
	}
	if obj.CreateV2Enabled, err = optBool(dec); err != nil {
		return err
	}
	if obj.WhitelistPda, err = optPubkey(dec); err != nil {
		return err
	}
	if obj.ReservedFeeRecipient, err = optPubkey(dec); err != nil {
		return err
	}
	if obj.MayhemModeEnabled, err = optBool(dec); err != nil {
		return err
	}
	if dec.Remaining() >= 7*solana.PublicKeyLength {
		if err = readPubkeyArrayInto(dec, obj.ReservedFeeRecipients[:]); err != nil {
			return err
		}
	}
	if obj.IsCashbackEnabled, err = optBool(dec); err != nil {
		return err
	}
	if dec.Remaining() >= 8*solana.PublicKeyLength {
		if err = readPubkeyArrayInto(dec, obj.BuybackFeeRecipients[:]); err != nil {
			return err
		}
	}
	if obj.BuybackBasisPoints, err = optU64(dec); err != nil {
		return err
	}
	if obj.InitialVirtualQuoteReserves, err = optU64(dec); err != nil {
		return err
	}
	if dec.Remaining() >= solana.PublicKeyLength {
		if err = readPubkeyArrayInto(dec, obj.WhitelistedQuoteMints[:]); err != nil {
			return err
		}
	}
	if obj.CreatorFeeConfigurable, err = optBool(dec); err != nil {
		return err
	}
	if obj.MaxConfigurableCreatorFeeBps, err = optU64(dec); err != nil {
		return err
	}
	if obj.HolderRewardClaimAuthority, err = optPubkey(dec); err != nil {
		return err
	}
	if obj.IsHolderRewardEnabled, err = optBool(dec); err != nil {
		return err
	}
	return nil
}

// AllFeeRecipients 返回该 Global 认可的全部普通 fee recipient（首位 + 7 位数组）。
func (obj Global) AllFeeRecipients() []solana.PublicKey {
	out := make([]solana.PublicKey, 0, 8)
	if !obj.FeeRecipient.IsZero() {
		out = append(out, obj.FeeRecipient)
	}
	for _, pk := range obj.FeeRecipients {
		if !pk.IsZero() {
			out = append(out, pk)
		}
	}
	return out
}

// AllBuybackFeeRecipients 返回全部 buyback fee recipient。
func (obj Global) AllBuybackFeeRecipients() []solana.PublicKey {
	out := make([]solana.PublicKey, 0, 8)
	for _, pk := range obj.BuybackFeeRecipients {
		if !pk.IsZero() {
			out = append(out, pk)
		}
	}
	return out
}

func readPubkeyArrayInto(dec *bin.Decoder, dst []solana.PublicKey) error {
	for i := range dst {
		pk, err := decPubkey(dec)
		if err != nil {
			return err
		}
		dst[i] = pk
	}
	return nil
}

// uint128FromBig 把 big.Int 收敛为 bin.Uint128。
// 不设置 Endianness 字段，与 bin.Decoder.ReadUint128 的产出保持一致，
// 这样解码出来的值与计算出来的值可以直接比较（Uint128 内含 interface 字段，
// 一旦一侧写入 binary.LittleEndian 这类函数值，== 比较会 panic）。
func uint128FromBig(v *big.Int) (bin.Uint128, error) {
	if v == nil {
		return bin.Uint128{}, fmt.Errorf("uint128: nil big.Int")
	}
	if v.Sign() < 0 {
		return bin.Uint128{}, fmt.Errorf("uint128: 不接受负数 %s", v.String())
	}
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	if v.Cmp(limit) >= 0 {
		return bin.Uint128{}, fmt.Errorf("uint128: 溢出 %s", v.String())
	}
	mask := new(big.Int).SetUint64(^uint64(0))
	lo := new(big.Int).And(v, mask).Uint64()
	hi := new(big.Int).Rsh(v, 64).Uint64()
	return bin.Uint128{Lo: lo, Hi: hi}, nil
}

func pow10(n uint32) float64 {
	out := 1.0
	for i := uint32(0); i < n; i++ {
		out *= 10
	}
	return out
}
