package test

import (
	"context"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"

	"copytrade/calculator"
)

// TestCalculatePumpV2Accounts_TokenProgram 测试使用标准 Token Program 的 Pump V2 账户计算
func TestCalculatePumpV2Accounts_TokenProgram(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data - 使用真实的链上交易数据
	// 交易来源: Pump.fun buy transaction with Token-2022
	creator := solana.MustPublicKeyFromBase58("HmgSia9qke4yYRRYQaXFs6dj4Nt7BSu4UNbaWymsTeS6")
	mint := solana.MustPublicKeyFromBase58("HRwsEGCErPEDDGFZAbXdezhmimjLi2Ge18GodUUwpump") // eye token
	user := solana.MustPublicKeyFromBase58("cwhihmydRGfmG9BX5i5Ame26NBtzYZbTX1BgoV2X43v")
	tokenProgram := solana.Token2022ProgramID // Token-2022 Program (from transaction)

	// 从链上交易获取的预期账户地址
	expectedAssociatedUser := solana.MustPublicKeyFromBase58("6id4Fkm5hbyHsEALxqHo4x7qnfbsRVPt3goj4g3yTSnu")
	expectedCreatorVault := solana.MustPublicKeyFromBase58("BhnSW3H1t7Bh2mVf77L5KFhUzYxn3eKYmQqd2D9qAd67")
	expectedGlobalVolumeAccumulator := solana.MustPublicKeyFromBase58("Hq2wp8uJ9jCPsYgNHex8RtqdvMPfVGoYwjvF1ATiwn2Y")
	expectedUserVolumeAccumulator := solana.MustPublicKeyFromBase58("3r7LTaSH8QkJppauDuxGANmxXh6FgdDCXZ5Jwb3ZKgDF")

	// Call the function
	ctx := context.Background()
	pumpAccounts, err := accountsCalc.CalculatePumpV2Accounts(ctx, creator, mint, user, tokenProgram)

	// Assertions
	assert.NoError(t, err, "CalculatePumpV2Accounts should not return an error")
	assert.NotNil(t, pumpAccounts, "PumpAccounts should not be nil")

	// Verify all accounts are not zero addresses
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.BondingCurve, "BondingCurve should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.AssociatedBondingCurve, "AssociatedBondingCurve should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.AssociatedTokenAccount, "AssociatedTokenAccount should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.CreatorVaultPda, "CreatorVaultPda should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.GlobalVolumeAccumulator, "GlobalVolumeAccumulator should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.UserVolumeAccumulator, "UserVolumeAccumulator should not be zero address")

	// Verify against actual on-chain transaction data
	assert.Equal(t, expectedAssociatedUser, pumpAccounts.AssociatedTokenAccount,
		"AssociatedTokenAccount should match on-chain value from transaction")
	assert.Equal(t, expectedCreatorVault, pumpAccounts.CreatorVaultPda,
		"CreatorVaultPda should match on-chain value from transaction")
	assert.Equal(t, expectedGlobalVolumeAccumulator, pumpAccounts.GlobalVolumeAccumulator,
		"GlobalVolumeAccumulator should match on-chain value from transaction")
	assert.Equal(t, expectedUserVolumeAccumulator, pumpAccounts.UserVolumeAccumulator,
		"UserVolumeAccumulator should match on-chain value from transaction")

	// Verify BondingCurve derivation
	PUMP_PROGRAM_ID := solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	expectedBondingCurve, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("bonding-curve"),
			mint.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedBondingCurve, pumpAccounts.BondingCurve, "BondingCurve should match expected derivation")

	// Verify AssociatedBondingCurve derivation (V2 uses ATA Program)
	expectedAssociatedBondingCurve, _, err := solana.FindProgramAddress(
		[][]byte{
			pumpAccounts.BondingCurve.Bytes(),
			tokenProgram.Bytes(),
			mint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedAssociatedBondingCurve, pumpAccounts.AssociatedBondingCurve, "AssociatedBondingCurve should match expected derivation")

	// Verify AssociatedTokenAccount derivation (V2 uses ATA Program)
	expectedAssociatedTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			user.Bytes(),
			tokenProgram.Bytes(),
			mint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedAssociatedTokenAccount, pumpAccounts.AssociatedTokenAccount, "AssociatedTokenAccount should match expected derivation")

	// Verify CreatorVaultPda derivation
	expectedCreatorVaultPda, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("creator-vault"),
			creator.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedCreatorVaultPda, pumpAccounts.CreatorVaultPda, "CreatorVaultPda should match expected derivation")

	// Verify GlobalVolumeAccumulator derivation
	derivedGlobalVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("global_volume_accumulator"),
		},
		PUMP_PROGRAM_ID,
	)
	assert.NoError(t, err)
	assert.Equal(t, derivedGlobalVolumeAccumulator, pumpAccounts.GlobalVolumeAccumulator, "GlobalVolumeAccumulator should match expected derivation")

	// Verify UserVolumeAccumulator derivation
	derivedUserVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("user_volume_accumulator"),
			user.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)
	assert.NoError(t, err)
	assert.Equal(t, derivedUserVolumeAccumulator, pumpAccounts.UserVolumeAccumulator, "UserVolumeAccumulator should match expected derivation")

	t.Logf("Calculated Pump V2 Accounts (Token Program): %+v", pumpAccounts)
}

