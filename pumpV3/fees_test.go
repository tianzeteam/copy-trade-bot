package pumpV3

import (
	"math/big"
	"testing"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

func TestFeeForAmountUsesCeiling(t *testing.T) {
	if got := FeeForAmount(100, 100); got != 1 {
		t.Fatalf("FeeForAmount(100, 100) = %d, want 1", got)
	}
	if got := FeeForAmount(101, 100); got != 2 {
		t.Fatalf("FeeForAmount(101, 100) = %d, want 2", got)
	}
}

func TestCalculateFeeTierSelectsHighestReachedThreshold(t *testing.T) {
	tiers := []FeeTier{
		{MarketCapLamportsThreshold: bin.Uint128{Lo: 100}, Fees: Fees{ProtocolFeeBps: 100}},
		{MarketCapLamportsThreshold: bin.Uint128{Lo: 200}, Fees: Fees{ProtocolFeeBps: 200}},
		{MarketCapLamportsThreshold: bin.Uint128{Lo: 300}, Fees: Fees{ProtocolFeeBps: 300}},
	}

	got, err := CalculateFeeTier(tiers, marketCap(50))
	if err != nil || got.ProtocolFeeBps != 100 {
		t.Fatalf("below first tier = %+v, %v", got, err)
	}
	got, err = CalculateFeeTier(tiers, marketCap(200))
	if err != nil || got.ProtocolFeeBps != 200 {
		t.Fatalf("at second tier = %+v, %v", got, err)
	}
}

func TestV3BuyAndSellIncludeCreatorFee(t *testing.T) {
	const (
		budget       = uint64(1_000_000_000)
		virtualQuote = uint64(30_000_000_000)
		virtualToken = uint64(1_073_000_000_000_000)
		realToken    = uint64(793_100_000_000_000)
	)
	bps := FeeBps{Protocol: 100, Creator: 200}
	creator := solana.MustPublicKeyFromBase58("HmgSia9qke4yYRRYQaXFs6dj4Nt7BSu4UNbaWymsTeS6")

	tokens, err := BuyTokenAmountFromQuoteBudget(budget, virtualQuote, virtualToken, realToken, bps.Total())
	if err != nil || tokens == 0 {
		t.Fatalf("BuyTokenAmountFromQuoteBudget = %d, %v", tokens, err)
	}
	cost, err := BuyQuoteCostFromTokenAmount(tokens, virtualQuote, virtualToken, realToken, bps, creator)
	if err != nil || cost > budget {
		t.Fatalf("BuyQuoteCostFromTokenAmount = %d, %v; budget = %d", cost, err, budget)
	}

	net, err := SellQuoteOutputFromTokenAmount(tokens, virtualQuote, virtualToken, bps, creator)
	if err != nil || net == 0 {
		t.Fatalf("SellQuoteOutputFromTokenAmount = %d, %v", net, err)
	}
}

func TestSupportedQuoteMints(t *testing.T) {
	if !IsSupportedQuoteMint(WSOLMint) || !IsSupportedQuoteMint(USDCMint) {
		t.Fatal("SOL and USDC must be supported quote mints")
	}
	if IsSupportedQuoteMint(solana.MustPublicKeyFromBase58("11111111111111111111111111111112")) {
		t.Fatal("unknown quote mint unexpectedly supported")
	}
	if QuoteDecimals(USDCMint) != 6 {
		t.Fatalf("USDC decimals = %d, want 6", QuoteDecimals(USDCMint))
	}
}

func marketCap(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}
