package bot

import (
	"copytrade/logger"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"
)

// SolBalanceChange represents a change in SOL balance for a specific account
type SolBalanceChange struct {
	TokenChangeAmount float64 // Amount of change in lamports
	SolChangeAmount   float64 // Progress percentage (if applicable)
}

func GetSwapBalances(txn *yellowstone_geyser_pb.SubscribeUpdateTransaction, poolCoinTokenAccount, poolPcTokenAccount string, accountList []string) *SolBalanceChange {

	preBalances := txn.Transaction.Meta.PreBalances
	postBalances := txn.Transaction.Meta.PostBalances

	preTokenBalances := txn.Transaction.Meta.PreTokenBalances
	postTokenBalances := txn.Transaction.Meta.PostTokenBalances

	
		// Find the index of poolPcTokenAccount in accountList
	poolPcTokenAccountIndex := -1
	for i, account := range accountList {
		if account == poolPcTokenAccount {
			poolPcTokenAccountIndex = i
			logger.Debugf("Found poolPcTokenAccount at index %d", poolPcTokenAccountIndex)
			break
		}
	}

	if poolPcTokenAccountIndex == -1 {
		logger.Warnf("poolPcTokenAccount %s not found in accountList", poolPcTokenAccount)
		return nil
	}
	poolCoinTokenAccountIndex := -1
	for i, account := range accountList {
		if account == poolCoinTokenAccount {
			poolCoinTokenAccountIndex = i
			logger.Debugf("Found poolPcTokenAccount at index %d", poolPcTokenAccountIndex)
			break
		}
	}

	if poolCoinTokenAccountIndex == -1 {
		logger.Warnf("poolPcTokenAccount %s not found in accountList", poolPcTokenAccount)
		return nil
	}

	if len(preBalances) == 0 || len(postBalances) == 0 || len(preBalances) != len(postBalances) {
		return nil
	}
	var swapSolAmount float64
	var tokenAmount float64

	// Find SOL balance change in the pool PC token account
	for _, preTokenBalance := range preTokenBalances {
		if preTokenBalance.AccountIndex == uint32(poolPcTokenAccountIndex) {
			preSolBalance := preTokenBalance.GetUiTokenAmount().UiAmount
			// Find matching post token balance
			for _, postTokenBalance := range postTokenBalances {
				if postTokenBalance.AccountIndex == preTokenBalance.AccountIndex {
					postSolBalance := postTokenBalance.GetUiTokenAmount().UiAmount
					swapSolAmount = postSolBalance - preSolBalance
					logger.Debugf("SOL balance change: %.8f", swapSolAmount)
					break
				}
			}
			break
		}
	}

	// Find token balance change in the pool coin token account
	for _, preTokenBalance := range preTokenBalances {
		if preTokenBalance.AccountIndex == uint32(poolCoinTokenAccountIndex) {
			preTokenAmount := preTokenBalance.GetUiTokenAmount().UiAmount
			// Find matching post token balance
			for _, postTokenBalance := range postTokenBalances {
				if postTokenBalance.AccountIndex == preTokenBalance.AccountIndex {
					postTokenAmount := postTokenBalance.GetUiTokenAmount().UiAmount
					tokenAmount = postTokenAmount - preTokenAmount
					logger.Debugf("Token balance change: %.8f", tokenAmount)
					break
				}
			}
			break
		}
	}

	// Include poolPcTokenAccount information in the result if found
	result := &SolBalanceChange{
		SolChangeAmount:   swapSolAmount,
		TokenChangeAmount: tokenAmount,
	}

	return result
}
