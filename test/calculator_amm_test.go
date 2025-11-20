package test

import (
	"testing"

	"copytrade/calculator"

	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalculateAmmAccountsV2 测试 PumpAMM V2 账户计算
// 使用真实的 SPACECAT-WSOL 交易数据进行验证
func TestCalculateAmmAccountsV2(t *testing.T) {
	// 真实交易数据（示例）
	// Pool: Pump.fun AMM (SPACECAT-WSOL) Market
	pool := solana.MustPublicKeyFromBase58("EKqskknwdwGGnz8a8coxQrQ3RXgr9Lcs52NDEFAKpDbF")

	// Base Mint: SPACECAT
	baseMint := solana.MustPublicKeyFromBase58("9fLGtf1rRkYd6Dy93hvYh2r8T8Wy2EPa6WHC5ibgpump")

	// User: 交易发起者
	user := solana.MustPublicKeyFromBase58("22jkcdB9gFCZps5LYTvvpN4iVgMynEP7ci6QHwkTBPSx")

	// Coin Creator: 从链上数据获取
	coinCreator := solana.MustPublicKeyFromBase58("AuvwvLGyN55JRLE64fGMSYaQzTZR9UjikC68pJAG2mAJ")

	// 创建 AccountsCalculator
	calc := calculator.NewAccountsCalculator("https://api.mainnet-beta.solana.com")

	// 执行账户计算 (使用 v2 版本，因为测试数据是 Token2022)
	accounts, err := calc.CalculateAmmAccounts(pool, baseMint, user, coinCreator, "v2")
	require.NoError(t, err, "CalculateAmmAccounts should not return error")
	require.NotNil(t, accounts, "accounts should not be nil")

	// 验证基础字段
	t.Run("Basic Fields", func(t *testing.T) {
		assert.Equal(t, pool, accounts.Pool, "Pool address should match")
		assert.Equal(t, baseMint, accounts.BaseMint, "BaseMint should match")
		assert.Equal(t, baseMint, accounts.Mint, "Mint should match BaseMint")
		assert.Equal(t, solana.SolMint, accounts.QuoteMint, "QuoteMint should be SOL")
		assert.Equal(t, coinCreator, accounts.CoinCreator, "CoinCreator should match")
	})

	// 验证 Token Programs
	t.Run("Token Programs", func(t *testing.T) {
		assert.Equal(t, solana.Token2022ProgramID, accounts.BaseTokenProgram,
			"BaseTokenProgram should be Token2022")
		assert.Equal(t, solana.TokenProgramID, accounts.QuoteTokenProgram,
			"QuoteTokenProgram should be Token")
	})

	// 验证用户账户
	t.Run("User Accounts", func(t *testing.T) {
		// User Base Token Account: 2a9R1P5BrzhdBHoTmq18kSpWWwFw11iu8EpQtLvoki1A
		expectedUserBaseTokenAccount := solana.MustPublicKeyFromBase58("2a9R1P5BrzhdBHoTmq18kSpWWwFw11iu8EpQtLvoki1A")
		assert.Equal(t, expectedUserBaseTokenAccount, accounts.UserBaseTokenAccount,
			"UserBaseTokenAccount should match")

		// User Quote Token Account: HN1iJzZuNrPkCE3ofNUnBrkyJV9YLfxeqboxNxQdkSrq
		expectedUserQuoteTokenAccount := solana.MustPublicKeyFromBase58("HN1iJzZuNrPkCE3ofNUnBrkyJV9YLfxeqboxNxQdkSrq")
		assert.Equal(t, expectedUserQuoteTokenAccount, accounts.UserQuoteTokenAccount,
			"UserQuoteTokenAccount should match")

		// WSOL Token Account 应该与 UserQuoteTokenAccount 相同
		assert.Equal(t, expectedUserQuoteTokenAccount, accounts.WsolTokenAccount,
			"WsolTokenAccount should match UserQuoteTokenAccount")
	})

	// 验证池账户
	t.Run("Pool Accounts", func(t *testing.T) {
		// Pool Base Token Account: Pump.fun AMM (SPACECAT-WSOL) Pool 1
		expectedPoolBaseTokenAccount := solana.MustPublicKeyFromBase58("E68dzkPW5o3rKEmhkSXv29xhjFg4BPqdtPknnM6V5vVs")
		assert.Equal(t, expectedPoolBaseTokenAccount, accounts.PoolBaseTokenAccount,
			"PoolBaseTokenAccount should match")

		// Pool Quote Token Account: Pump.fun AMM (SPACECAT-WSOL) Pool 2
		expectedPoolQuoteTokenAccount := solana.MustPublicKeyFromBase58("6KEkonB9F9oi2B1G1aVK5KGZpB4e91vjwYXgibjioQc6")
		assert.Equal(t, expectedPoolQuoteTokenAccount, accounts.PoolQuoteTokenAccount,
			"PoolQuoteTokenAccount should match")
	})

	// 验证协议费用相关账户
	t.Run("Protocol Fee Accounts", func(t *testing.T) {
		// Protocol Fee Recipient: Pump.fun AMM: Protocol Fee 4
		expectedProtocolFeeRecipient := solana.MustPublicKeyFromBase58("9rPYyANsfQZw3DnDmKE3YCQF5E8oD89UXoHn9JFEhJUz")
		assert.Equal(t, expectedProtocolFeeRecipient, accounts.ProtocolFeeRecipient,
			"ProtocolFeeRecipient should match")

		// Protocol Fee Recipient Token Account: Bvtgim23rfocUzxVX9j9QFxTbBnH8JZxnaGLCEkXvjKS
		expectedProtocolFeeRecipientTokenAccount := solana.MustPublicKeyFromBase58("Bvtgim23rfocUzxVX9j9QFxTbBnH8JZxnaGLCEkXvjKS")
		assert.Equal(t, expectedProtocolFeeRecipientTokenAccount, accounts.ProtocolFeeRecipientTokenAccount,
			"ProtocolFeeRecipientTokenAccount should match")
	})

	// 验证 Coin Creator 相关账户
	t.Run("Coin Creator Accounts", func(t *testing.T) {
		// Coin Creator Vault Authority: BGE3J6hPxPzCnzd1Nmn3bxrTQwqyRYd4h9gmWQQktcQ3
		expectedCoinCreatorVaultAuthority := solana.MustPublicKeyFromBase58("BGE3J6hPxPzCnzd1Nmn3bxrTQwqyRYd4h9gmWQQktcQ3")
		assert.Equal(t, expectedCoinCreatorVaultAuthority, accounts.CoinCreatorVaultAuthority,
			"CoinCreatorVaultAuthority should match")

		// Coin Creator Vault Ata: 6dWJbwG5YAwApTK6a71zRDFgvVcNLJyX2koFTtm4S1aY
		expectedCoinCreatorVaultAta := solana.MustPublicKeyFromBase58("6dWJbwG5YAwApTK6a71zRDFgvVcNLJyX2koFTtm4S1aY")
		assert.Equal(t, expectedCoinCreatorVaultAta, accounts.CoinCreatorVaultAta,
			"CoinCreatorVaultAta should match")
	})

	// 验证全局配置和事件授权
	t.Run("Global Config and Event Authority", func(t *testing.T) {
		// Global Config: ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw
		expectedGlobalConfig := solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw")
		assert.Equal(t, expectedGlobalConfig, accounts.GlobalConfig,
			"GlobalConfig should match")

		// Event Authority: GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR
		expectedEventAuthority := solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR")
		assert.Equal(t, expectedEventAuthority, accounts.EventAuthority,
			"EventAuthority should match")
	})

	// 验证成交量累加器
	t.Run("Volume Accumulators", func(t *testing.T) {
		// Global Volume Accumulator: C2aFPdENg4A2HQsmrd5rTw5TaYBX5Ku887cWjbFKtZpw
		expectedGlobalVolumeAccumulator := solana.MustPublicKeyFromBase58("C2aFPdENg4A2HQsmrd5rTw5TaYBX5Ku887cWjbFKtZpw")
		assert.Equal(t, expectedGlobalVolumeAccumulator, accounts.GlobalVolumeAccumulator,
			"GlobalVolumeAccumulator should match")

		// User Volume Accumulator: 7z54CpwRxVNzqu8cwgCT9EfcYUY7jLDhXzVr7K36YVmK
		expectedUserVolumeAccumulator := solana.MustPublicKeyFromBase58("7z54CpwRxVNzqu8cwgCT9EfcYUY7jLDhXzVr7K36YVmK")
		assert.Equal(t, expectedUserVolumeAccumulator, accounts.UserVolumeAccumulator,
			"UserVolumeAccumulator should match")
	})

	// 验证费用配置
	t.Run("Fee Config", func(t *testing.T) {
		// Fee Config: 5PHirr8joyTMp9JMm6nW7hNDVyEYdkzDqazxPD7RaTjx
		expectedFeeConfig := solana.MustPublicKeyFromBase58("5PHirr8joyTMp9JMm6nW7hNDVyEYdkzDqazxPD7RaTjx")
		assert.Equal(t, expectedFeeConfig, accounts.FeeConfig,
			"FeeConfig should match")

		// Fee Program: pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ
		expectedFeeProgram := solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")
		assert.Equal(t, expectedFeeProgram, accounts.FeeProgram,
			"FeeProgram should match")
	})

	// 验证所有账户都不为空
	t.Run("No Empty Accounts", func(t *testing.T) {
		assert.NotEqual(t, solana.PublicKey{}, accounts.Pool, "Pool should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.BaseMint, "BaseMint should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.QuoteMint, "QuoteMint should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.PoolBaseTokenAccount, "PoolBaseTokenAccount should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.PoolQuoteTokenAccount, "PoolQuoteTokenAccount should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.UserBaseTokenAccount, "UserBaseTokenAccount should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.UserQuoteTokenAccount, "UserQuoteTokenAccount should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.GlobalConfig, "GlobalConfig should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.ProtocolFeeRecipient, "ProtocolFeeRecipient should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.EventAuthority, "EventAuthority should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.FeeConfig, "FeeConfig should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.FeeProgram, "FeeProgram should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.BaseTokenProgram, "BaseTokenProgram should not be empty")
		assert.NotEqual(t, solana.PublicKey{}, accounts.QuoteTokenProgram, "QuoteTokenProgram should not be empty")
	})
}

// TestCalculateAmmAccountsPDADerivation 测试 PDA 推导的正确性
func TestCalculateAmmAccountsPDADerivation(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("5VVJt6rHh9FdYLhxvXJ8KvWOFbA7vZPvJzXYmEPUKLqL")
	baseMint := solana.MustPublicKeyFromBase58("HeLp6NuQkmYB4pYWo2zYs22mESHXPQYzXbB8n4V98jwC")
	user := solana.MustPublicKeyFromBase58("22jkcdB9gFCZps5LYTvvpN4iVgMynEP7ci6QHwkTBPSx")
	coinCreator := solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")

	calc := calculator.NewAccountsCalculator("https://api.mainnet-beta.solana.com")

	accounts, err := calc.CalculateAmmAccounts(pool, baseMint, user, coinCreator, "v2")
	require.NoError(t, err)

	// 验证 PDA 推导
	t.Run("Pool Base Token Account PDA", func(t *testing.T) {
		// 手动推导 PDA
		derivedPDA, _, err := solana.FindProgramAddress(
			[][]byte{
				pool.Bytes(),
				solana.Token2022ProgramID.Bytes(),
				baseMint.Bytes(),
			},
			solana.SPLAssociatedTokenAccountProgramID,
		)
		require.NoError(t, err)
		assert.Equal(t, derivedPDA, accounts.PoolBaseTokenAccount,
			"PoolBaseTokenAccount PDA derivation should match")
	})

	t.Run("Pool Quote Token Account PDA", func(t *testing.T) {
		derivedPDA, _, err := solana.FindProgramAddress(
			[][]byte{
				pool.Bytes(),
				solana.TokenProgramID.Bytes(),
				solana.SolMint.Bytes(),
			},
			solana.SPLAssociatedTokenAccountProgramID,
		)
		require.NoError(t, err)
		assert.Equal(t, derivedPDA, accounts.PoolQuoteTokenAccount,
			"PoolQuoteTokenAccount PDA derivation should match")
	})

	t.Run("Coin Creator Vault Authority PDA", func(t *testing.T) {
		pumpAMMProgramID := solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
		derivedPDA, _, err := solana.FindProgramAddress(
			[][]byte{
				[]byte("creator_vault"),
				coinCreator.Bytes(),
			},
			pumpAMMProgramID,
		)
		require.NoError(t, err)
		assert.Equal(t, derivedPDA, accounts.CoinCreatorVaultAuthority,
			"CoinCreatorVaultAuthority PDA derivation should match")
	})

	t.Run("Global Config PDA", func(t *testing.T) {
		pumpAMMProgramID := solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
		derivedPDA, _, err := solana.FindProgramAddress(
			[][]byte{
				[]byte("global_config"),
			},
			pumpAMMProgramID,
		)
		require.NoError(t, err)
		assert.Equal(t, derivedPDA, accounts.GlobalConfig,
			"GlobalConfig PDA derivation should match")
	})

	t.Run("Event Authority PDA", func(t *testing.T) {
		pumpAMMProgramID := solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
		derivedPDA, _, err := solana.FindProgramAddress(
			[][]byte{
				[]byte("__event_authority"),
			},
			pumpAMMProgramID,
		)
		require.NoError(t, err)
		assert.Equal(t, derivedPDA, accounts.EventAuthority,
			"EventAuthority PDA derivation should match")
	})

	t.Run("Global Volume Accumulator PDA", func(t *testing.T) {
		pumpAMMProgramID := solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
		derivedPDA, _, err := solana.FindProgramAddress(
			[][]byte{
				[]byte("global_volume_accumulator"),
			},
			pumpAMMProgramID,
		)
		require.NoError(t, err)
		assert.Equal(t, derivedPDA, accounts.GlobalVolumeAccumulator,
			"GlobalVolumeAccumulator PDA derivation should match")
	})

	t.Run("User Volume Accumulator PDA", func(t *testing.T) {
		pumpAMMProgramID := solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
		derivedPDA, _, err := solana.FindProgramAddress(
			[][]byte{
				[]byte("user_volume_accumulator"),
				user.Bytes(),
			},
			pumpAMMProgramID,
		)
		require.NoError(t, err)
		assert.Equal(t, derivedPDA, accounts.UserVolumeAccumulator,
			"UserVolumeAccumulator PDA derivation should match")
	})
}

// TestCalculateAmmAccountsWithDifferentUsers 测试不同用户的账户计算
func TestCalculateAmmAccountsWithDifferentUsers(t *testing.T) {
	pool := solana.MustPublicKeyFromBase58("5VVJt6rHh9FdYLhxvXJ8KvWOFbA7vZPvJzXYmEPUKLqL")
	baseMint := solana.MustPublicKeyFromBase58("HeLp6NuQkmYB4pYWo2zYs22mESHXPQYzXbB8n4V98jwC")
	coinCreator := solana.MustPublicKeyFromBase58("CebN5WGQ4jvEPvsVU4EoHEpgzq1VV7AbicfhtW4xC9iM")

	calc := calculator.NewAccountsCalculator("https://api.mainnet-beta.solana.com")

	// 测试两个不同的用户
	user1 := solana.MustPublicKeyFromBase58("22jkcdB9gFCZps5LYTvvpN4iVgMynEP7ci6QHwkTBPSx")
	user2 := solana.MustPublicKeyFromBase58("3XYZ1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZabcde")

	accounts1, err1 := calc.CalculateAmmAccounts(pool, baseMint, user1, coinCreator, "v2")
	require.NoError(t, err1)

	accounts2, err2 := calc.CalculateAmmAccounts(pool, baseMint, user2, coinCreator, "v2")
	require.NoError(t, err2)

	// 验证用户特定的账户不同
	t.Run("User Specific Accounts Should Differ", func(t *testing.T) {
		assert.NotEqual(t, accounts1.UserBaseTokenAccount, accounts2.UserBaseTokenAccount,
			"Different users should have different UserBaseTokenAccount")
		assert.NotEqual(t, accounts1.UserQuoteTokenAccount, accounts2.UserQuoteTokenAccount,
			"Different users should have different UserQuoteTokenAccount")
		assert.NotEqual(t, accounts1.UserVolumeAccumulator, accounts2.UserVolumeAccumulator,
			"Different users should have different UserVolumeAccumulator")
	})

	// 验证池相关的账户相同
	t.Run("Pool Accounts Should Be Same", func(t *testing.T) {
		assert.Equal(t, accounts1.Pool, accounts2.Pool,
			"Pool should be same for different users")
		assert.Equal(t, accounts1.PoolBaseTokenAccount, accounts2.PoolBaseTokenAccount,
			"PoolBaseTokenAccount should be same for different users")
		assert.Equal(t, accounts1.PoolQuoteTokenAccount, accounts2.PoolQuoteTokenAccount,
			"PoolQuoteTokenAccount should be same for different users")
		assert.Equal(t, accounts1.GlobalConfig, accounts2.GlobalConfig,
			"GlobalConfig should be same for different users")
		assert.Equal(t, accounts1.GlobalVolumeAccumulator, accounts2.GlobalVolumeAccumulator,
			"GlobalVolumeAccumulator should be same for different users")
	})
}
