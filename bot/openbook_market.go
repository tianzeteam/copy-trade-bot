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
			logger.Debugf("Received nil update from OpenBook market stream")
			continue
		}

		// Check if the update contains account data
		accountUpdate := update.GetAccount()
		if accountUpdate == nil {
			logger.Debugf("Received nil account data from OpenBook market stream")
			continue
		}

		// Process the market account update
		b.handleOpenBookMarketUpdate(accountUpdate)
	}
}

// handleOpenBookMarketUpdate processes a single OpenBook market account update
func (b *Bot) handleOpenBookMarketUpdate(accountUpdate *yellowstone_geyser_pb.SubscribeUpdateAccount) {

	pubkey := base58.Encode(accountUpdate.Account.Pubkey)
	marketID, err := solana.PublicKeyFromBase58(pubkey)
	if err != nil {
		logger.Warnf("Invalid market pubkey: %s, error: %v", pubkey, err)
		return
	}

	var marketLayout layouts.MarketStateLayoutV3
	if err := marketLayout.Decode(accountUpdate.Account.Data); err != nil {
		logger.Warnf("Failed to decode OpenBook market data for account %s: %v", pubkey, err)
		return
	}

	baseMint := marketLayout.BaseMint.String()
	quoteMint := marketLayout.QuoteMint.String()
	baseVault := marketLayout.BaseVault.String()
	quoteVault := marketLayout.QuoteVault.String()
	ownAddress := marketLayout.OwnAddress.String()

	// Create a new MarketState object
	marketState := global.NewMarketStateFromLayout(marketID, marketLayout)

	// 2. Store in global map for access from other components
	global.SetOpenBookMarketState(marketID.String(), marketState)

	// 3. Also store by baseMint for easier lookup by token
	global.SetOpenBookMarketState(baseMint, marketState)

	//Emit market update event

	logger.Debugf("OpenBook market update: Market=%s, OwnAddress=%s,BaseMint=%s, QuoteMint=%s, baseVault=%s, quoteVault=%s",
		pubkey, ownAddress, baseMint, quoteMint, baseVault, quoteVault)

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
