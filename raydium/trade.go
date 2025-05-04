package raydium

import (
	"context"

	"copytrade/raydium/layouts"
	"copytrade/raydium/utils"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

type FeeConfig struct {
	MicroLamports uint64
}

func (r *Raydium) MakeSwapTransaction(poolKeys *layouts.ApiPoolInfoV4, amountIn *utils.TokenAmount, minAmountOut *utils.TokenAmount, feeConfig FeeConfig, signerPublicKey solana.PublicKey) ([]solana.Instruction, error) {
	_, err := r.Connection.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)

	if err != nil {
		return nil, err
	}

	var instructions []solana.Instruction = []solana.Instruction{
		computebudget.NewSetComputeUnitPriceInstruction(feeConfig.MicroLamports).Build(),
		computebudget.NewSetComputeUnitLimitInstruction(600000).Build(),
	}

	tokenAccountIn, err := r.selectOrCreateAccount(amountIn, &instructions, "in", signerPublicKey)

	if err != nil {
		return nil, err
	}

	tokenAccountOut, err := r.selectOrCreateAccount(minAmountOut, &instructions, "out", signerPublicKey)

	if err != nil {
		return nil, err
	}
	instr, err := NewSwapV4Instruction(
		poolKeys,
		uint64(amountIn.Amount),
		uint64(minAmountOut.Amount),
		tokenAccountIn,
		tokenAccountOut,
		signerPublicKey,
	)

	if err != nil {
		return nil, err
	}

	instructions = append(instructions, instr)

	if amountIn.Token.Mint == WSOl.String() {
		closeAccInst, err := token.NewCloseAccountInstruction(
			tokenAccountIn,
			signerPublicKey,
			signerPublicKey,
			[]solana.PublicKey{},
		).ValidateAndBuild()

		if err != nil {
			return nil, err
		}

		instructions = append(instructions, closeAccInst)
	} else if minAmountOut.Token.Mint == WSOl.String() {
		closeAccInst, err := token.NewCloseAccountInstruction(
			tokenAccountOut,
			signerPublicKey,
			signerPublicKey,
			[]solana.PublicKey{},
		).ValidateAndBuild()

		if err != nil {
			return nil, err
		}

		instructions = append(instructions, closeAccInst)
	}

	return instructions, nil
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
