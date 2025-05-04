package pumpamm

import (
	"encoding/base64"
	"reflect"
	"unsafe"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"lukechampine.com/uint128"
)

// PumpAmmSwapData represents the structure of a swap transaction in PumpFun
type PUMP_AMM_SWAP_LAYOUT struct {
	Timestamp                        uint64
	BaseAmountIn                     uint128.Uint128
	MinQuoteAmountOut                uint128.Uint128
	UserBaseTokenReserves            uint128.Uint128
	UserQuoteTokenReserves           uint128.Uint128
	PoolBaseTokenReserves            uint128.Uint128
	PoolQuoteTokenReserves           uint128.Uint128
	QuoteAmountOut                   uint128.Uint128
	LpFeeBasisPoints                 uint64
	LpFee                            uint64
	ProtocolFeeBasisPoints           uint64
	ProtocolFee                      uint64
	QuoteAmountOutWithoutLpFee       uint128.Uint128
	UserQuoteAmountOut               uint128.Uint128
	Pool                             solana.PublicKey
	User                             solana.PublicKey
	UserBaseTokenAccount             solana.PublicKey
	UserQuoteTokenAccount            solana.PublicKey
	ProtocolFeeRecipient             solana.PublicKey
	ProtocolFeeRecipientTokenAccount solana.PublicKey
}

// PUMP_AMM_POOL_LAYOUT represents the structure of a PumpAMM pool account
type PUMP_AMM_POOL_LAYOUT struct {
	PoolBump              uint8
	Index                 uint16
	Creator               solana.PublicKey
	BaseMint              solana.PublicKey
	QuoteMint             solana.PublicKey
	LpMint                solana.PublicKey
	PoolBaseTokenAccount  solana.PublicKey
	PoolQuoteTokenAccount solana.PublicKey
	LpSupply              uint64
}

// LIQUIDITY_STATE_LAYOUT_V4 represents the liquidity pool state layout for PumpFun AMM v4
type LIQUIDITY_STATE_LAYOUT_V4 struct {
	Status                 uint64
	Nonce                  uint64
	MaxOrder               uint64
	Depth                  uint64
	BaseDecimal            uint64
	QuoteDecimal           uint64
	State                  uint64
	ResetFlag              uint64
	MinSize                uint64
	VolMaxCutRatio         uint64
	AmountWaveRatio        uint64
	BaseLotSize            uint64
	QuoteLotSize           uint64
	MinPriceMultiplier     uint64
	MaxPriceMultiplier     uint64
	SystemDecimalValue     uint64
	MinSeparateNumerator   uint64
	MinSeparateDenominator uint64
	TradeFeeNumerator      uint64
	TradeFeeDenominator    uint64
	PnlNumerator           uint64
	PnlDenominator         uint64
	SwapFeeNumerator       uint64
	SwapFeeDenominator     uint64
	BaseNeedTakePnl        uint64
	QuoteNeedTakePnl       uint64
	QuoteTotalPnl          uint64
	BaseTotalPnl           uint64
	PoolOpenTime           uint64
	PunishPcAmount         uint64
	PunishCoinAmount       uint64
	OrderbookToInitTime    uint64
	SwapBaseInAmount       uint128.Uint128
	SwapQuoteOutAmount     uint128.Uint128
	SwapBase2QuoteFee      uint64
	SwapQuoteInAmount      uint128.Uint128
	SwapBaseOutAmount      uint128.Uint128
	SwapQuote2BaseFee      uint64
	BaseVault              solana.PublicKey
	QuoteVault             solana.PublicKey
	BaseMint               solana.PublicKey
	QuoteMint              solana.PublicKey
	LpMint                 solana.PublicKey
	OpenOrders             solana.PublicKey
	MarketId               solana.PublicKey
	MarketProgramId        solana.PublicKey
	TargetOrders           solana.PublicKey
	WithdrawQueue          solana.PublicKey
	LpVault                solana.PublicKey
	Owner                  solana.PublicKey
	LpReserve              uint64
	Padding                [3]uint64
}

func (l *PUMP_AMM_POOL_LAYOUT) Span() uint64 {
	return uint64(unsafe.Sizeof(*l))
}

func (l *PUMP_AMM_POOL_LAYOUT) Offset(value string) uint64 {
	fieldType, found := reflect.TypeOf(*l).FieldByName(value)
	if !found {
		return 0
	}
	return uint64(fieldType.Offset)
}

func (l *PUMP_AMM_POOL_LAYOUT) DecodeBase64(data string) error {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return err
	}
	return l.Decode(decoded)
}

func (l *PUMP_AMM_POOL_LAYOUT) Decode(data []byte) error {
	err := bin.UnmarshalBorsh(&l, data)
	return err
}
