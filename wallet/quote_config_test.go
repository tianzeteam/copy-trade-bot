package wallet

import (
	"testing"

	"copytrade/pumpV3"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
)

func TestQuoteAmountConfigForSelectsQuoteAndFallsBack(t *testing.T) {
	scheme := &types.ParamScheme{
		BuySolAmount:           0.5,
		MinFollowTriggerAmount: 0.1,
		QuoteAmounts: map[string]types.QuoteAmountConfig{
			"USDC": {
				BuyAmount:              10,
				MinFollowTriggerAmount: 1,
				MaxFollowTriggerAmount: 100,
				MinPoolSize:            2,
				MaxPoolSize:            200,
			},
		},
	}

	usdc := quoteAmountConfigFor(scheme, pumpV3.USDCMint)
	if usdc.BuyAmount != 10 || usdc.MinFollowTriggerAmount != 1 {
		t.Fatalf("USDC config = %+v", usdc)
	}
	sol := quoteAmountConfigFor(scheme, pumpV3.WSOLMint)
	if sol.BuyAmount != scheme.BuySolAmount || sol.MinFollowTriggerAmount != scheme.MinFollowTriggerAmount {
		t.Fatalf("SOL fallback = %+v", sol)
	}

	unknown := solana.MustPublicKeyFromBase58("11111111111111111111111111111112")
	if got := quoteAmountConfigFor(scheme, unknown); got.BuyAmount != scheme.BuySolAmount {
		t.Fatalf("unknown quote fallback = %+v", got)
	}
}
