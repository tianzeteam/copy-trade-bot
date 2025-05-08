package test

import (
	"context"
	"testing"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"

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

	// Call the function
	ammAccounts, err := accountsCalc.CalculateAmmAccounts(context.Background(), pool, baseMint, user)

	// Assertions
	assert.NoError(t, err, "CalculateAmmAccounts should not return an error")
	assert.NotNil(t, ammAccounts, "AmmAccounts should not be nil")

	// Verify the returned account addresses
	assert.Equal(t, pool, ammAccounts.Pool, "Pool address should match input")
	assert.Equal(t, baseMint, ammAccounts.BaseMint, "Base mint address should match input")
	assert.Equal(t, baseMint, ammAccounts.Mint, "Mint address should match base mint")
	assert.Equal(t, solana.SolMint, ammAccounts.QuoteMint, "Quote mint should be SOL mint")
	assert.Equal(t, solana.MustPublicKeyFromBase58("BfdmiYercAetkBeg5Xjf47ezZpvkH8xpVncvNRELwzDZ"), ammAccounts.PoolBaseTokenAccount, "pool base account verify")
	assert.Equal(t, solana.MustPublicKeyFromBase58("WWPM82fYzGvZ8cdC3GtrX4DS3GGYA88XKdhuo75fqrX"), ammAccounts.PoolQuoteTokenAccount, "pool quote account verify")

	// Verify that the PDA derivations are correct
	// Note: We can't easily predict the exact values of the PDAs without reimplementing the derivation logic,
	// but we can check that they're not empty
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.PoolBaseTokenAccount, "Pool base token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.PoolQuoteTokenAccount, "Pool quote token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.UserBaseTokenAccount, "User base token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.UserQuoteTokenAccount, "User quote token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.WsolTokenAccount, "WSOL token account should not be empty")

	// Additional test: Verify that the user base token account is derived correctly
	expectedUserBaseTokenAccount, _, _ := solana.FindAssociatedTokenAddress(user, baseMint)
	assert.Equal(t, expectedUserBaseTokenAccount, ammAccounts.UserBaseTokenAccount, "User base token account should be correctly derived")
}

// TestCalculateAmmAccountsWithToken2022 tests the CalculateAmmAccounts function with a Token-2022 mint
func TestCalculateAmmAccountsWithToken2022(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://api.mainnet-beta.solana.com")

	// Test data with a Token-2022 mint
	pool := solana.MustPublicKeyFromBase58("9sCuqKYYZTofgBJsLdJCFEzHMiPyRKMDB3ncFZRoLGKe")     // Example pool address
	baseMint := solana.MustPublicKeyFromBase58("J1toso1uCk3RLmjorhTtrVwY9HJ7X8V9yYac6Y7kGCPn") // Example Token-2022 mint
	user := solana.MustPublicKeyFromBase58("63vEsMh9SEuxJEQL9tVbbHzHtymXkyt5ArL1peVRYyic")     // User wallet address

	// Call the function
	ammAccounts, err := accountsCalc.CalculateAmmAccounts(context.Background(), pool, baseMint, user)

	// Assertions
	assert.NoError(t, err, "CalculateAmmAccounts should not return an error")
	assert.NotNil(t, ammAccounts, "AmmAccounts should not be nil")

	// Verify the returned account addresses
	assert.Equal(t, pool, ammAccounts.Pool, "Pool address should match input")
	assert.Equal(t, baseMint, ammAccounts.BaseMint, "Base mint address should match input")
	assert.Equal(t, baseMint, ammAccounts.Mint, "Mint address should match base mint")
	assert.Equal(t, solana.SolMint, ammAccounts.QuoteMint, "Quote mint should be SOL mint")

	// Verify that the accounts are not empty
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.PoolBaseTokenAccount, "Pool base token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.PoolQuoteTokenAccount, "Pool quote token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.UserBaseTokenAccount, "User base token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.UserQuoteTokenAccount, "User quote token account should not be empty")
	assert.NotEqual(t, solana.PublicKey{}, ammAccounts.WsolTokenAccount, "WSOL token account should not be empty")
}
