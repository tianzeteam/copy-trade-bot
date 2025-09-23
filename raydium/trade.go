package raydium

import (
	"github.com/gagliardetto/solana-go"
)

type FeeConfig struct {
	MicroLamports uint64
}

func (r *Raydium) generatePubkeyWithSeed(from solana.PublicKey, programId solana.PublicKey) (solana.PublicKey, string, error) {
	seed := solana.NewWallet().PublicKey().String()[0:32]
	publicKey, err := solana.CreateWithSeed(from, seed, programId)

	return publicKey, seed, err
}