// TestCalculatePumpV2Accounts_Token2022Program 测试使用 Token-2022 Program 的 Pump V2 账户计算
func TestCalculatePumpV2Accounts_Token2022Program(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data
	creator := solana.MustPublicKeyFromBase58("HmgSia9qke4yYRRYQaXFs6dj4Nt7BSu4UNbaWymsTeS6")
	mint := solana.MustPublicKeyFromBase58("HRwsEGCErPEDDGFZAbXdezhmimjLi2Ge18GodUUwpump") // eye token
	user := solana.MustPublicKeyFromBase58("cwhihmydRGfmG9BX5i5Ame26NBtzYZbTX1BgoV2X43v")
	tokenProgram := solana.Token2022ProgramID // Token-2022 Program

	// Call the function
	ctx := context.Background()
	pumpAccounts, err := accountsCalc.CalculatePumpV2Accounts(ctx, creator, mint, user, tokenProgram)

	// Assertions
	assert.NoError(t, err, "CalculatePumpV2Accounts should not return an error")
	assert.NotNil(t, pumpAccounts, "PumpAccounts should not be nil")

	// Verify all accounts are not zero addresses
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.BondingCurve, "BondingCurve should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.AssociatedBondingCurve, "AssociatedBondingCurve should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.AssociatedTokenAccount, "AssociatedTokenAccount should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.CreatorVaultPda, "CreatorVaultPda should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.GlobalVolumeAccumulator, "GlobalVolumeAccumulator should not be zero address")
	assert.NotEqual(t, solana.PublicKey{}, pumpAccounts.UserVolumeAccumulator, "UserVolumeAccumulator should not be zero address")

	// Verify BondingCurve derivation (same for both Token Program and Token-2022)
	PUMP_PROGRAM_ID := solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	expectedBondingCurve, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("bonding-curve"),
			mint.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedBondingCurve, pumpAccounts.BondingCurve, "BondingCurve should match expected derivation")

	// Verify AssociatedBondingCurve derivation with Token-2022 Program
	expectedAssociatedBondingCurve, _, err := solana.FindProgramAddress(
		[][]byte{
			pumpAccounts.BondingCurve.Bytes(),
			tokenProgram.Bytes(),
			mint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedAssociatedBondingCurve, pumpAccounts.AssociatedBondingCurve, "AssociatedBondingCurve should match expected derivation with Token-2022")

	// Verify AssociatedTokenAccount derivation with Token-2022 Program
	expectedAssociatedTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			user.Bytes(),
			tokenProgram.Bytes(),
			mint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	assert.NoError(t, err)
	assert.Equal(t, expectedAssociatedTokenAccount, pumpAccounts.AssociatedTokenAccount, "AssociatedTokenAccount should match expected derivation with Token-2022")

	t.Logf("Calculated Pump V2 Accounts (Token-2022 Program): %+v", pumpAccounts)
}

