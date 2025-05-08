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
func (c *AccountsCalculator) CalculatePumpAccounts(ctx context.Context, mint, user solana.PublicKey) (*PumpAccounts, error) {
	// 计算bonding curve地址
	bondingCurve, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("bonding-curve"),
			mint.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("计算bonding curve失败: %w", err)
	}
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

// AmmAccounts 存储AMM相关的账户地址
type AmmAccounts struct {
	Pool                  solana.PublicKey // AMM池地址
	Mint                  solana.PublicKey // 基础代币地址
	BaseMint              solana.PublicKey // 基础代币地址
	QuoteMint             solana.PublicKey // 报价代币地址
	PoolBaseTokenAccount  solana.PublicKey // 池中基础代币账户
	PoolQuoteTokenAccount solana.PublicKey // 池中报价代币账户
	UserBaseTokenAccount  solana.PublicKey // 用户的基础代币账户
	UserQuoteTokenAccount solana.PublicKey // 用户的报价代币账户
	WsolTokenAccount      solana.PublicKey // WSOL账户
}

// CalculateAmmAccounts 计算AMM所需的所有账户
func (c *AccountsCalculator) CalculateAmmAccounts(ctx context.Context, pool, baseMint, user solana.PublicKey) (*AmmAccounts, error) {
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

	// 假设我们使用SOL作为报价代币
	quoteMint := solana.SolMint

	// 计算pool_base_token_account (池中基础代币账户)
	// 根据IDL，seeds是 [pool, base_token_program, base_mint]
	poolBaseTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			pool.Bytes(),
			solana.TokenProgramID.Bytes(), // 假设使用标准Token程序
			baseMint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID, // 使用Associated Token Account程序
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find pool base token account: %w", err)
	}

	// 计算pool_quote_token_account (池中报价代币账户)
	// 根据IDL，seeds是 [pool, quote_token_program, quote_mint]
	poolQuoteTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			pool.Bytes(),
			solana.TokenProgramID.Bytes(), // 假设使用标准Token程序
			quoteMint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID, // 使用Associated Token Account程序
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find pool quote token account: %w", err)
	}

	// 计算用户的基础代币associated token account
	userBaseTokenAccount, _, err := solana.FindAssociatedTokenAddress(
		user,
		baseMint,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find user base token account: %w", err)
	}

	// 计算用户的报价代币associated token account
	// 如果是SOL，则使用WSOL账户
	userQuoteTokenAccount := wsolTokenAccount
	if quoteMint != solana.SolMint {
		userQuoteTokenAccount, _, err = solana.FindAssociatedTokenAddress(
			user,
			quoteMint,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to find user quote token account: %w", err)
		}
	}

	// 返回所有计算出的账户地址
	return &AmmAccounts{
		Pool:                  pool,
		Mint:                  baseMint,
		BaseMint:              baseMint,
		QuoteMint:             quoteMint,
		PoolBaseTokenAccount:  poolBaseTokenAccount,
		PoolQuoteTokenAccount: poolQuoteTokenAccount,
		UserBaseTokenAccount:  userBaseTokenAccount,
		UserQuoteTokenAccount: userQuoteTokenAccount,
		WsolTokenAccount:      wsolTokenAccount,
	}, nil
}
