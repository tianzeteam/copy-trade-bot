package calculator

import (
	"context"
	"copytrade/global"

	"copytrade/helpers"
	"copytrade/logger"
	pumpV3 "copytrade/pumpV3"
	"copytrade/types"
	"fmt"
	"time"

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

// CalculatePumpV3Accounts 派生统一交易接口（buy_v2 / sell_v2）所需的全部账户。
//
//	creator          代币创建者钱包（BondingCurve.creator）
//	mint             base 代币 mint
//	user             交易发起者（机器人钱包）
//	baseTokenProgram base 代币所属程序：V1 币为 SPL Token，V2 币为 Token-2022
//	quoteMint        计价 mint：SOL 配对传 wSOL，USDC 配对传 USDC（指令参数语义）
//	feeRecipient     手续费接收地址。复用触发事件的地址即可——链上程序只校验其在全局名单内。
//
// SOL 配对币无需预创建用户的 quote ATA（associated_quote_user 仅做 seed 约束）。
func (c *AccountsCalculator) CalculatePumpV3Accounts(_ context.Context, creator, mint, user, baseTokenProgram, quoteMint, feeRecipient solana.PublicKey) (*types.PumpV3Accounts, error) {
	quoteTokenProgram := pumpV3.QuoteTokenProgram(quoteMint)

	bondingCurve, err := pumpV3.BondingCurvePDA(mint)
	if err != nil {
		return nil, fmt.Errorf("计算 bonding curve 失败: %w", err)
	}
	if creator.IsZero() {
		return nil, fmt.Errorf("Pump 曲线 creator 尚未缓存")
	}
	creatorVault, err := pumpV3.CreatorVaultPDA(creator)
	if err != nil {
		return nil, fmt.Errorf("计算 creator vault 失败: %w", err)
	}
	sharingConfig, err := pumpV3.SharingConfigPDA(mint)
	if err != nil {
		return nil, fmt.Errorf("计算 sharing config 失败: %w", err)
	}
	globalVolumeAccumulator, err := pumpV3.GlobalVolumeAccumulatorPDA()
	if err != nil {
		return nil, fmt.Errorf("计算 global volume accumulator 失败: %w", err)
	}
	userVolumeAccumulator, err := pumpV3.UserVolumeAccumulatorPDA(user)
	if err != nil {
		return nil, fmt.Errorf("计算 user volume accumulator 失败: %w", err)
	}
	feeConfigAddr, err := pumpV3.FeeConfigPDA()
	if err != nil {
		return nil, fmt.Errorf("计算 fee config 失败: %w", err)
	}

	associatedBaseBondingCurve, err := pumpV3.AssociatedTokenAccount(bondingCurve, mint, baseTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算曲线 base ATA 失败: %w", err)
	}
	associatedQuoteBondingCurve, err := pumpV3.AssociatedTokenAccount(bondingCurve, quoteMint, quoteTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算曲线 quote ATA 失败: %w", err)
	}
	associatedBaseUser, err := pumpV3.AssociatedTokenAccount(user, mint, baseTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算用户 base ATA 失败: %w", err)
	}
	associatedQuoteUser, err := pumpV3.AssociatedTokenAccount(user, quoteMint, quoteTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算用户 quote ATA 失败: %w", err)
	}
	associatedCreatorVault, err := pumpV3.AssociatedTokenAccount(creatorVault, quoteMint, quoteTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算 creator vault quote ATA 失败: %w", err)
	}
	associatedQuoteFeeRecipient, err := pumpV3.AssociatedTokenAccount(feeRecipient, quoteMint, quoteTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算 fee recipient quote ATA 失败: %w", err)
	}
	associatedUserVolumeAccumulator, err := pumpV3.AssociatedTokenAccount(userVolumeAccumulator, quoteMint, quoteTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算 UVA quote ATA 失败: %w", err)
	}

	// buyback recipient 名单只从已预热的 Global 缓存读取；热路径不访问 RPC。
	_, globalConfig := global.GetPumpFeeConfig()
	if globalConfig == nil {
		return nil, fmt.Errorf("Pump Global 配置尚未预热")
	}
	buybackPool := globalConfig.AllBuybackFeeRecipients()
	if len(buybackPool) == 0 {
		return nil, fmt.Errorf("链上 Global 未提供 buyback fee recipient，无法构建泵交易")
	}
	idx := int(time.Now().UnixNano())
	if idx < 0 {
		idx = -idx
	}
	buybackFeeRecipient := buybackPool[idx%len(buybackPool)]
	associatedQuoteBuybackFeeRecipient, err := pumpV3.AssociatedTokenAccount(buybackFeeRecipient, quoteMint, quoteTokenProgram)
	if err != nil {
		return nil, fmt.Errorf("计算 buyback recipient quote ATA 失败: %w", err)
	}

	return &types.PumpV3Accounts{
		BaseMint:                           mint,
		QuoteMint:                          quoteMint,
		BaseTokenProgram:                   baseTokenProgram,
		QuoteTokenProgram:                  quoteTokenProgram,
		AssociatedTokenProgram:             solana.SPLAssociatedTokenAccountProgramID,
		FeeRecipient:                       feeRecipient,
		AssociatedQuoteFeeRecipient:        associatedQuoteFeeRecipient,
		BuybackFeeRecipient:                buybackFeeRecipient,
		AssociatedQuoteBuybackFeeRecipient: associatedQuoteBuybackFeeRecipient,
		BondingCurve:                       bondingCurve,
		AssociatedBaseBondingCurve:         associatedBaseBondingCurve,
		AssociatedQuoteBondingCurve:        associatedQuoteBondingCurve,
		User:                               user,
		AssociatedBaseUser:                 associatedBaseUser,
		AssociatedQuoteUser:                associatedQuoteUser,
		CreatorVault:                       creatorVault,
		AssociatedCreatorVault:             associatedCreatorVault,
		SharingConfig:                      sharingConfig,
		GlobalVolumeAccumulator:            globalVolumeAccumulator,
		UserVolumeAccumulator:              userVolumeAccumulator,
		AssociatedUserVolumeAccumulator:    associatedUserVolumeAccumulator,
		FeeConfigAccount:                   feeConfigAddr,
		FeeProgram:                         pumpV3.FeeProgramID,
		EventAuthority:                     pumpV3.EventAuthority,
		Program:                            pumpV3.ProgramID,
	}, nil
}

// FetchPumpBondingCurve 拉取并解码内盘曲线账户（含 creator / quote_mint /
// mayhem / 供应量等字段），用于事件里缺失这些信息的场景（如 ShredStream 路径、
// mayhem 币的真实供应量）。
func (c *AccountsCalculator) FetchPumpBondingCurve(ctx context.Context, bondingCurve solana.PublicKey) (*pumpV3.BondingCurve, error) {
	data, err := getPumpAccountData(ctx, c.client, bondingCurve)
	if err != nil {
		return nil, err
	}
	return DecodePumpBondingCurve(data)
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

// CalculateAmmAccounts 计算 PumpAMM V2 所需的所有账户
func (c *AccountsCalculator) CalculateAmmAccounts(pool, baseMint, user solana.PublicKey, coinCreator solana.PublicKey, pumpVersion string) (*types.AmmAccounts, error) {

	// 创建WSOL账户
	wsolTokenAccount, _, err := solana.FindAssociatedTokenAddress(
		user,
		solana.SolMint,
	)
	if err != nil {
		logger.Errorf("failed to create WSOL account with seed: %v", err)
		return nil, fmt.Errorf("failed to create WSOL account with seed: %v", err)
	}

	// Quote mint 固定为 SOL
	quoteMint := solana.SolMint

	// 根据 pumpVersion 判断 Token Program
	// PumpAMM V1: 使用标准 Token Program
	// PumpAMM V2: 使用 Token2022 Program
	var baseTokenProgram solana.PublicKey
	if pumpVersion == "v2" {
		baseTokenProgram = solana.Token2022ProgramID
	} else {
		// v1 或其他版本使用标准 Token Program
		baseTokenProgram = solana.TokenProgramID
	}
	quoteTokenProgram := solana.TokenProgramID // SOL 使用标准 Token Program

	// 计算 pool_base_token_account (池中基础代币账户)
	// 根据 IDL: PDA with seeds [pool, base_token_program, base_mint], program = ATA Program
	poolBaseTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			pool.Bytes(),
			baseTokenProgram.Bytes(),
			baseMint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find pool base token account: %w", err)
	}

	// 计算 pool_quote_token_account (池中报价代币账户)
	// 根据 IDL: PDA with seeds [pool, quote_token_program, quote_mint], program = ATA Program
	poolQuoteTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			pool.Bytes(),
			quoteTokenProgram.Bytes(),
			quoteMint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find pool quote token account: %w", err)
	}

	// 计算用户的基础代币associated token account
	// 对于 Token2022 代币，需要使用 Token2022ProgramID
	userBaseTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			user.Bytes(),
			baseTokenProgram.Bytes(),
			baseMint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
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

	// 计算 coin_creator_vault_authority (代币创建者金库授权地址)
	// 根据 IDL: PDA with seeds ["creator_vault", coin_creator], program = PumpAMM Program
	coinCreatorVaultAuthority, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("creator_vault"),
			coinCreator.Bytes(),
		},
		PumpAMMProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find coin creator vault authority: %w", err)
	}

	// 计算 coin_creator_vault_ata (代币创建者金库ATA地址)
	// 根据 IDL: PDA with seeds [coin_creator_vault_authority, quote_token_program, quote_mint], program = ATA Program
	coinCreatorVaultAta, _, err := solana.FindProgramAddress(
		[][]byte{
			coinCreatorVaultAuthority.Bytes(),
			quoteTokenProgram.Bytes(),
			quoteMint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find coin creator vault ata: %w", err)
	}

	// 计算 global_config (全局配置地址)
	// 根据 IDL: PDA with seeds ["global_config"], program = PumpAMM Program
	globalConfig, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("global_config"),
		},
		PumpAMMProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find global config: %w", err)
	}

	// 计算 protocol_fee_recipient (协议费用接收者地址)
	// 这是从 global_config 中读取的真实链上数据
	// Pump.fun AMM: Protocol Fee 4
	protocolFeeRecipient := solana.MustPublicKeyFromBase58("9rPYyANsfQZw3DnDmKE3YCQF5E8oD89UXoHn9JFEhJUz")

	// 计算 protocol_fee_recipient_token_account (协议费用接收者代币账户)
	// 根据 IDL: PDA with seeds [protocol_fee_recipient, quote_token_program, quote_mint], program = ATA Program
	protocolFeeRecipientTokenAccount, _, err := solana.FindProgramAddress(
		[][]byte{
			protocolFeeRecipient.Bytes(),
			quoteTokenProgram.Bytes(),
			quoteMint.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find protocol fee recipient token account: %w", err)
	}

	// 计算 event_authority (事件授权地址)
	// 根据 IDL: PDA with seeds ["__event_authority"], program = PumpAMM Program
	eventAuthority, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("__event_authority"),
		},
		PumpAMMProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find event authority: %w", err)
	}

	// fee_program 地址（固定）
	feeProgram := solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ")

	// 计算 fee_config (费用配置地址)
	// 根据 IDL: PDA with seeds ["fee_config", specific_pubkey], program = fee_program
	specificPubkey := []byte{12, 20, 222, 252, 130, 94, 198, 118, 148, 37, 8, 24, 187, 101, 64, 101, 244, 41, 141, 49, 86, 213, 113, 180, 212, 248, 9, 12, 24, 233, 168, 99}
	feeConfig, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("fee_config"),
			specificPubkey,
		},
		feeProgram,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find fee config: %w", err)
	}

	// 计算 global_volume_accumulator (全局成交量累加器地址)
	// 根据 IDL: PDA with seeds ["global_volume_accumulator"], program = PumpAMM Program
	globalVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("global_volume_accumulator"),
		},
		PumpAMMProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find global volume accumulator: %w", err)
	}

	// 计算 user_volume_accumulator (用户成交量累加器地址)
	// 根据 IDL: PDA with seeds ["user_volume_accumulator", user], program = PumpAMM Program
	userVolumeAccumulator, _, err := solana.FindProgramAddress(
		[][]byte{
			[]byte("user_volume_accumulator"),
			user.Bytes(),
		},
		PumpAMMProgramID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find user volume accumulator: %w", err)
	}

	// 返回所有计算出的账户地址
	ammAccounts := &types.AmmAccounts{
		Pool:                             pool,
		Mint:                             baseMint,
		BaseMint:                         baseMint,
		QuoteMint:                        quoteMint,
		PoolBaseTokenAccount:             poolBaseTokenAccount,
		PoolQuoteTokenAccount:            poolQuoteTokenAccount,
		UserBaseTokenAccount:             userBaseTokenAccount,
		UserQuoteTokenAccount:            userQuoteTokenAccount,
		WsolTokenAccount:                 wsolTokenAccount,
		CoinCreator:                      coinCreator,
		CoinCreatorVaultAuthority:        coinCreatorVaultAuthority,
		CoinCreatorVaultAta:              coinCreatorVaultAta,
		GlobalVolumeAccumulator:          globalVolumeAccumulator,
		UserVolumeAccumulator:            userVolumeAccumulator,
		GlobalConfig:                     globalConfig,
		ProtocolFeeRecipient:             protocolFeeRecipient,
		ProtocolFeeRecipientTokenAccount: protocolFeeRecipientTokenAccount,
		EventAuthority:                   eventAuthority,
		FeeConfig:                        feeConfig,
		FeeProgram:                       feeProgram,
		BaseTokenProgram:                 baseTokenProgram,
		QuoteTokenProgram:                quoteTokenProgram,
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
