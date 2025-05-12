package bot

import (
	"github.com/gagliardetto/solana-go"
)

// GetMarketStateByBaseMint retrieves a market state by base mint address
func (b *Bot) GetMarketStateByBaseMint(baseMint solana.PublicKey) *MarketState {
	b.marketStatesLock.RLock()
	defer b.marketStatesLock.RUnlock()
	
	// Convert PublicKey to string for map lookup
	baseMintStr := baseMint.String()
	
	// Return the market state if found
	if marketState, ok := b.marketStates[baseMintStr]; ok {
		return marketState
	}
	
	// Return nil if not found
	return nil
}

// GetAllMarketStates returns a copy of all market states
func (b *Bot) GetAllMarketStates() map[string]*MarketState {
	b.marketStatesLock.RLock()
	defer b.marketStatesLock.RUnlock()
	
	// Create a copy of the market states map
	result := make(map[string]*MarketState, len(b.marketStates))
	for k, v := range b.marketStates {
		result[k] = v
	}
	
	return result
}

// GetMarketStatesByQuoteMint returns all markets that use a specific quote token
func (b *Bot) GetMarketStatesByQuoteMint(quoteMint solana.PublicKey) []*MarketState {
	b.marketStatesLock.RLock()
	defer b.marketStatesLock.RUnlock()
	
	quoteMintStr := quoteMint.String()
	var result []*MarketState
	
	// Find all markets with the specified quote mint
	for _, marketState := range b.marketStates {
		if marketState.QuoteMint.String() == quoteMintStr {
			result = append(result, marketState)
		}
	}
	
	return result
}
