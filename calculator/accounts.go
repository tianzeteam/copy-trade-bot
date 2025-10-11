package calculator

import (
	"context"
	//"copytrade/global"
	//"copytrade/types"

	"copytrade/helpers"
	"copytrade/logger"
	"copytrade/types"
	"fmt"

	"copytrade/pumpamm"
	"copytrade/raydium/layouts"
	"copytrade/raydium/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	// PumpAMMProgramID 是PumpAMM程序的ID
	PumpAMMProgramID = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")

	PUMP_PROGRAM_ID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")

	// RAYDIUM_V4_PROGRAM_ID is the program ID for Raydium V4
	RAYDIUM_V4_PROGRAM_ID = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")

	OPENBOOK_MARKET_PROGRAM_ID = solana.MustPublicKeyFromBase58("srmqPvymJeFKQ4zGQed1GFppgkRHL9kaELCbyksJtPX")

	// RAYDIUM_LAUNCHPAD_PROGRAM_ID is the program ID for the Raydium Launchpad
	RAYDIUM_LAUNCHPAD_PROGRAM_ID = solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj")
	RAYDIUM_CPMM_PROGRAM_ID      = solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")

	// METEORA_DBC_PROGRAM_ID is the program ID for Meteora DBC
	METEORA_DBC_PROGRAM_ID = solana.MustPublicKeyFromBase58("dbcij3LWUppWqq96dh6gJWwBifmcGfLSB5D4DuSMaqN")
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
func (c *AccountsCalculator) CalculatePumpAccounts(ctx context.Context, creator, mint, user solana.PublicKey) (*types.PumpAccounts, error) {
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

	// 计算creator vault PDA地址
	// 根据Pump程序的设计，creator vault PDA的seeds是 ["creator-vault", creator]
	creatorVaultPda, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("creator-vault"),
			creator.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("计算creator vault PDA失败: %w", err)
	}
	// 计算global_volume_accumulator (全局成交量累加器地址)
	globalVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("global_volume_accumulator"),
		},
		PUMP_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("计算global volume accumulator失败: %w", err)
	}
	// 计算user_volume_accumulator (用户成交量累加器地址)
	userVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("user_volume_accumulator"),
			user.Bytes(),
		},
		PUMP_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("计算user volume accumulator失败: %w", err)
	}

	return &types.PumpAccounts{
		BondingCurve:            bondingCurve,
		AssociatedBondingCurve:  associatedBondingCurve,
		AssociatedTokenAccount:  associatedTokenAccount,
		CreatorVaultPda:         creatorVaultPda,
		GlobalVolumeAccumulator: globalVolumeAccumulator,
		UserVolumeAccumulator:   userVolumeAccumulator,
	}, nil
}

