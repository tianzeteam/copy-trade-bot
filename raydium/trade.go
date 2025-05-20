package raydium

import (
	"context"

	"copytrade/raydium/utils"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

type FeeConfig struct {
	MicroLamports uint64
}

func (r *Raydium) selectOrCreateAccount(amount *utils.TokenAmount, insttr *[]solana.Instruction, side string, signerPublicKey solana.PublicKey) (solana.PublicKey, error) {
	acc, err := r.Connection.GetTokenAccountsByOwner(context.Background(), signerPublicKey, &rpc.GetTokenAccountsConfig{Mint: amount.Token.PublicKey().ToPointer()}, &rpc.GetTokenAccountsOpts{
		Encoding: "jsonParsed",
	})
	if err != nil {
		return solana.PublicKey{}, err
	}

	if len(acc.Value) > 0 {
		return acc.Value[0].Pubkey, nil
	}

	ataAddress, _, err := solana.FindAssociatedTokenAddress(signerPublicKey, amount.Token.PublicKey())

	if err != nil {
		return solana.PublicKey{}, err
	}

	rentCost, err := r.Connection.GetMinimumBalanceForRentExemption(context.Background(), 165, rpc.CommitmentConfirmed)

	if err != nil {
		return solana.PublicKey{}, err
	}

	accountLamports := rentCost

	if amount.Mint == WSOl.String() {
		if side == "in" {
			accountLamports += uint64(amount.Amount)
		}

		pubKey, seed, err := r.generatePubkeyWithSeed(signerPublicKey, token.ProgramID)

		if err != nil {
			return solana.PublicKey{}, err
		}
		createInst, err := system.NewCreateAccountWithSeedInstruction(
			signerPublicKey,
			seed,
			accountLamports,
			165,
			token.ProgramID,
			signerPublicKey,
			pubKey,
			signerPublicKey,
		).ValidateAndBuild()

		if err != nil {
			return solana.PublicKey{}, err
		}

		initInst, err := token.NewInitializeAccountInstruction(
			pubKey,
			WSOl,
			signerPublicKey,
			solana.SysVarRentPubkey,
		).ValidateAndBuild()

		if err != nil {
			return solana.PublicKey{}, err
		}

		*insttr = append(*insttr, createInst)
		*insttr = append(*insttr, initInst)

		return pubKey, nil
	}

	createAtaInst, err := associatedtokenaccount.NewCreateInstruction(
		signerPublicKey,
		signerPublicKey,
		amount.Token.PublicKey(),
	).ValidateAndBuild()

	if err != nil {
		return solana.PublicKey{}, err
	}
	*insttr = append(*insttr, createAtaInst)

	return ataAddress, nil
}

func (r *Raydium) generatePubkeyWithSeed(from solana.PublicKey, programId solana.PublicKey) (solana.PublicKey, string, error) {
	seed := solana.NewWallet().PublicKey().String()[0:32]
	publicKey, err := solana.CreateWithSeed(from, seed, programId)

	return publicKey, seed, err
}
