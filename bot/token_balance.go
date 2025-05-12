package bot

import (
	"copytrade/core"
	"copytrade/logger"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"
	"strconv"
)

type TokenBalanceChange struct {
	Owner    string // Token owner address
	Mint     string // Token mint address
	Amount   uint64 // Amount of change
	Decimals uint32
	IsBuy    bool // True if tokens were sold (balance decreased), false if bought (balance increased)
}

func CheckTokenBalances(preTokenBalances, postTokenBalances []*yellowstone_geyser_pb.TokenBalance) *TokenBalanceChange {
	if len(postTokenBalances) == 0 {
		return nil
	}

	for _, postBalance := range postTokenBalances {
		if postBalance.Mint == core.WSOL.String() {
			continue
		}

		if postBalance.Owner == core.RAYDIUM_AUTHORITY_v4 {
			continue
		}
		var preAmount uint64 = 0
		for _, preBalance := range preTokenBalances {
			if preBalance.Owner == postBalance.Owner && preBalance.Mint == postBalance.Mint {
				parsedAmount, err := strconv.ParseUint(preBalance.UiTokenAmount.Amount, 10, 64)
				if err != nil {
					logger.Warnf("Failed to parse token amount: %v", err)
					continue
				}
				preAmount = parsedAmount
				break
			}
		}
		// Get decimals directly as it's already a uint32
		decimals := postBalance.UiTokenAmount.Decimals

		postAmount, _ := strconv.ParseUint(postBalance.UiTokenAmount.Amount, 10, 64)

		// Calculate change, handling the case where post < pre (which would cause overflow in uint64)
		var change uint64
		var isBuy bool

		if postAmount >= preAmount {
			change = postAmount - preAmount
			isBuy = true
		} else {
			change = preAmount - postAmount
			isBuy = false
		}

		// If there's a change, return the token information and change amount
		if change != 0 {
			return &TokenBalanceChange{
				Owner:    postBalance.Owner,
				Mint:     postBalance.Mint,
				Amount:   change,
				Decimals: decimals,
				IsBuy:    isBuy,
			}
		}
	}

	return nil
}