// TestCalculatePumpV2Accounts_DifferentUsers 测试不同用户的账户计算
func TestCalculatePumpV2Accounts_DifferentUsers(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data
	creator := solana.MustPublicKeyFromBase58("F5X7fkeLhB1wELU1cnfgR4g4nDpLdA3J2QgAkfVQrxRy")
	mint := solana.MustPublicKeyFromBase58("GkCfs9n8jLvuviiToewryCFa2rHQGKQv9FdnhTUEJtCH")
	user1 := solana.MustPublicKeyFromBase58("FkyWzLirQxLRrNqGd1RtWNWDxymh9TftJSP8d8xeTtpB")
	user2 := solana.MustPublicKeyFromBase58("AwiFZDp7ee2QPbwUbGcpDSQ8hWz4a2qsNfqa13noY1EL")
	tokenProgram := solana.TokenProgramID

	ctx := context.Background()

	// Calculate accounts for user1
	accounts1, err1 := accountsCalc.CalculatePumpV2Accounts(ctx, creator, mint, user1, tokenProgram)
	assert.NoError(t, err1)
	assert.NotNil(t, accounts1)

	// Calculate accounts for user2
	accounts2, err2 := accountsCalc.CalculatePumpV2Accounts(ctx, creator, mint, user2, tokenProgram)
	assert.NoError(t, err2)
	assert.NotNil(t, accounts2)

	// BondingCurve, CreatorVaultPda, and GlobalVolumeAccumulator should be the same for both users
	assert.Equal(t, accounts1.BondingCurve, accounts2.BondingCurve, "BondingCurve should be the same for different users")
	assert.Equal(t, accounts1.AssociatedBondingCurve, accounts2.AssociatedBondingCurve, "AssociatedBondingCurve should be the same for different users")
	assert.Equal(t, accounts1.CreatorVaultPda, accounts2.CreatorVaultPda, "CreatorVaultPda should be the same for different users")
	assert.Equal(t, accounts1.GlobalVolumeAccumulator, accounts2.GlobalVolumeAccumulator, "GlobalVolumeAccumulator should be the same for different users")

	// AssociatedTokenAccount and UserVolumeAccumulator should be different for different users
	assert.NotEqual(t, accounts1.AssociatedTokenAccount, accounts2.AssociatedTokenAccount, "AssociatedTokenAccount should be different for different users")
	assert.NotEqual(t, accounts1.UserVolumeAccumulator, accounts2.UserVolumeAccumulator, "UserVolumeAccumulator should be different for different users")

	t.Logf("User1 Accounts: %+v", accounts1)
	t.Logf("User2 Accounts: %+v", accounts2)
}

// TestCalculatePumpV2Accounts_DifferentMints 测试不同代币的账户计算
func TestCalculatePumpV2Accounts_DifferentMints(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data
	creator := solana.MustPublicKeyFromBase58("F5X7fkeLhB1wELU1cnfgR4g4nDpLdA3J2QgAkfVQrxRy")
	mint1 := solana.MustPublicKeyFromBase58("GkCfs9n8jLvuviiToewryCFa2rHQGKQv9FdnhTUEJtCH")
	mint2 := solana.MustPublicKeyFromBase58("6XaLsZGcBN9nfHzsxGX5KL25bYbFn5n8NiCLSzRAbonk")
	user := solana.MustPublicKeyFromBase58("FkyWzLirQxLRrNqGd1RtWNWDxymh9TftJSP8d8xeTtpB")
	tokenProgram := solana.TokenProgramID

	ctx := context.Background()

	// Calculate accounts for mint1
	accounts1, err1 := accountsCalc.CalculatePumpV2Accounts(ctx, creator, mint1, user, tokenProgram)
	assert.NoError(t, err1)
	assert.NotNil(t, accounts1)

	// Calculate accounts for mint2
	accounts2, err2 := accountsCalc.CalculatePumpV2Accounts(ctx, creator, mint2, user, tokenProgram)
	assert.NoError(t, err2)
	assert.NotNil(t, accounts2)

	// All accounts should be different for different mints except CreatorVaultPda, GlobalVolumeAccumulator, and UserVolumeAccumulator
	assert.NotEqual(t, accounts1.BondingCurve, accounts2.BondingCurve, "BondingCurve should be different for different mints")
	assert.NotEqual(t, accounts1.AssociatedBondingCurve, accounts2.AssociatedBondingCurve, "AssociatedBondingCurve should be different for different mints")
	assert.NotEqual(t, accounts1.AssociatedTokenAccount, accounts2.AssociatedTokenAccount, "AssociatedTokenAccount should be different for different mints")

	// CreatorVaultPda, GlobalVolumeAccumulator, and UserVolumeAccumulator should be the same (they don't depend on mint)
	assert.Equal(t, accounts1.CreatorVaultPda, accounts2.CreatorVaultPda, "CreatorVaultPda should be the same for different mints")
	assert.Equal(t, accounts1.GlobalVolumeAccumulator, accounts2.GlobalVolumeAccumulator, "GlobalVolumeAccumulator should be the same for different mints")
	assert.Equal(t, accounts1.UserVolumeAccumulator, accounts2.UserVolumeAccumulator, "UserVolumeAccumulator should be the same for different mints (same user)")

	t.Logf("Mint1 Accounts: %+v", accounts1)
	t.Logf("Mint2 Accounts: %+v", accounts2)
}