func (c *AccountsCalculator) CalculateV4Accounts(user solana.PublicKey, tradeInfo *types.AmmRaydiumV4TradeInfo, marketLayout *types.MarketState, poolLayout *layouts.LIQUIDITY_STATE_LAYOUT_V4) (*types.RaydiumV4Accounts, error) {

	authority, _, _ := solana.FindProgramAddress([][]byte{{97, 109, 109, 32, 97, 117, 116, 104, 111, 114, 105, 116, 121}}, RAYDIUM_V4_PROGRAM_ID)
	//TODO
	marketAuthority, _, _ := utils.GetAssociatedAuthority(OPENBOOK_MARKET_PROGRAM_ID, marketLayout.MarketID)
	// 计算用户的基础代币associated token account
	userBaseTokenAccount, _, err := solana.FindAssociatedTokenAddress(
		user,
		tradeInfo.MintAddress,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find user base token account: %w", err)
	}
	userQuoteTokenAccount, _, err := solana.FindAssociatedTokenAddress(
		user,
		solana.SolMint,
	)
	if err != nil {
		logger.Errorf("failed to create WSOL account with seed: %v", err)
		return nil, fmt.Errorf("failed to create WSOL account with seed: %v", err)
	}

	return &types.RaydiumV4Accounts{
		Pool:                  tradeInfo.AmmPool,
		BaseMint:              tradeInfo.MintAddress,
		QuoteMint:             marketLayout.QuoteMint,
		UserBaseTokenAccount:  userBaseTokenAccount,
		UserQuoteTokenAccount: userQuoteTokenAccount,

		BaseDecimals:  uint64(poolLayout.BaseDecimal),
		QuoteDecimals: uint64(poolLayout.QuoteDecimal),

		Version:            4,
		ProgramId:          RAYDIUM_V4_PROGRAM_ID,
		OpenOrders:         tradeInfo.OpenOrders,
		TargetOrders:       tradeInfo.TargetOrders,
		BaseVault:          tradeInfo.BaseVault,
		QuoteVault:         tradeInfo.QuoteVault,
		MarketVersion:      3,
		Authority:          authority,
		MarketProgramId:    OPENBOOK_MARKET_PROGRAM_ID,
		MarketId:           marketLayout.MarketID,
		MarketAuthority:    marketAuthority,
		MarketBaseVault:    marketLayout.BaseVault,
		MarketQuoteVault:   marketLayout.QuoteVault,
		MarketBids:         marketLayout.Bids,
		MarketAsks:         marketLayout.Asks,
		MarketEventQueue:   marketLayout.MarketEventQueue,
		WithdrawQueue:      poolLayout.WithdrawQueue,
		LpVault:            poolLayout.LpVault,
		LpMint:             poolLayout.LpMint,
		LookupTableAccount: solana.PublicKey{},
	}, nil

}

// CalculateAmmAccounts 计算AMM所需的所有账户
func (c *AccountsCalculator) CalculateAmmAccounts(pool, baseMint, user solana.PublicKey, coinCreator solana.PublicKey) (*types.AmmAccounts, error) {

	// 创建WSOL账户
	wsolTokenAccount, _, err := solana.FindAssociatedTokenAddress(
		user,
		solana.SolMint,
	)
	if err != nil {
		logger.Errorf("failed to create WSOL account with seed: %v", err)
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

	// 计算coin_creator_vault_authority (代币创建者金库授权地址)
	// 根据IDL，seeds是 ["creator_vault", coin_creator]
	coinCreatorVaultAuthority, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("creator_vault"),
			coinCreator.Bytes(),
		},
		pumpamm.ProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find coin creator vault authority: %w", err)
	}

	// 计算coin_creator_vault_ata (代币创建者金库ATA地址)
	var seeds [][]byte
	// path: coinCreatorVaultAuthority
	seeds = append(seeds, coinCreatorVaultAuthority.Bytes())
	// path: quoteTokenProgram
	seeds = append(seeds, solana.TokenProgramID.Bytes())
	// path: quoteMint
	seeds = append(seeds, quoteMint.Bytes())
	coinCreatorVaultAta, _, err := solana.FindProgramAddress(
		seeds,
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find coin creator vault ata: %w", err)
	}

	// 计算global_volume_accumulator (全局成交量累加器地址)
	globalVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("global_volume_accumulator"),
		},
		pumpamm.ProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find global volume accumulator: %w", err)
	}

	// 计算user_volume_accumulator (用户成交量累加器地址)
	userVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("user_volume_accumulator"),
			user.Bytes(),
		},
		pumpamm.ProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find user volume accumulator: %w", err)
	}

	// 返回所有计算出的账户地址
	ammAccounts := &types.AmmAccounts{
		Pool:                      pool,
		Mint:                      baseMint,
		BaseMint:                  baseMint,
		QuoteMint:                 quoteMint,
		PoolBaseTokenAccount:      poolBaseTokenAccount,
		PoolQuoteTokenAccount:     poolQuoteTokenAccount,
		UserBaseTokenAccount:      userBaseTokenAccount,
		UserQuoteTokenAccount:     userQuoteTokenAccount,
		WsolTokenAccount:          wsolTokenAccount,
		CoinCreator:               coinCreator,
		CoinCreatorVaultAuthority: coinCreatorVaultAuthority,
		CoinCreatorVaultAta:       coinCreatorVaultAta,
		GlobalVolumeAccumulator:   globalVolumeAccumulator,
		UserVolumeAccumulator:     userVolumeAccumulator,
	}

	// 调试输出所有账户地址
	logger.Debugf("[DEBUG] CalculateAmmAccounts 结果:\n%s", ammAccounts)

	return ammAccounts, nil

}

