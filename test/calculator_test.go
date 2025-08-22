package test

import (
	"testing"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"

	"copytrade/IDL/generated/raydium_cp_swap"
	"copytrade/calculator"
)

// TestCalculateAmmAccounts tests the CalculateAmmAccounts function
func TestCalculateAmmAccounts(t *testing.T) {
	// Setup
	// Use a mock RPC endpoint for testing
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data
	pool := solana.MustPublicKeyFromBase58("2fD7xjWXAyQE4PBFciCJobQoiwKhqHm2xYazzwt9eSYJ")     // Example pool address
	baseMint := solana.MustPublicKeyFromBase58("GkCfs9n8jLvuviiToewryCFa2rHQGKQv9FdnhTUEJtCH") // USDC mint address
	user := solana.MustPublicKeyFromBase58("FkyWzLirQxLRrNqGd1RtWNWDxymh9TftJSP8d8xeTtpB")     // User wallet address
	creator := solana.MustPublicKeyFromBase58("FkyWzLirQxLRrNqGd1RtWNWDxymh9TftJSP8d8xeTtpB")
	// Call the function
	ammAccounts, err := accountsCalc.CalculateAmmAccounts(pool, baseMint, user, creator)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, ammAccounts)
}

// TestFindCreatorVaultPDA tests the derivation of creator vault PDA
func TestFindCreatorVaultPDA(t *testing.T) {
	// Define test data
	creator := solana.MustPublicKeyFromBase58("F5X7fkeLhB1wELU1cnfgR4g4nDpLdA3J2QgAkfVQrxRy") // Example creator address

	// Define the PUMP_PROGRAM_ID (make sure this matches the one in your calculator package)
	PUMP_PROGRAM_ID := solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")

	// Derive the creator vault PDA
	creatorVaultPda, bump, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("creator-vault"),
			creator.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)

	// Assert that there was no error
	assert.NoError(t, err, "FindProgramAddress should not return an error")

	// Assert that the bump seed is within the expected range (0-255)
	assert.GreaterOrEqual(t, bump, uint8(0), "Bump seed should be >= 0")
	assert.LessOrEqual(t, bump, uint8(255), "Bump seed should be <= 255")

	// Assert that the PDA is not the zero address
	assert.NotEqual(t, solana.PublicKey{}, creatorVaultPda, "Creator vault PDA should not be the zero address")

	// Print the derived PDA and bump for debugging
	t.Logf("Creator vault PDA: %s, Bump: %d", creatorVaultPda.String(), bump)

	// Optional: If you know the expected PDA for this creator and program ID, you can assert it
	// expectedPDA := solana.MustPublicKeyFromBase58("expected_pda_address_here")
	// assert.Equal(t, expectedPDA, creatorVaultPda, "Creator vault PDA should match expected value")
}

// TestCalculateLaunchpadAccounts tests the CalculateLaunchpadAccounts function
func TestCalculateLaunchpadAccounts(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data
	user := solana.MustPublicKeyFromBase58("AwiFZDp7ee2QPbwUbGcpDSQ8hWz4a2qsNfqa13noY1EL")
	baseMint := solana.MustPublicKeyFromBase58("6XaLsZGcBN9nfHzsxGX5KL25bYbFn5n8NiCLSzRAbonk")
	quoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112") // WSOL

	// Call the function
	launchpadAccounts, err := accountsCalc.CalculateLaunchpadAccounts(user, baseMint, quoteMint, solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112"))

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, launchpadAccounts)

	// Verify some of the derived accounts
	RAYDIUM_LAUNCHPAD_PROGRAM_ID := solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj")

	// Expected PoolState
	expectedPoolState, _, _ := solana.FindProgramAddress(
		[][]byte{
			[]byte("pool"),
			baseMint.Bytes(),
			quoteMint.Bytes(),
		},
		RAYDIUM_LAUNCHPAD_PROGRAM_ID,
	)
	assert.Equal(t, expectedPoolState, launchpadAccounts.PoolState, "PoolState should match expected value")

	// Expected UserBaseTokenAccount
	expectedUserBaseTokenAccount, _, _ := solana.FindAssociatedTokenAddress(user, baseMint)
	assert.Equal(t, expectedUserBaseTokenAccount, launchpadAccounts.UserBaseTokenAccount, "UserBaseTokenAccount should match expected value")

	// Expected BaseVault
	expectedBaseVault, _, _ := solana.FindProgramAddress(
		[][]byte{
			[]byte("pool_vault"),
			launchpadAccounts.PoolState.Bytes(),
			baseMint.Bytes(),
		},
		RAYDIUM_LAUNCHPAD_PROGRAM_ID,
	)
	assert.Equal(t, expectedBaseVault, launchpadAccounts.BaseVault, "BaseVault should match expected value")

	t.Logf("Calculated Launchpad Accounts: %+v", launchpadAccounts)
}

func TestCalculateRaydiumCPMMAccounts(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data
	user := solana.MustPublicKeyFromBase58("DdVyu9TUfwZzeYfUtVnLJz15dQXnrZJdTMAyj55Dj6eP")
	pool := solana.MustPublicKeyFromBase58("FWzBQu58DedVoMbTybsDUNsCogQBsAvNCZNcj2NHmfrW")
	baseMint := solana.MustPublicKeyFromBase58("Cv8fyBXphgaPAxFBxzhSPeTwaAU6juSX8ZwMozZobonk")
	quoteMint := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112") // WSOL

	// Call the method to test
	cpmmAccounts, err := accountsCalc.CalculateRaydiumCPMMAccounts(user, "D4FPEruKEHrG5TenZ2mpDGEfu1iUvTiqBxvpU8HLBvC2", pool, baseMint, quoteMint)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, cpmmAccounts)

	t.Logf("Calculated Raydium CPMM Accounts: %+v", cpmmAccounts)
}

func TestParseCpmmTransaction(t *testing.T) {
	// Setup

	events, err := raydium_cp_swap.DecodeEventsInLogs([]string{"Program data: QMbN6CYIceIyagseqAZdpzjB83V/qL6/dVNoJq6VuGvJsAzzVbr5zzDOjnLVEQAAczPHwIkBAAAAt8jYAQAAAGZUqCgAAAAAAAAAAAAAAAAAAAAAAAAAAAE="})
	if err != nil {
		t.Errorf("解析 Universal Raydium CPMM事件失败: %v", err)
		return
	}
	t.Logf("解析到 Universal Raydium CPMM 事件: %+v", events)

	for _, event := range events {
		switch event.Name {
		case "SwapEvent":
			if tradeEvent, ok := event.Data.(*raydium_cp_swap.SwapEventEventData); ok {
				t.Logf("解析到 Universal Raydium CPMM 交易事件: %+v", tradeEvent)
			}
		case "LpChangeEvent":
			if poolCreateEvent, ok := event.Data.(*raydium_cp_swap.LpChangeEventEventData); ok {
				t.Logf("解析到 Universal Raydium CPMM 池创建事件: %+v", poolCreateEvent)
			}
		}
	}
}
