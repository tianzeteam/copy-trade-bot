package bot

import (
	"context"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/raydium/layouts"
	"copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

// SubscribeToRaydiumPools subscribes to Raydium V4 pool accounts for a specific quote token
func (b *Bot) SubscribeToRaydiumPools(ctx context.Context, client *yellowstone_geyser.Client, quoteTokenMint solana.PublicKey) error {
	// Create a new stream client for Raydium pools
	streamName := fmt.Sprintf("raydium_pools_%s", quoteTokenMint.String())
	if err := client.AddStreamClient(ctx, streamName, yellowstone_geyser_pb.CommitmentLevel_CONFIRMED); err != nil {
		return fmt.Errorf("failed to add stream client for Raydium pools: %w", err)
	}

	// Get the stream client
	poolStreamClient := client.GetStreamClient(streamName)
	if poolStreamClient == nil {
		return fmt.Errorf("failed to get stream client for Raydium pools")
	}

	// Data size filter (equivalent to dataSize: LIQUIDITY_STATE_LAYOUT_V4.span)
	datasizeFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Datasize{
			Datasize: 1328, // Size of Raydium V4 liquidity state layout
		},
	}

	// Quote mint filter (equivalent to memcmp for quoteMint)
	quoteMintFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 72, // Offset of quoteMint in the layout
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: quoteTokenMint.String(),
				},
			},
		},
	}

	// Market program ID filter (equivalent to memcmp for marketProgramId)
	marketProgramFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 104, // Offset of marketProgramId in the layout
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: core.OPENBOOK_MARKET_PROGRAM_ID.String(),
				},
			},
		},
	}

	// Status filter (equivalent to memcmp for status)
	statusFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 0, // Offset of status in the layout
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Bytes{
					Bytes: []byte{6, 0, 0, 0, 0, 0, 0, 0}, // Status 6 (active pools)
				},
			},
		},
	}

	// Subscribe to accounts owned by the Raydium V4 program
	if err := poolStreamClient.SubscribeAccounts("raydium_pool_accounts", &yellowstone_geyser_pb.SubscribeRequestFilterAccounts{
		Owner: []string{core.RAYDIUM_V4_PROGRAM_ID.String()}, // Only monitor accounts owned by Raydium V4 program
		Filters: []*yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
			datasizeFilter,      // Data size filter
			quoteMintFilter,     // Quote mint filter
			marketProgramFilter, // Market program ID filter
			statusFilter,        // Status filter
		},
	}); err != nil {
		return fmt.Errorf("failed to subscribe to Raydium pool accounts: %w", err)
	}

	// Start ping service to keep the connection alive
	go b.startPingService(poolStreamClient)

	// Process pool updates in a goroutine
	go b.processRaydiumPoolUpdates(poolStreamClient)

	logger.Infof("Successfully subscribed to Raydium V4 pools for quote token: %s", quoteTokenMint.String())
	return nil
}

// processRaydiumPoolUpdates processes updates from Raydium pool accounts
func (b *Bot) processRaydiumPoolUpdates(poolStreamClient *yellowstone_geyser.StreamClient) {
	for update := range poolStreamClient.Ch {
		// Check if update is nil
		if update == nil {
			logger.Warn("Received nil update from Raydium pool stream")
			continue
		}

		// Check if the update contains account data
		accountUpdate := update.GetAccount()
		if accountUpdate == nil {
			logger.Warn("Received nil account data from Raydium pool stream")
			continue
		}

		// Process the pool account update
		b.handleRaydiumPoolUpdate(accountUpdate)
	}
}

// handleRaydiumPoolUpdate processes a single Raydium pool account update
func (b *Bot) handleRaydiumPoolUpdate(accountUpdate *yellowstone_geyser_pb.SubscribeUpdateAccount) {
	pubkey := base58.Encode(accountUpdate.Account.Pubkey)
	poolID, err := solana.PublicKeyFromBase58(pubkey)
	if err != nil {
		logger.Warnf("Invalid pool pubkey: %s, error: %v", pubkey, err)
		return
	}

	// Decode the pool state
	var poolLayout layouts.LIQUIDITY_STATE_LAYOUT_V4
	if err := poolLayout.Decode(accountUpdate.Account.Data); err != nil {
		logger.Warnf("Failed to decode Raydium V4 pool data for account %s: %v", pubkey, err)
		return
	}

	// Extract pool information
	baseMint := poolLayout.BaseMint.String()
	quoteMint := poolLayout.QuoteMint.String()
	// Get the reserves from the swap amounts
	baseReserve := poolLayout.SwapBaseInAmount.Lo
	quoteReserve := poolLayout.SwapQuoteOutAmount.Lo
	marketId := poolLayout.MarketId.String()

	// Calculate current price
	currentPrice := (float64(quoteReserve) / 1e9) / (float64(baseReserve) / 1e6)
	logger.Infof("Calculated Raydium V4 pool price: %f", currentPrice)

	// Create trade info object
	tradeInfo := &global.AmmRaydiumV4TradeInfo{
		AmmPool:      poolID,
		MintAddress:  poolLayout.BaseMint,
		CurrentPrice: currentPrice,
		MarketId:     poolLayout.MarketId,
		BaseVault:    poolLayout.BaseVault,
		QuoteVault:   poolLayout.QuoteVault,
		OpenOrders:   poolLayout.OpenOrders,
		TargetOrders: poolLayout.TargetOrders,
	}

	// Store in global map for access from other components
	global.SetRaydiumV4TradeInfo(baseMint, tradeInfo)

	// Emit pool update event
	logger.Infof("Raydium V4 pool update: Pool=%s, BaseMint=%s, QuoteMint=%s, BaseReserve=%d, QuoteReserve=%d, MarketId=%s, Price=%f",
		pubkey, baseMint, quoteMint, baseReserve, quoteReserve, marketId, currentPrice)
}