func (c *AccountsCalculator) CalculateLaunchpadAccounts(user solana.PublicKey, baseMint, quoteMint solana.PublicKey, claimCreatorFee solana.PublicKey) (*types.RaydiumLaunchpadAccounts, error) {

	authority, _, err := solana.FindProgramAddress(
		[][]byte{
			{118, 97, 117, 108, 116, 95, 97, 117, 116, 104, 95, 115, 101, 101, 100}, // "vault_auth_seed"
		},
		RAYDIUM_LAUNCHPAD_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find authority PDA: %w", err)
	}

	poolState, _, err := solana.FindProgramAddress(
		[][]byte{
			{112, 111, 111, 108}, // "pool"
			baseMint.Bytes(),
			quoteMint.Bytes(),
		},
		RAYDIUM_LAUNCHPAD_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find pool state PDA: %w", err)
	}

	userBaseTokenAccount, _, err := solana.FindAssociatedTokenAddress(user, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to find user base token account: %w", err)
	}

	userQuoteTokenAccount, _, err := solana.FindAssociatedTokenAddress(user, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to find user quote token account: %w", err)
	}

	baseVault, _, err := solana.FindProgramAddress(
		[][]byte{
			{112, 111, 111, 108, 95, 118, 97, 117, 108, 116}, // "pool_vault"
			poolState.Bytes(),
			baseMint.Bytes(),
		},
		RAYDIUM_LAUNCHPAD_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find base vault PDA: %w", err)
	}

	quoteVault, _, err := solana.FindProgramAddress(
		[][]byte{
			{112, 111, 111, 108, 95, 118, 97, 117, 108, 116}, // "pool_vault"
			poolState.Bytes(),
			quoteMint.Bytes(),
		},
		RAYDIUM_LAUNCHPAD_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find quote vault PDA: %w", err)
	}

	eventAuthority, _, err := solana.FindProgramAddress(
		[][]byte{
			{95, 95, 101, 118, 101, 110, 116, 95, 97, 117, 116, 104, 111, 114, 105, 116, 121}, // "__event_authority"
		},
		RAYDIUM_LAUNCHPAD_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find event authority PDA: %w", err)
	}

	return &types.RaydiumLaunchpadAccounts{
		Payer:                 user,
		Authority:             authority,
		GlobalConfig:          solana.MustPublicKeyFromBase58("6s1xP3hpbAfFoNtUNF8mfHsjr2Bd97JxFJRWLbL6aHuX"), // This should be fetched from pool state
		PlatformConfig:        solana.MustPublicKeyFromBase58("FfYek5vEz23cMkWsdJwG2oa6EphsvXSHrGpdALN4g6W1"), // This should be fetched from pool state
		PoolState:             poolState,
		UserBaseTokenAccount:  userBaseTokenAccount,
		UserQuoteTokenAccount: userQuoteTokenAccount,
		BaseVault:             baseVault,
		QuoteVault:            quoteVault,
		BaseTokenMint:         baseMint,
		QuoteTokenMint:        quoteMint,
		BaseTokenProgram:      solana.TokenProgramID,
		QuoteTokenProgram:     solana.TokenProgramID,
		EventAuthority:        eventAuthority,
		Program:               RAYDIUM_LAUNCHPAD_PROGRAM_ID,
		ClaimPlatformFee:      solana.MustPublicKeyFromBase58("AMWjewk8jvQkqyrUv5cowmbQdoS2ChbJopXXPtVqKtch"),
		ClaimCreatorFee:       claimCreatorFee,
	}, nil
}

