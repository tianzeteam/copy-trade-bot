package global

import (
	"testing"

	"github.com/gagliardetto/solana-go"
)

func TestPumpCurveSnapshotUpdatePreservesCachedFields(t *testing.T) {
	mint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	creator := solana.MustPublicKeyFromBase58("HmgSia9qke4yYRRYQaXFs6dj4Nt7BSu4UNbaWymsTeS6")
	SetPumpCurveSnapshot(PumpCurveSnapshot{
		Mint:                 mint,
		Creator:              creator,
		VirtualTokenReserves: 100,
		TokenTotalSupply:     200,
	})

	UpdatePumpCurveSnapshot(mint, func(snapshot *PumpCurveSnapshot) {
		snapshot.VirtualTokenReserves = 101
	})

	snapshot, ok := GetPumpCurveSnapshot(mint)
	if !ok {
		t.Fatal("snapshot missing")
	}
	if snapshot.Creator != creator || snapshot.TokenTotalSupply != 200 {
		t.Fatalf("cached fields were lost: %+v", snapshot)
	}
	if snapshot.VirtualTokenReserves != 101 {
		t.Fatalf("virtual token reserves = %d, want 101", snapshot.VirtualTokenReserves)
	}
}

func TestPumpCurveSnapshotRejectsZeroMint(t *testing.T) {
	SetPumpCurveSnapshot(PumpCurveSnapshot{VirtualTokenReserves: 1})
	if _, ok := GetPumpCurveSnapshot(solana.PublicKey{}); ok {
		t.Fatal("zero mint unexpectedly cached")
	}
}
