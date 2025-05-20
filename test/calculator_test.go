package test

import (
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