// CalculateRaydiumCPMMAccounts 计算Raydium CPMM账户
func (c *AccountsCalculator) CalculateRaydiumCPMMAccounts(user solana.PublicKey, ammConfig string, pool, baseMint, quoteMint solana.PublicKey) (*types.RaydiumCPMMAccounts, error) {

	inputTokenAccount, _, err := solana.FindAssociatedTokenAddress(user, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to find user input token account: %w", err)
	}

	outputTokenAccount, _, err := solana.FindAssociatedTokenAddress(user, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to find user output token account: %w", err)
	}

	inputVault, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("pool_vault"),
			pool.Bytes(),
			quoteMint.Bytes(),
		},
		RAYDIUM_CPMM_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find input vault PDA: %w", err)
	}

	outputVault, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("pool_vault"),
			pool.Bytes(),
			baseMint.Bytes(),
		},
		RAYDIUM_CPMM_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find output vault PDA: %w", err)
	}

	observationState, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("observation"),
			pool.Bytes(),
		},
		RAYDIUM_CPMM_PROGRAM_ID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find observation state PDA: %w", err)
	}

	return &types.RaydiumCPMMAccounts{
		Payer:              user,
		Authority:          solana.MustPublicKeyFromBase58("GpMZbSM2GgvTKHJirzeGfMFoaZ8UR2X7F4v8vHTvxFbL"),
		AmmConfig:          solana.MustPublicKeyFromBase58(ammConfig),
		PoolState:          pool,
		InputTokenAccount:  inputTokenAccount,
		OutputTokenAccount: outputTokenAccount,
		InputVault:         inputVault,
		OutputVault:        outputVault,
		ObservationState:   observationState,
		InputTokenMint:     quoteMint,
		OutputTokenMint:    baseMint,
		InputTokenProgram:  solana.TokenProgramID,
		OutputTokenProgram: solana.TokenProgramID,
		Program:            RAYDIUM_CPMM_PROGRAM_ID,
	}, nil
}

// CalculateMeteoraDbcAccounts 计算Meteora DBC账户
func (c *AccountsCalculator) CalculateMeteoraDbcAccounts(user solana.PublicKey, config string, baseMint, quoteMint solana.PublicKey) (*types.MeteoraDbcAccounts, error) {

	// 使用 helpers 包中的辅助函数派生账户
	configPubkey := solana.MustPublicKeyFromBase58(config)

	// 使用 DeriveDbcPoolPDA 派生 pool 地址
	pool := helpers.DeriveDbcPoolPDA(quoteMint, baseMint, configPubkey)

	// 派生 pool authority PDA
	poolAuthority := helpers.DerivePoolAuthorityPDA()

	eventAuthority := helpers.DeriveEventAuthorityPDA()

	// 计算用户的代币账户
	inputTokenAccount, _, err := solana.FindAssociatedTokenAddress(user, quoteMint)
	if err != nil {
		return nil, fmt.Errorf("failed to find user input token account: %w", err)
	}

	outputTokenAccount, _, err := solana.FindAssociatedTokenAddress(user, baseMint)
	if err != nil {
		return nil, fmt.Errorf("failed to find user output token account: %w", err)
	}

	// 使用 helpers.DeriveTokenVaultPDA 派生 vault 地址
	inputVault := helpers.DeriveTokenVaultPDA(pool, quoteMint)
	outputVault := helpers.DeriveTokenVaultPDA(pool, baseMint)

	// 使用 output token account 作为 referral account
	referralTokenAccount := METEORA_DBC_PROGRAM_ID

	return &types.MeteoraDbcAccounts{
		Payer:                user,
		Config:               configPubkey,
		PoolAuthority:        poolAuthority,
		Pool:                 pool,
		InputTokenAccount:    inputTokenAccount,
		OutputTokenAccount:   outputTokenAccount,
		InputVault:           inputVault,
		OutputVault:          outputVault,
		InputTokenMint:       quoteMint,
		OutputTokenMint:      baseMint,
		InputTokenProgram:    solana.TokenProgramID,
		OutputTokenProgram:   solana.TokenProgramID,
		Authority:            eventAuthority,
		ReferralTokenAccount: referralTokenAccount,
		Program:              METEORA_DBC_PROGRAM_ID,
	}, nil
}
