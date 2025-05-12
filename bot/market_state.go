package bot

import (
	"copytrade/raydium/layouts"
	"github.com/gagliardetto/solana-go"
)

// MarketState represents an OpenBook market state with additional metadata
type MarketState struct {
	// Market account public key
	MarketID solana.PublicKey
	// Market state data
	Layout layouts.MarketStateLayoutV3
	// Base token mint address
	BaseMint solana.PublicKey
	// Quote token mint address
	QuoteMint solana.PublicKey
	// Base vault address
	BaseVault solana.PublicKey
	// Quote vault address
	QuoteVault solana.PublicKey
	// Bids account address
	Bids solana.PublicKey
	// Asks account address
	Asks solana.PublicKey
	// Base lot size
	BaseLotSize uint64
	// Quote lot size
	QuoteLotSize uint64
	// Fee rate in basis points
	FeeRateBps uint64
}

// NewMarketStateFromLayout creates a new MarketState from a MarketStateLayoutV3
func NewMarketStateFromLayout(marketID solana.PublicKey, layout layouts.MarketStateLayoutV3) *MarketState {
	return &MarketState{
		MarketID:     marketID,
		Layout:       layout,
		BaseMint:     layout.BaseMint,
		QuoteMint:    layout.QuoteMint,
		BaseVault:    layout.BaseVault,
		QuoteVault:   layout.QuoteVault,
		Bids:         layout.Bids,
		Asks:         layout.Asks,
		BaseLotSize:  layout.BaseLotSize,
		QuoteLotSize: layout.QuoteLotSize,
		FeeRateBps:   layout.FeeRateBps,
	}
}
