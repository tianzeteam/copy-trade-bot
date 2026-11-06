package wallet

import (
	"strings"

	"copytrade/pumpV3"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
)

func quoteAmountConfigFor(scheme *types.ParamScheme, quoteMint solana.PublicKey) types.QuoteAmountConfig {
	if scheme == nil {
		return types.QuoteAmountConfig{}
	}
	fallback := types.QuoteAmountConfig{
		BuyAmount:              scheme.BuySolAmount,
		MinFollowTriggerAmount: scheme.MinFollowTriggerAmount,
		MaxFollowTriggerAmount: scheme.MaxFollowTriggerAmount,
		MinPoolSize:            scheme.MinPoolSize,
		MaxPoolSize:            scheme.MaxPoolSize,
	}
	if len(scheme.QuoteAmounts) == 0 {
		return fallback
	}

	key := quoteConfigKey(quoteMint)
	if config, ok := scheme.QuoteAmounts[key]; ok {
		return config
	}
	for configuredKey, config := range scheme.QuoteAmounts {
		if strings.EqualFold(configuredKey, key) || strings.EqualFold(configuredKey, quoteMint.String()) {
			return config
		}
	}
	return fallback
}

func quoteConfigKey(quoteMint solana.PublicKey) string {
	switch {
	case pumpV3.IsSolLikeQuoteMint(quoteMint):
		return "SOL"
	case pumpV3.IsStableQuoteMint(quoteMint):
		return "USDC"
	default:
		return quoteMint.String()
	}
}
