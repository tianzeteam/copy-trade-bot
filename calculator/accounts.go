package calculator

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	PUMP_PROGRAM_ID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
)

// AccountsCalculator 账户计算器
type AccountsCalculator struct {
	client *rpc.Client
}

func NewAccountsCalculator(endpoint string) *AccountsCalculator {
	return &AccountsCalculator{
		client: rpc.New(endpoint),
	}
}

// CalculatePumpAccounts 计算Pump所需的所有账户
func (c *AccountsCalculator) CalculatePumpAccounts(ctx context.Context, mint, user, bondingCurve solana.PublicKey) (*PumpAccounts, error) {
	// 计算bonding curve地址

	// 计算associated bonding curve地址
	associatedBondingCurve, _, err := solana.FindAssociatedTokenAddress(
		bondingCurve,
		mint,
	)
	if err != nil {
		return nil, fmt.Errorf("计算associated bonding curve失败: %w", err)
	}

	// 计算用户的associated token account
	associatedTokenAccount, _, err := solana.FindAssociatedTokenAddress(
		user,
		mint,
	)
	if err != nil {
		return nil, fmt.Errorf("计算associated token account失败: %w", err)
	}

	return &PumpAccounts{
		BondingCurve:           bondingCurve,
		AssociatedBondingCurve: associatedBondingCurve,
		AssociatedTokenAccount: associatedTokenAccount,
	}, nil
}

// PumpAccounts 存储所有需要的账户地址
type PumpAccounts struct {
	BondingCurve           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
	AssociatedTokenAccount solana.PublicKey
}

// CalculateAmmAccounts 计算AMM所需的所有账户
func (c *AccountsCalculator) CalculateAmmAccounts(ctx context.Context, mint, user solana.PublicKey) (*AmmAccounts, error) {
	// 为WSOL账户生成种子
	log.Println("Generating seed for WSOL account...")
	randomBytes := make([]byte, 24)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %v", err)
	}
	seed := base64.URLEncoding.EncodeToString(randomBytes)

	// 创建WSOL账户
	wsolTokenAccount, err := solana.CreateWithSeed(
		user,
		seed,
		solana.TokenProgramID,
	)
	if err != nil {
		log.Printf("failed to create WSOL account with seed: %v", err)
		return nil, fmt.Errorf("failed to create WSOL account with seed: %v", err)
	}

	// 计算用户的associated token account
	associatedTokenAccount, _, err := solana.FindAssociatedTokenAddress(
		user,
		mint,
	)
	if err != nil {
		return nil, fmt.Errorf("计算associated token account失败: %w", err)
	}

	return &AmmAccounts{
		Mint:                   mint,
		WsolTokenAccount:       wsolTokenAccount,
		AssociatedTokenAccount: associatedTokenAccount,
	}, nil
}

// AmmAccounts 存储所有需要的账户地址
type AmmAccounts struct {
	Mint                   solana.PublicKey
	WsolTokenAccount       solana.PublicKey
	AssociatedTokenAccount solana.PublicKey
}
