package raydium

import (
	"bytes"
	"fmt"

	"copytrade/raydium/layouts"
	"copytrade/raydium/utils"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type SwapV4Instruction struct {
	Instruction      uint8
	AmountIn         uint64
	MinimumOutAmount uint64
}

func (inst *SwapV4Instruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBorshEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

func NewSwapV4Instruction(

	poolKeys *layouts.ApiPoolInfoV4,
	amountIn uint64,
	minAmountOut uint64,
	tokenAccountIn solana.PublicKey,
	tokenAccountOut solana.PublicKey,
	signer solana.PublicKey) (solana.Instruction, error) {
	inst := &SwapV4Instruction{
		Instruction:      9,
		AmountIn:         amountIn,
		MinimumOutAmount: minAmountOut,
	}
	keys := solana.AccountMetaSlice{}

	keys.Append(solana.Meta(solana.TokenProgramID))
	keys.Append(solana.Meta(poolKeys.ID).WRITE())
	keys.Append(solana.Meta(poolKeys.Authority))
	keys.Append(solana.Meta(poolKeys.OpenOrders).WRITE())

	if poolKeys.Version == 4 {
		keys.Append(solana.Meta(poolKeys.TargetOrders).WRITE())
	}

	keys.Append(solana.Meta(poolKeys.BaseVault).WRITE())
	keys.Append(solana.Meta(poolKeys.QuoteVault).WRITE())

	if poolKeys.Version == 5 {
		keys.Append(solana.Meta(ModelDataPubkey).WRITE())
	}
	keys.Append(solana.Meta(poolKeys.MarketProgramId))
	keys.Append(solana.Meta(poolKeys.MarketId).WRITE())
	keys.Append(solana.Meta(poolKeys.MarketBids).WRITE())
	keys.Append(solana.Meta(poolKeys.MarketAsks).WRITE())
	keys.Append(solana.Meta(poolKeys.MarketEventQueue).WRITE())
	keys.Append(solana.Meta(poolKeys.BaseVault).WRITE())
	keys.Append(solana.Meta(poolKeys.QuoteVault).WRITE())
	keys.Append(solana.Meta(poolKeys.MarketAuthority))
	keys.Append(solana.Meta(tokenAccountIn).WRITE())
	keys.Append(solana.Meta(tokenAccountOut).WRITE())
	keys.Append(solana.Meta(signer).SIGNER())

	data, err := inst.Data()
	if err != nil {
		return nil, err
	}
	intstr := solana.NewInstruction(
		RAYDIUM_V4_PROGRAM_ID,
		keys,
		data,
	)

	return intstr, nil
}

type SimulateStruct struct {
	Instruction  uint8
	SimulateType uint8
}

func (l *SimulateStruct) Encode() ([]byte, error) {
	return bin.MarshalBin(&l)
}

type LiquidityPoolInfo struct {
	Status        int    `json:"status"`
	BaseDecimals  uint64 `json:"coin_decimals"`
	QuoteDecimals uint64 `json:"pc_decimals"`
	LpDecimals    uint64 `json:"lp_decimals"`
	QuoteReserve  uint64 `json:"pool_pc_amount"`
	BaseReserve   uint64 `json:"pool_coin_amount"`
	LpSupply      int    `json:"pool_lp_supply"`
	PoolOpenTime  int64  `json:"pool_open_time"`
	AmmID         string `json:"amm_id"`
}

type AmountsOut struct {
	AmountIn     *utils.TokenAmount
	AmountOut    *utils.TokenAmount
	MinAmountOut *utils.TokenAmount
}
