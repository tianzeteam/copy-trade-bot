package bot

import (
	"context"
	"copytrade/core"
	"copytrade/logger"
	"copytrade/raydium/layouts"
	"copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

// SubscribeToOpenBookMarkets subscribes to OpenBook market accounts for a specific quote token
func (b *Bot) SubscribeToOpenBookMarkets(ctx context.Context, client *yellowstone_geyser.Client, quoteTokenMint solana.PublicKey) error {
	// Create a new stream client for OpenBook market accounts
	streamName := fmt.Sprintf("openbook_markets_%s", quoteTokenMint.String())
	if err := client.AddStreamClient(ctx, streamName, yellowstone_geyser_pb.CommitmentLevel_CONFIRMED); err != nil {
		return fmt.Errorf("failed to add stream client for OpenBook markets: %w", err)
	}

	// Get the stream client
	marketStreamClient := client.GetStreamClient(streamName)
	if marketStreamClient == nil {
		return fmt.Errorf("failed to get stream client for OpenBook markets")
	}

	datasizeFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Datasize{
			Datasize: 388,
		},
	}

	memcmpFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 85,
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: quoteTokenMint.String(),
				},
			},
		},
	}

	// Subscribe to accounts owned by the OpenBook program
	if err := marketStreamClient.SubscribeAccounts("openbook_market_accounts", &yellowstone_geyser_pb.SubscribeRequestFilterAccounts{
		Owner: []string{core.OPENBOOK_MARKET_PROGRAM_ID.String()}, // Only monitor accounts owned by OpenBook program
		Filters: []*yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
			datasizeFilter, // Data size filter
			memcmpFilter,   // Memcmp filter for quoteMint
		},
	}); err != nil {
		return fmt.Errorf("failed to subscribe to OpenBook market accounts: %w", err)
	}

	// Start ping service to keep the connection alive
	go b.startPingService(marketStreamClient)

	// Process market updates in a goroutine
	go b.processOpenBookMarketUpdates(marketStreamClient)

	logger.Infof("Successfully subscribed to OpenBook markets for quote token: %s", quoteTokenMint.String())
	return nil
}

// processOpenBookMarketUpdates processes updates from OpenBook market accounts
func (b *Bot) processOpenBookMarketUpdates(marketStreamClient *yellowstone_geyser.StreamClient) {
	for update := range marketStreamClient.Ch {
		// Check if update is nil
		if update == nil {
			logger.Warn("Received nil update from OpenBook market stream")
			continue
		}

		// Check if the update contains account data
		accountUpdate := update.GetAccount()
		if accountUpdate == nil {
			logger.Warn("Received nil account data from OpenBook market stream")
			continue
		}

		// Process the market account update
		b.handleOpenBookMarketUpdate(accountUpdate)
	}
}

// handleOpenBookMarketUpdate processes a single OpenBook market account update
func (b *Bot) handleOpenBookMarketUpdate(accountUpdate *yellowstone_geyser_pb.SubscribeUpdateAccount) {
	// Get the account pubkey
	pubkey := base58.Encode(accountUpdate.Account.Pubkey)
	marketID, err := solana.PublicKeyFromBase58(pubkey)
	if err != nil {
		logger.Warnf("Invalid market pubkey: %s, error: %v", pubkey, err)
		return
	}

	// Decode the market state
	var marketLayout layouts.MarketStateLayoutV3
	if err := marketLayout.Decode(accountUpdate.Account.Data); err != nil {
		logger.Warnf("Failed to decode OpenBook market data for account %s: %v", pubkey, err)
		return
	}

	// Extract market information
	baseMint := marketLayout.BaseMint.String()
	quoteMint := marketLayout.QuoteMint.String()
	baseVault := marketLayout.BaseVault.String()
	quoteVault := marketLayout.QuoteVault.String()

	//lpMint := marketLayout.LpMint.String()
	//lpVault := marketLayout.LpVault.String()

	// Create a new MarketState object
	marketState := NewMarketStateFromLayout(marketID, marketLayout)

	// Store the market state in memory with baseMint as key
	b.marketStatesLock.Lock()
	b.marketStates[marketID.String()] = marketState
	b.marketStatesLock.Unlock()

	//Emit market update event
	logger.Infof("OpenBook market update: Market=%s, BaseMint=%s, QuoteMint=%s,baseVault=%s,quoteVault=%s",
		pubkey, baseMint, quoteMint, baseVault, quoteVault)

}

// startPingService sends periodic pings to keep the connection alive
func (b *Bot) startPingService(streamClient *yellowstone_geyser.StreamClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := streamClient.SendCustomRequest(&yellowstone_geyser_pb.SubscribeRequest{
				Ping: &yellowstone_geyser_pb.SubscribeRequestPing{Id: 1},
			})
			if err != nil {
				logger.Warnf("Failed to send ping to stream: %v", err)
			}
		}
	}
}
