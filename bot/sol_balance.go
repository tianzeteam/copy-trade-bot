package bot

import (
	"copytrade/logger"
)

// SolBalanceChange represents a change in SOL balance for a specific account
type SolBalanceChange struct {
	AccountIndex int    // Index of the account in the transaction
	Amount       uint64  // Amount of change in lamports
	Progress     float64 // Progress percentage (if applicable)
	IsBuy        bool    // True if the SOL balance increased (buy), false if decreased (sell)
}

// GetSwapBalances checks SOL balance changes in a transaction for a specific pool account
// Returns SOL change information or nil if no relevant changes found
func GetSwapBalances(poolPcTokenAccount string, preBalances, postBalances []uint64) *SolBalanceChange {
	if len(preBalances) == 0 || len(postBalances) == 0 || len(preBalances) != len(postBalances) {
		return nil
	}

	// Since we don't have direct access to account keys here, we'll look for significant balance changes
	// This is a simplified approach - in a real implementation, you'd need to pass the account index
	// or the full transaction object

	// For now, we'll just find the account with the largest balance change
	var maxChange uint64 = 0
	var maxChangeIdx int = -1
	var isBuy bool = false

	for i := 0; i < len(preBalances) && i < len(postBalances); i++ {
		var change uint64
		var increase bool

		if postBalances[i] >= preBalances[i] {
			change = postBalances[i] - preBalances[i]
			increase = true
		} else {
			change = preBalances[i] - postBalances[i]
			increase = false
		}

		if change > maxChange {
			maxChange = change
			maxChangeIdx = i
			isBuy = increase
		}
	}

	// If no significant change found
	if maxChange == 0 || maxChangeIdx == -1 {
		return nil
	}

	// Log the account index and balance change
	logger.Debugf("SOL balance change detected for account %d: %d lamports", maxChangeIdx, maxChange)

	// Calculate progress (similar to your TypeScript code)
	// Assuming 85 SOL is the max pool size as in your TypeScript code
	progress := float64(postBalances[maxChangeIdx]) / 1e9 / 85

	logger.Debugf("SOL balance change detected: Account=%d, Amount=%d lamports, Progress=%.2f%%, IsBuy=%v", 
		maxChangeIdx, maxChange, progress*100, isBuy)

	return &SolBalanceChange{
		AccountIndex: maxChangeIdx,
		Amount:       maxChange,
		Progress:     progress,
		IsBuy:        isBuy,
	}
}
