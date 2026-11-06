package calculator

import (
	"copytrade/global"
	pumpV3 "copytrade/pumpV3"
	bin "github.com/gagliardetto/binary"
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestCalculatePumpV3BuyAmountsUsesSnapshotWithoutRPC(t *testing.T) {
	global.SetPumpFeeConfig(nil, &pumpV3.Global{
		FeeBasisPoints:        100,
		CreatorFeeBasisPoints: 100,
	})
	mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	creator := solana.MustPublicKeyFromBase58("HmgSia9qke4yYRRYQaXFs6dj4Nt7BSu4UNbaWymsTeS6")
	snapshot := global.PumpCurveSnapshot{
		Mint:                 mint,
		Creator:              creator,
		QuoteMint:            pumpV3.WSOLMint,
		VirtualQuoteReserves: 30_000_000_000,
		VirtualTokenReserves: 1_073_000_000_000_000,
		RealTokenReserves:    793_100_000_000_000,
		TokenTotalSupply:     pumpV3.OneBillionSupply,
	}

	calc := NewAmountsCalculator("http://127.0.0.1:1")
	amounts, err := calc.CalculatePumpV3BuyAmounts(snapshot, 1_000_000_000, 0.01)
	if err != nil {
		t.Fatalf("CalculatePumpV3BuyAmounts() error = %v", err)
	}
	if amounts.TokenAmount == 0 || amounts.MaxCost == 0 {
		t.Fatalf("unexpected amounts: %+v", amounts)
	}
}

func TestCalculatePumpV3BuyAmountsUsesUSDCStableFees(t *testing.T) {
	global.SetPumpFeeConfig(&pumpV3.FeeConfig{
		StableFeeTiers: []pumpV3.FeeTier{{
			MarketCapLamportsThreshold: bin.Uint128{},
			Fees:                       pumpV3.Fees{ProtocolFeeBps: 95, CreatorFeeBps: 30},
		}},
	}, &pumpV3.Global{})
	snapshot := global.PumpCurveSnapshot{
		Mint:                 solana.MustPublicKeyFromBase58("C4hZ7KCD1cabwNuH9XaiXo2N9wdZcWDwViUZPh9apump"),
		Creator:              solana.MustPublicKeyFromBase58("HYhpwyCNnYF6p6Q8LGH5vHxF9HbCcqi9j5uDUrHomGzQ"),
		QuoteMint:            pumpV3.USDCMint,
		VirtualQuoteReserves: 3_765_071_948,
		VirtualTokenReserves: 1_055_018_046_401_349,
		RealTokenReserves:    775_118_046_401_349,
		TokenTotalSupply:     pumpV3.OneBillionSupply,
	}
	calc := NewAmountsCalculator("http://127.0.0.1:1")
	bps, err := calc.pumpFeeBps(snapshot)
	if err != nil || bps.Protocol != 95 || bps.Creator != 30 {
		t.Fatalf("USDC stable fee = %+v, %v", bps, err)
	}
	amounts, err := calc.CalculatePumpV3BuyAmounts(snapshot, 1_000_000, 0.01)
	if err != nil || amounts.TokenAmount == 0 {
		t.Fatalf("USDC buy amounts = %+v, %v", amounts, err)
	}
}