// TestCalculatePumpV2Accounts_CompareWithV1 测试 V2 与 V1 的区别
func TestCalculatePumpV2Accounts_CompareWithV1(t *testing.T) {
	// Setup
	accountsCalc := calculator.NewAccountsCalculator("https://mainnet.helius-rpc.com/?api-key=fc8353eb-13cb-423e-8d36-49e2b26d3a78")

	// Test data
	creator := solana.MustPublicKeyFromBase58("F5X7fkeLhB1wELU1cnfgR4g4nDpLdA3J2QgAkfVQrxRy")
	mint := solana.MustPublicKeyFromBase58("GkCfs9n8jLvuviiToewryCFa2rHQGKQv9FdnhTUEJtCH")
	user := solana.MustPublicKeyFromBase58("FkyWzLirQxLRrNqGd1RtWNWDxymh9TftJSP8d8xeTtpB")
	tokenProgram := solana.TokenProgramID

	ctx := context.Background()

	// Calculate V1 accounts
	accountsV1, err1 := accountsCalc.CalculatePumpAccounts(ctx, creator, mint, user)
	assert.NoError(t, err1)
	assert.NotNil(t, accountsV1)

	// Calculate V2 accounts with standard Token Program
	accountsV2, err2 := accountsCalc.CalculatePumpV2Accounts(ctx, creator, mint, user, tokenProgram)
	assert.NoError(t, err2)
	assert.NotNil(t, accountsV2)

	// BondingCurve should be the same
	assert.Equal(t, accountsV1.BondingCurve, accountsV2.BondingCurve, "BondingCurve should be the same in V1 and V2")

	// AssociatedBondingCurve and AssociatedTokenAccount may be different due to different derivation methods
	// V1 uses FindAssociatedTokenAddress, V2 uses FindProgramAddress with ATA Program
	t.Logf("V1 AssociatedBondingCurve: %s", accountsV1.AssociatedBondingCurve)
	t.Logf("V2 AssociatedBondingCurve: %s", accountsV2.AssociatedBondingCurve)
	t.Logf("V1 AssociatedTokenAccount: %s", accountsV1.AssociatedTokenAccount)
	t.Logf("V2 AssociatedTokenAccount: %s", accountsV2.AssociatedTokenAccount)

	// CreatorVaultPda, GlobalVolumeAccumulator, and UserVolumeAccumulator should be the same
	assert.Equal(t, accountsV1.CreatorVaultPda, accountsV2.CreatorVaultPda, "CreatorVaultPda should be the same in V1 and V2")
	assert.Equal(t, accountsV1.GlobalVolumeAccumulator, accountsV2.GlobalVolumeAccumulator, "GlobalVolumeAccumulator should be the same in V1 and V2")
	assert.Equal(t, accountsV1.UserVolumeAccumulator, accountsV2.UserVolumeAccumulator, "UserVolumeAccumulator should be the same in V1 and V2")

	t.Logf("V1 Accounts: %+v", accountsV1)
	t.Logf("V2 Accounts: %+v", accountsV2)
}
