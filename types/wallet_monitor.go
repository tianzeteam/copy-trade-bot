package types

import (
	"context"
	"time"

	"github.com/gagliardetto/solana-go"
)

// WalletType defines wallet type
type WalletType string

// WalletSettings defines settings for a monitored wallet
type WalletSettings struct {
	Address       solana.PublicKey // Wallet address
	Name          string           // Wallet name or identifier
	Type          WalletType       // Wallet type
	TakeProfitPct int              // Take profit percentage
	StopLossPct   int              // Stop loss percentage
	ExitTimeSec   int              // Position holding time (seconds)
	Enabled       bool             // Whether enabled
}

// TradeRecord stores trade information
type TradeRecord struct {
	MintAddress   string          // Token address
	WalletAddress string          // Trading wallet address
	EntryPrice    float64         // Entry price
	Amount        uint64          // Trade amount
	Timestamp     time.Time       // Trade time
	ExitDeadline  time.Time       // Exit deadline
	Settings      *WalletSettings // Trade settings
	StrategyName  string          // Strategy name
}

// WalletMonitorInterface defines the interface for wallet monitoring
type WalletMonitorInterface interface {
	// TradeRecords returns the map of trade records
	GetTradeRecords() map[string]*TradeRecord
	
	// AddWallet adds a wallet to monitor
	AddWallet(wallet *WalletSettings)
	
	// RemoveWallet removes a monitored wallet
	RemoveWallet(address solana.PublicKey) bool
	
	// GetWallet gets wallet settings
	GetWallet(address solana.PublicKey) *WalletSettings
	
	// RecordTrade records original trader's trade
	RecordTrade(mintAddress string, copyAddress solana.PublicKey, entryPrice float64, originAmount uint64)
	
	// RecordBotTrade records bot's copy trade
	RecordBotTrade(mintAddress string, copyAddress, botAddress solana.PublicKey, entryPrice float64)
	
	// CheckExitConditions checks if exit conditions are met
	CheckExitConditions(platform, mintAddress string, currentPrice float64) (bool, string, *TradeRecord)
	
	// RemoveTrade removes a trade record
	RemoveTrade(mintAddress string)
	
	// StartMonitoring starts monitoring
	StartMonitoring(ctx context.Context)
}
