package raydium

import (
	"crypto/sha256"

	"copytrade/raydium/utils"

	bin "github.com/gagliardetto/binary"
)

// Raydium的指令判别器定义
// 注意: Raydium V4 使用自定义的判别器格式，而不是标准的Anchor格式
// 下面的值是基于实际观察到的数据得出的

// SwapBaseIn指令的判别器 (类型9)
var SwapDiscriminator = uint8(9)

// SwapBaseOut指令的判别器 (类型11)
var SwapDiscriminator_V2 = uint8(11)

// 使用与Anchor类似的方式计算判别器 (仅供参考)
func CalculateAnchorStyleDiscriminator(instructionName string) [8]byte {
	hash := sha256.Sum256([]byte(instructionName))
	var discriminator [8]byte
	copy(discriminator[:], hash[:8])
	return discriminator
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
