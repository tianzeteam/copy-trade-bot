package core

import (
	"context"

	//"copytrade/calculator"
	//"copytrade/core"
	"copytrade/calculator"
	"copytrade/constants"
	"copytrade/global"

	"copytrade/logger"
	pumpV3 "copytrade/pumpV3"
	"copytrade/pumpammV2"
	"copytrade/raydium"
	"copytrade/raydium_cp_swap"

	"copytrade/raydium_launchpad"
	"copytrade/swqos"
	"copytrade/types"
	"copytrade/utils"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	associatedtokenaccount "copytrade/helpers/associatedtokenaccount"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// 定义Pump AMM常量
var ()

// Trader 定义交易接口
type Trader interface {
	Buy(ctx context.Context, params BuyParams) (string, error) // 返回交易签名
	Sell(ctx context.Context, params SellParams) (string, error)
}

// TraderImpl 交易实现
type TraderImpl struct {
	Client       *rpc.Client
	Wallet       *solana.Wallet
	AccountsCalc *calculator.AccountsCalculator
	AmountsCalc  *calculator.AmountsCalculator
	Config       *utils.Config // 添加全局配置
}

// NewTrader 创建交易实例
func NewTrader(endpoint string, privateKey solana.PrivateKey, config *utils.Config) *TraderImpl {
	// 使用正确的 RPC URL 创建客户端
	client := rpc.New(endpoint)
	// 设置请求头，避免一些 RPC 节点的限制

	return &TraderImpl{
		Client:       client,
		Wallet:       &solana.Wallet{PrivateKey: privateKey},
		AccountsCalc: calculator.NewAccountsCalculator(endpoint),
		AmountsCalc:  calculator.NewAmountsCalculator(endpoint),
		Config:       config,
	}
}

// buildPumpV3TradeAccounts 把 calculator 派生的账户集合转成指令构造器需要的结构。
// Global / SystemProgram 为协议固定地址，直接在此补齐。
func buildPumpV3TradeAccounts(acc *types.PumpV3Accounts) pumpV3.TradeAccounts {
	return pumpV3.TradeAccounts{
		Global:                             pumpV3.GlobalAccount,
		BaseMint:                           acc.BaseMint,
		QuoteMint:                          acc.QuoteMint,
		BaseTokenProgram:                   acc.BaseTokenProgram,
		QuoteTokenProgram:                  acc.QuoteTokenProgram,
		AssociatedTokenProgram:             acc.AssociatedTokenProgram,
		FeeRecipient:                       acc.FeeRecipient,
		AssociatedQuoteFeeRecipient:        acc.AssociatedQuoteFeeRecipient,
		BuybackFeeRecipient:                acc.BuybackFeeRecipient,
		AssociatedQuoteBuybackFeeRecipient: acc.AssociatedQuoteBuybackFeeRecipient,
		BondingCurve:                       acc.BondingCurve,
		AssociatedBaseBondingCurve:         acc.AssociatedBaseBondingCurve,
		AssociatedQuoteBondingCurve:        acc.AssociatedQuoteBondingCurve,
		User:                               acc.User,
		AssociatedBaseUser:                 acc.AssociatedBaseUser,
		AssociatedQuoteUser:                acc.AssociatedQuoteUser,
		CreatorVault:                       acc.CreatorVault,
		AssociatedCreatorVault:             acc.AssociatedCreatorVault,
		SharingConfig:                      acc.SharingConfig,
		GlobalVolumeAccumulator:            acc.GlobalVolumeAccumulator,
		UserVolumeAccumulator:              acc.UserVolumeAccumulator,
		AssociatedUserVolumeAccumulator:    acc.AssociatedUserVolumeAccumulator,
		FeeConfig:                          acc.FeeConfigAccount,
		FeeProgram:                         acc.FeeProgram,
		SystemProgram:                      solana.SystemProgramID,
		EventAuthority:                     acc.EventAuthority,
		Program:                            acc.Program,
	}
}

// Buy 执行买入交易
func (t *TraderImpl) Buy(ctx context.Context, params types.OrderParams) (string, error) {

	logger.Infof("开始执行买入交易...")
	logger.Infof("Mint地址: %s, nonce账户: %s,bot钱包: %s", params.TradeParams.Accounts.Mint, params.TradeParams.Accounts.NonceAccount, params.TradeParams.Accounts.User)
	logger.Infof("购买数量: %d", params.TradeParams.Amount)
	logger.Infof("构建指令时间: %v", time.Now().Format("2006-01-02 15:04:05.000000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 构建交易指令
	instructions := []solana.Instruction{}

	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)
	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(params.TradeParams.FeeConfig.ComputeUnits))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// Pump V3 使用幂等 ATA 创建，不在组装阶段查询账户状态。
	if v3Accounts := params.TradeParams.Accounts.PumpV3; v3Accounts != nil {
		baseTokenProgram := v3Accounts.BaseTokenProgram
		if baseTokenProgram.IsZero() {
			baseTokenProgram = solana.TokenProgramID
			if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
				baseTokenProgram = solana.Token2022ProgramID
			}
		}
		baseAtaBuilder := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(botWallet.PublicKey()).
			SetWallet(botWallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			SetTokenProgram(baseTokenProgram)
		if baseTokenProgram.Equals(solana.Token2022ProgramID) {
			instructions = append(instructions, baseAtaBuilder.BuildV2())
		} else {
			instructions = append(instructions, baseAtaBuilder.Build())
		}

		// 非 SOL quote（当前为主网 USDC）需要机器人的 quote ATA；已存在时幂等跳过。
		if !pumpV3.IsSolLikeQuoteMint(v3Accounts.QuoteMint) {
			quoteTokenProgram := v3Accounts.QuoteTokenProgram
			if quoteTokenProgram.IsZero() {
				quoteTokenProgram = solana.TokenProgramID
			}
			quoteAtaBuilder := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(botWallet.PublicKey()).
				SetWallet(botWallet.PublicKey()).
				SetMint(v3Accounts.QuoteMint).
				SetTokenProgram(quoteTokenProgram)
			if quoteTokenProgram.Equals(solana.Token2022ProgramID) {
				instructions = append(instructions, quoteAtaBuilder.BuildV2())
			} else {
				instructions = append(instructions, quoteAtaBuilder.Build())
			}
		}
	} else if t.Config.SkipATACheck {
		// 兼容旧调用：V3 主链路不会进入这个分支。
		logger.Infof("快速模式：跳过ATA检查，直接创建ATA账户")
		ataBuilder := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(botWallet.PublicKey()).
			SetWallet(botWallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint)
		if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
			instructions = append(instructions, ataBuilder.BuildV2())
		} else {
			instructions = append(instructions, ataBuilder.Build())
		}
	} else {
		// 兼容旧调用：仅非 V3 路径保留显式 ATA 查询。
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.AssociatedTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA账户不存在，创建新账户...")
			ataBuilder := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(botWallet.PublicKey()).
				SetWallet(botWallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint)
			if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
				instructions = append(instructions, ataBuilder.BuildV2())
			} else {
				instructions = append(instructions, ataBuilder.Build())
			}
		}
	}

	// 统一交易接口：V1 / V2 币一律走 buy_v2，差异只体现在 token program 与 ATA 类型上。
	if params.TradeParams.Accounts.PumpV3 == nil {
		logger.Errorf("Pump V3 账户未初始化")
		return "", fmt.Errorf("pump V3 账户未初始化")
	}
	buildInstruction, err := pumpV3.NewBuyV2Instruction(
		params.TradeParams.Amount,
		params.TradeParams.Cost,
		buildPumpV3TradeAccounts(params.TradeParams.Accounts.PumpV3),
	)
	if err != nil {
		logger.Errorf("构建 Pump buy_v2 指令失败: %v", err)
		return "", err
	}

	instructions = append(instructions, buildInstruction)

	// 创建通道
	resultChan := make(chan string, 10)
	errChan := make(chan error, 10)
	timeout := time.After(30 * time.Second)
	logger.Infof("构建指令时间结束: %v", time.Now().Format("2006-01-02 15:04:05.000000"))

	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("Astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[Blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("Blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Debugf("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")

		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("买入交易已发送! 交易ID: %s", sig)
		return sig, nil
	case err := <-errChan:
		logger.Errorf("pump 买入交易失败 %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

// pump Amm Buy 执行买入交易
func (t *TraderImpl) PumpAmmBuy(ctx context.Context, params types.OrderParams) (string, error) {

	logger.Infof("开始执行Pump Amm买入交易...")
	logger.Infof("Mint地址: %s", params.TradeParams.Accounts.Mint)
	logger.Infof("购买数量: %d,最大支付金额: %.6f", params.TradeParams.Amount, float64(params.TradeParams.Cost)/1e9)
	logger.Infof("time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 构建交易指令
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(params.TradeParams.FeeConfig.ComputeUnits))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// 创建 ATA 指令（参考 Buy 方法，支持 V2 Token-2022）
	var baseTokenProgramID = solana.TokenProgramID
	if t.Config.SkipATACheck {
		// 快速模式：直接添加创建ATA指令
		logger.Infof("快速模式：跳过ATA检查，直接创建ATA账户")
		ataBuilder := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(botWallet.PublicKey()).
			SetWallet(botWallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint)

		var createATAIx *associatedtokenaccount.Instruction

		if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
			// V2: 使用 BuildV2() 创建 Token-2022 ATA
			baseTokenProgramID = solana.Token2022ProgramID
			createATAIx = ataBuilder.BuildV2()
			logger.Infof("使用 BuildV2() 创建 Token-2022 ATA")
		} else {
			// V1: 使用标准 Build() 创建 Token ATA

			createATAIx = ataBuilder.Build()
			logger.Infof("使用 Build() 创建标准 Token ATA")
		}

		instructions = append(instructions, createATAIx)
	} else {
		// 安全模式：先检查再创建
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.AssociatedTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA账户不存在，创建新账户...")
			ataBuilder := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(botWallet.PublicKey()).
				SetWallet(botWallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint)

			var createATAIx *associatedtokenaccount.Instruction
			if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
				// V2: 使用 BuildV2() 创建 Token-2022 ATA
				baseTokenProgramID = solana.Token2022ProgramID
				createATAIx = ataBuilder.BuildV2()
				logger.Infof("使用 BuildV2() 创建 Token-2022 ATA")
			} else {
				// V1: 使用标准 Build() 创建 Token ATA
				createATAIx = ataBuilder.Build()
				logger.Infof("使用 Build() 创建标准 Token ATA")
			}

			instructions = append(instructions, createATAIx)
		}
	}
	logger.Info("PumpAmm Buy: Idempotent模式创建并初始化WSOL账户...")
	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(botWallet.PublicKey()).
		SetWallet(botWallet.PublicKey()).
		SetMint(WSOL).
		Build()
	// 初始化WSOL账户指令
	instructions = append(instructions, createWsolAccountInstruction)

	// 将SOL转入WSOL账户
	transferIx := system.NewTransferInstruction(
		params.TradeParams.Cost,
		botWallet.PublicKey(),
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, transferIx)

	// 同步WSOL余额
	syncNativeIx := token.NewSyncNativeInstruction(
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, syncNativeIx)

	// 使用 pumpammV2 模块创建买入指令
	buyInstruction, err := pumpammV2.NewBuyInstruction(
		params.TradeParams.Amount,                                                      // baseAmountOut
		params.TradeParams.Cost,                                                        // maxQuoteAmountIn
		params.TradeParams.Accounts.Pool,                                               // pool
		params.TradeParams.Accounts.User,                                               // user
		GLOBAL_CONFIG_ACCOUNT,                                                          // globalConfig
		params.TradeParams.Accounts.BaseMint,                                           // baseMint
		params.TradeParams.Accounts.QuoteMint,                                          // quoteMint
		params.TradeParams.Accounts.UserBaseTokenAccount,                               // userBaseTokenAccount
		params.TradeParams.Accounts.UserQuoteTokenAccount,                              // userQuoteTokenAccount
		params.TradeParams.Accounts.PoolBaseTokenAccount,                               // poolBaseTokenAccount
		params.TradeParams.Accounts.PoolQuoteTokenAccount,                              // poolQuoteTokenAccount
		PROTOCOL_FEE_RECIPIENT,                                                         // protocolFeeRecipient
		PROTOCOL_FEE_RECIPIENT_TOKEN_ACCOUNT,                                           // protocolFeeRecipientTokenAccount
		baseTokenProgramID,                                                             // baseTokenProgram
		solana.TokenProgramID,                                                          // quoteTokenProgram
		solana.SystemProgramID,                                                         // systemProgram
		solana.SPLAssociatedTokenAccountProgramID,                                      // associatedTokenProgram
		AMM_EVENT_AUTHORITY,                                                            // eventAuthority
		pumpammV2.ProgramID,                                                            // program
		params.TradeParams.Accounts.CreatorVault,                                       // coinCreatorVaultAta
		params.TradeParams.Accounts.CoinCreatorVaultAuthority,                          // coinCreatorVaultAuthority
		params.TradeParams.Accounts.GlobalVolumeAccumulator,                            // globalVolumeAccumulator
		params.TradeParams.Accounts.UserVolumeAccumulator,                              // userVolumeAccumulator
		solana.MustPublicKeyFromBase58("5PHirr8joyTMp9JMm6nW7hNDVyEYdkzDqazxPD7RaTjx"), // feeConfig
		solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ"),  // feeProgram
	)
	if err != nil {
		logger.Errorf("创建买入指令失败: %v", err)
		return "", fmt.Errorf("创建买入指令失败: %w", err)
	}

	instructions = append(instructions, buyInstruction)

	// 关闭WSOL账户
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		params.TradeParams.Accounts.UserQuoteTokenAccount, // 要关闭的账户
		botWallet.PublicKey(),                             // 接收剩余SOL的地址
		botWallet.PublicKey(),                             // 关闭账户的所有者
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[Blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("Blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("买入交易已发送! 交易ID: %s", sig)
		return sig, nil
	case err := <-errChan:
		logger.Errorf("买入交易已发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

// raydium pool v4 Buy 执行买入交易
func (t *TraderImpl) RaydiumV4Buy(ctx context.Context, params types.OrderParams) (string, error) {

	if t.Config.SimulateMode {
		logger.Infof("模拟模式，不执行买入交易 %s", params.TradeParams.Accounts.Mint)
		return "mock", fmt.Errorf("模拟模式，不执行买入交易")
	}
	logger.Infof("开始执行买入交易...")
	logger.Infof("Mint地址: %s", params.TradeParams.Accounts.Mint)
	logger.Infof("购买数量: %d", params.TradeParams.Amount)
	logger.Infof("time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 构建交易指令
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	if t.Config.SkipATACheck {
		// 快速模式：直接添加创建ATA指令
		logger.Infof("快速模式：跳过ATA检查，直接创建ATA账户")
		createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(botWallet.PublicKey()).
			SetWallet(botWallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			Build()
		instructions = append(instructions, createATAIx)
	} else {
		// 安全模式：先检查再创建
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.AssociatedTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA账户不存在，创建新账户...")
			createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(botWallet.PublicKey()).
				SetWallet(botWallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint).
				Build()
			instructions = append(instructions, createATAIx)
		}
	}
	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(botWallet.PublicKey()).
		SetWallet(botWallet.PublicKey()).
		SetMint(WSOL).
		Build()
	// 初始化WSOL账户指令
	instructions = append(instructions, createWsolAccountInstruction)

	// 将SOL转入WSOL账户
	transferIx := system.NewTransferInstruction(
		params.TradeParams.Cost,
		botWallet.PublicKey(),
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, transferIx)

	// 同步WSOL余额
	syncNativeIx := token.NewSyncNativeInstruction(
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, syncNativeIx)

	// 构建Buy指令
	// 使用pumpamm包的正确API
	buyInstruction := raydium.NewSwapInstructionBuilder().
		SetProgramAccount(RAYDIUM_V4_PROGRAM_ID).
		SetTokenProgramAccount(solana.TokenProgramID).
		SetPoolAccount(params.TradeParams.Accounts.RaydiumV4Accounts.Pool).
		SetAuthorityAccount(params.TradeParams.Accounts.RaydiumV4Accounts.Authority).
		SetOpenOrders(params.TradeParams.Accounts.RaydiumV4Accounts.OpenOrders).
		SetTargetOrders(params.TradeParams.Accounts.RaydiumV4Accounts.TargetOrders).
		SetBaseVault(params.TradeParams.Accounts.RaydiumV4Accounts.BaseVault).
		SetQuoteVault(params.TradeParams.Accounts.RaydiumV4Accounts.QuoteVault).
		SetMarketProgramId(params.TradeParams.Accounts.RaydiumV4Accounts.MarketProgramId).
		SetMarketId(params.TradeParams.Accounts.RaydiumV4Accounts.MarketId).
		SetMarketBids(params.TradeParams.Accounts.RaydiumV4Accounts.MarketAsks).
		SetMarketAsks(params.TradeParams.Accounts.RaydiumV4Accounts.MarketAsks).
		SetMarketEventQueue(params.TradeParams.Accounts.RaydiumV4Accounts.MarketEventQueue).
		SetMarketBaseVault(params.TradeParams.Accounts.RaydiumV4Accounts.MarketBaseVault).
		SetMarketQuoteVault(params.TradeParams.Accounts.RaydiumV4Accounts.MarketQuoteVault).
		SetMarketAuthority(params.TradeParams.Accounts.RaydiumV4Accounts.MarketAuthority).
		SetUserBaseTokenAccountAccount(params.TradeParams.Accounts.UserBaseTokenAccount).
		SetUserQuoteTokenAccountAccount(params.TradeParams.Accounts.RaydiumV4Accounts.UserQuoteTokenAccount).
		SetUserAccount(params.TradeParams.Accounts.User).
		SetBaseAmountIn(params.TradeParams.Amount).
		SetMinQuoteAmountOut(params.TradeParams.Cost)

	instructions = append(instructions, buyInstruction.Build())

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("Astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[Blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("Blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("买入交易已发送! 交易ID: %s", sig)
		return sig, nil
	case err := <-errChan:
		logger.Errorf("买入交易已发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

func (t *TraderImpl) RaydiumLaunchpadSell(ctx context.Context, params types.OrderParams) (string, error) {

	mintAddress := params.TradeParams.Accounts.Mint.String()
	logger.Infof("开始执行卖出交易...")
	logger.Infof("Mint地址: %s", mintAddress)
	logger.Infof("卖出数量: %d", params.TradeParams.Amount)
	logger.Infof("time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 检查是否可以卖出（3秒内同一个币最多卖出一次）
	if !global.CanSellToken(mintAddress) {
		logger.Warnf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
		return "", fmt.Errorf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
	}
	// 构建交易指令
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(botWallet.PublicKey()).
		SetWallet(botWallet.PublicKey()).
		SetMint(WSOL).
		Build()

	instructions = append(instructions, createWsolAccountInstruction)

	// 构建sell指令
	// 使用raydium_launchpad包的正确API
	sellInstruction := raydium_launchpad.NewSellExactInInstructionBuilder().
		SetAmountIn(params.TradeParams.Amount).
		SetMinimumAmountOut(params.TradeParams.Cost).
		SetProgramAccount(RAYDIUM_LAUNCHPAD_PROGRAM_ID).
		SetPayerAccount(botWallet.PublicKey()).
		SetAuthorityAccount(params.Accounts.LaunchpadAccounts.Authority).
		SetGlobalConfigAccount(params.Accounts.LaunchpadAccounts.GlobalConfig).
		SetPlatformConfigAccount(params.Accounts.LaunchpadAccounts.PlatformConfig).
		SetPoolStateAccount(params.Accounts.LaunchpadAccounts.PoolState).
		SetUserBaseTokenAccount(params.Accounts.LaunchpadAccounts.UserBaseTokenAccount).
		SetUserQuoteTokenAccount(params.Accounts.LaunchpadAccounts.UserQuoteTokenAccount).
		SetBaseVaultAccount(params.Accounts.LaunchpadAccounts.BaseVault).
		SetQuoteVaultAccount(params.Accounts.LaunchpadAccounts.QuoteVault).
		SetBaseTokenMintAccount(params.Accounts.LaunchpadAccounts.BaseTokenMint).
		SetQuoteTokenMintAccount(params.Accounts.LaunchpadAccounts.QuoteTokenMint).
		SetBaseTokenProgramAccount(solana.TokenProgramID).
		SetQuoteTokenProgramAccount(solana.TokenProgramID).
		SetEventAuthorityAccount(params.Accounts.LaunchpadAccounts.EventAuthority).
		SetSystemProgramAccount(solana.SystemProgramID).
		SetClaimPlatformFeeAccount(params.Accounts.LaunchpadAccounts.ClaimPlatformFee).
		SetClaimCreatorFeeAccount(params.Accounts.LaunchpadAccounts.ClaimCreatorFee)

	instructions = append(instructions, sellInstruction.Build())

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	// 检查各个模式是否启用并准备发送
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Debugf("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("卖出交易已发送! 交易ID: %s", sig)
		global.UpdateTokenSellTime(mintAddress)
		return sig, nil
	case err := <-errChan:
		logger.Errorf("卖出交易已发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

// raydium pool v4 Sell 执行买入交易
func (t *TraderImpl) RaydiumV4Sell(ctx context.Context, params types.OrderParams) (string, error) {

	mintAddress := params.TradeParams.Accounts.Mint.String()
	logger.Infof("开始执行卖出交易...")
	logger.Infof("Mint地址: %s", mintAddress)
	logger.Infof("卖出数量: %d", params.TradeParams.Amount)
	logger.Infof("time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 检查是否可以卖出（3秒内同一个币最多卖出一次）
	if !global.CanSellToken(mintAddress) {
		logger.Warnf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
		return "", fmt.Errorf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
	}
	// 构建交易指令
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	if t.Config.SkipATACheck {
		// 快速模式：直接添加创建ATA指令
		logger.Infof("快速模式：跳过ATA检查，直接创建ATA账户")
		createATAIx := associatedtokenaccount.NewCreateInstruction(
			params.TradeParams.Accounts.User, // payer
			params.TradeParams.Accounts.User, // owner
			params.TradeParams.Accounts.Mint, // mint
		).Build()
		instructions = append(instructions, createATAIx)
	} else {
		// 安全模式：先检查再创建
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.AssociatedTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA账户不存在，创建新账户...")
			createATAIx := associatedtokenaccount.NewCreateInstruction(
				params.TradeParams.Accounts.User, // payer
				params.TradeParams.Accounts.User, // owner
				params.TradeParams.Accounts.Mint, // mint
			).Build()
			instructions = append(instructions, createATAIx)
		}
	}

	// 构建sell指令
	// 使用pumpamm包的正确API
	sellInstruction := raydium.NewSwapInstructionBuilder().
		SetProgramAccount(RAYDIUM_V4_PROGRAM_ID).
		SetTokenProgramAccount(solana.TokenProgramID).
		SetPoolAccount(params.TradeParams.Accounts.RaydiumV4Accounts.Pool).
		SetAuthorityAccount(params.TradeParams.Accounts.RaydiumV4Accounts.Authority).
		SetOpenOrders(params.TradeParams.Accounts.RaydiumV4Accounts.OpenOrders).
		SetTargetOrders(params.TradeParams.Accounts.RaydiumV4Accounts.TargetOrders).
		SetBaseVault(params.TradeParams.Accounts.RaydiumV4Accounts.BaseVault).
		SetQuoteVault(params.TradeParams.Accounts.RaydiumV4Accounts.QuoteVault).
		SetMarketProgramId(params.TradeParams.Accounts.RaydiumV4Accounts.MarketProgramId).
		SetMarketId(params.TradeParams.Accounts.RaydiumV4Accounts.MarketId).
		SetMarketBids(params.TradeParams.Accounts.RaydiumV4Accounts.MarketAsks).
		SetMarketAsks(params.TradeParams.Accounts.RaydiumV4Accounts.MarketAsks).
		SetMarketEventQueue(params.TradeParams.Accounts.RaydiumV4Accounts.MarketEventQueue).
		SetMarketBaseVault(params.TradeParams.Accounts.RaydiumV4Accounts.MarketBaseVault).
		SetMarketQuoteVault(params.TradeParams.Accounts.RaydiumV4Accounts.MarketQuoteVault).
		SetMarketAuthority(params.TradeParams.Accounts.RaydiumV4Accounts.MarketAuthority).
		SetUserBaseTokenAccountAccount(params.TradeParams.Accounts.RaydiumV4Accounts.UserQuoteTokenAccount).
		SetUserQuoteTokenAccountAccount(params.TradeParams.Accounts.RaydiumV4Accounts.UserBaseTokenAccount).
		SetUserAccount(params.TradeParams.Accounts.User).
		SetBaseAmountIn(params.TradeParams.Amount).
		SetMinQuoteAmountOut(params.TradeParams.Cost)

	instructions = append(instructions, sellInstruction.Build())

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	// 检查各个模式是否启用并准备发送
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Debugf("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("卖出交易已发送! 交易ID: %s", sig)
		global.UpdateTokenSellTime(mintAddress)
		return sig, nil
	case err := <-errChan:
		logger.Errorf("卖出交易已发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

// Sell 实现卖出交易
func (t *TraderImpl) Sell(ctx context.Context, params types.OrderParams) (string, error) {
	logger.Infof("开始执行卖出交易...")

	mintAddress := params.TradeParams.Accounts.Mint.String()
	logger.Infof("Mint地址: %s ,卖出数量: %d, 最小获得: %d SOL, time: %v", mintAddress, params.TradeParams.Amount, params.TradeParams.Cost, time.Now().Format("2006-01-02 15:04:05.000000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 检查是否可以卖出（5秒内同一个币最多卖出一次）
	if !global.CanSellToken(mintAddress) {
		logger.Warnf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
		return "", fmt.Errorf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
	}

	// 构建交易指令
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)
	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// 统一交易接口：一律走 sell_v2。
	if params.TradeParams.Accounts.PumpV3 == nil {
		logger.Errorf("Pump V3 账户未初始化")
		return "", fmt.Errorf("pump V3 账户未初始化")
	}
	buildInstruction, err := pumpV3.NewSellV2Instruction(
		params.TradeParams.Amount,
		params.TradeParams.Cost,
		buildPumpV3TradeAccounts(params.TradeParams.Accounts.PumpV3),
	)
	if err != nil {
		logger.Errorf("构建 Pump sell_v2 指令失败: %v", err)
		return "", err
	}

	instructions = append(instructions, buildInstruction)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	// 检查各个模式是否启用并准备发送
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Debugf("[NextBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.JitoParams != nil && params.JitoParams.Enable {
		log.Printf("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		log.Printf("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("卖出交易已发送! 交易ID: %s", sig)
		// 更新代币卖出时间，防止短时间内重复卖出
		global.UpdateTokenSellTime(mintAddress)
		return sig, nil
	case err := <-errChan:
		global.UpdateTokenSellTime(mintAddress)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

func createSetComputeUnitPriceInstruction(microLamports uint64) solana.Instruction {
	data := make([]byte, 9)
	data[0] = 3 // 指令索引
	binary.LittleEndian.PutUint64(data[1:], microLamports)

	return solana.NewInstruction(
		solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111"),
		solana.AccountMetaSlice{},
		data,
	)
}

// createSetComputeUnitLimitInstruction 创建设置计算单元限制的指令
func createSetComputeUnitLimitInstruction(units uint32) solana.Instruction {
	data := make([]byte, 5)
	data[0] = 2 // 指令索引为 2
	binary.LittleEndian.PutUint32(data[1:], units)

	return solana.NewInstruction(
		solana.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111"),
		solana.AccountMetaSlice{},
		data,
	)
}

// PumpAmmSell 实现卖出交易
func (t *TraderImpl) AmmSell(ctx context.Context, params types.OrderParams, balanceNeeded uint64) (string, error) {
	logger.Infof("开始执行 AMM 卖出交易...")

	mintAddress := params.TradeParams.Accounts.Mint.String()
	logger.Infof("Mint地址: %s, 卖出数量: %d, 最小获得: %.6f SOL, time: %v",
		mintAddress, params.TradeParams.Amount, float64(params.TradeParams.Cost)/1e9, time.Now().Format("2006-01-02 15:04:05.000000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 检查是否可以卖出（3秒内同一个币最多卖出一次）
	if !global.CanSellToken(mintAddress) {
		logger.Warnf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
		return "", fmt.Errorf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
	}

	// 构建交易指令
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))
	// 检查WSOL账户是否已存在
	userQuoteTokenAccount := params.TradeParams.Accounts.UserQuoteTokenAccount

	logger.Info("AmmSell: Idempotent模式创建并初始化WSOL账户...")
	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(botWallet.PublicKey()).
		SetWallet(botWallet.PublicKey()).
		SetMint(WSOL).
		Build()

	instructions = append(instructions, createWsolAccountInstruction)

	// 根据代币版本确定 Token Program
	var baseTokenProgramID = solana.TokenProgramID
	if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
		baseTokenProgramID = solana.Token2022ProgramID
		logger.Infof("卖出使用 Token-2022 Program")
	} else {
		logger.Infof("卖出使用标准 Token Program")
	}

	// 使用 pumpammV2 模块创建卖出指令
	sellInstruction, err := pumpammV2.NewSellInstruction(
		params.TradeParams.Amount,                             // baseAmountIn
		params.TradeParams.Cost,                               // minQuoteAmountOut
		params.TradeParams.Accounts.Pool,                      // pool
		params.TradeParams.Accounts.User,                      // user
		GLOBAL_CONFIG_ACCOUNT,                                 // globalConfig
		params.TradeParams.Accounts.Mint,                      // baseMint
		params.TradeParams.Accounts.QuoteMint,                 // quoteMint
		params.TradeParams.Accounts.UserBaseTokenAccount,      // userBaseTokenAccount
		userQuoteTokenAccount,                                 // userQuoteTokenAccount
		params.TradeParams.Accounts.PoolBaseTokenAccount,      // poolBaseTokenAccount
		params.TradeParams.Accounts.PoolQuoteTokenAccount,     // poolQuoteTokenAccount
		PROTOCOL_FEE_RECIPIENT,                                // protocolFeeRecipient
		PROTOCOL_FEE_RECIPIENT_TOKEN_ACCOUNT,                  // protocolFeeRecipientTokenAccount
		baseTokenProgramID,                                    // baseTokenProgram
		solana.TokenProgramID,                                 // quoteTokenProgram
		solana.SystemProgramID,                                // systemProgram
		solana.SPLAssociatedTokenAccountProgramID,             // associatedTokenProgram
		AMM_EVENT_AUTHORITY,                                   // eventAuthority
		pumpammV2.ProgramID,                                   // program
		params.TradeParams.Accounts.CreatorVault,              // coinCreatorVaultAta
		params.TradeParams.Accounts.CoinCreatorVaultAuthority, // coinCreatorVaultAuthority
		solana.MustPublicKeyFromBase58("5PHirr8joyTMp9JMm6nW7hNDVyEYdkzDqazxPD7RaTjx"), // feeConfig
		solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ"),  // feeProgram
	)
	if err != nil {
		logger.Errorf("AmmSell构建指令失败: %v", err)
		return "", fmt.Errorf("AmmSell构建指令失败: %w", err)
	}

	instructions = append(instructions, sellInstruction)
	// 关闭WSOL账户
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		userQuoteTokenAccount, // 要关闭的账户
		botWallet.PublicKey(), // 接收剩余SOL的地址
		botWallet.PublicKey(), // 关闭账户的所有者
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("AMM卖出交易已发送! 交易ID: %s", sig)
		// 更新代币卖出时间，防止短时间内重复卖出
		global.UpdateTokenSellTime(mintAddress)
		return sig, nil
	case err := <-errChan:
		// 更新代币卖出时间，防止短时间内重复卖出
		logger.Errorf("AMM卖出交易发送失败!: %v", err)
		global.UpdateTokenSellTime(mintAddress)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

// RaydiumCPMMBuy executes a buy transaction on the Raydium CPMM
func (t *TraderImpl) RaydiumCPMMBuy(ctx context.Context, params types.OrderParams) (string, error) {

	logger.Infof("Starting Raydium CPMM buy transaction...")
	logger.Infof("Mint address: %s", params.TradeParams.Accounts.Mint)
	logger.Infof("sol amount in: %d", params.TradeParams.Amount)
	logger.Infof("min token out: %d", params.TradeParams.Cost)
	logger.Infof("Time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	botWallet := global.GetConfig().TradeWalletKey
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// Add compute unit and priority fee instructions
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// Create Associated Token Account if it doesn't exist
	cpmmAccounts := params.TradeParams.Accounts.RaydiumCPMMAccounts
	if t.Config.SkipATACheck {
		logger.Infof("Fast mode: skipping ATA check, creating ATA directly.")
		createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(botWallet.PublicKey()).
			SetWallet(botWallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			Build()
		instructions = append(instructions, createATAIx)
	} else {
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.RaydiumCPMMAccounts.OutputTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA not found, creating new account...")
			createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(botWallet.PublicKey()).
				SetWallet(botWallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint).
				Build()
			instructions = append(instructions, createATAIx)
		}
	}

	// Setup WSOL account
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(botWallet.PublicKey()).
		SetWallet(botWallet.PublicKey()).
		SetMint(WSOL).
		Build()
	instructions = append(instructions, createWsolAccountInstruction)

	transferIx := system.NewTransferInstruction(
		params.TradeParams.MaxSolAmount,
		botWallet.PublicKey(),
		cpmmAccounts.InputTokenAccount,
	).Build()
	instructions = append(instructions, transferIx)

	syncNativeIx := token.NewSyncNativeInstruction(
		cpmmAccounts.InputTokenAccount,
	).Build()
	instructions = append(instructions, syncNativeIx)

	// Build the Raydium CPMM swap instruction
	//logger.Infof("Building Raydium CPMM swap instruction...%+v", cpmmAccounts)
	buyInstruction := raydium_cp_swap.NewSwapBaseInputInstructionBuilder().
		SetPayerAccount(botWallet.PublicKey()).
		SetAuthorityAccount(cpmmAccounts.Authority).
		SetAmmConfigAccount(cpmmAccounts.AmmConfig).
		SetPoolStateAccount(cpmmAccounts.PoolState).
		SetInputTokenAccountAccount(cpmmAccounts.InputTokenAccount).
		SetOutputTokenAccountAccount(cpmmAccounts.OutputTokenAccount).
		SetInputVaultAccount(cpmmAccounts.InputVault).
		SetOutputVaultAccount(cpmmAccounts.OutputVault).
		SetInputTokenProgramAccount(solana.TokenProgramID).
		SetOutputTokenProgramAccount(solana.TokenProgramID).
		SetInputTokenMintAccount(cpmmAccounts.InputTokenMint).
		SetOutputTokenMintAccount(cpmmAccounts.OutputTokenMint).
		SetObservationStateAccount(cpmmAccounts.ObservationState).
		SetAmountIn(params.TradeParams.Amount).
		SetMinimumAmountOut(params.TradeParams.Cost).
		Build()

	instructions = append(instructions, buyInstruction)

	// Close WSOL account
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		cpmmAccounts.InputTokenAccount,
		botWallet.PublicKey(),
		botWallet.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// Create and send transaction
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("Astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[Blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("nextblock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("买入交易已发送! 交易ID: %s", sig)

		return sig, nil
	case err := <-errChan:
		logger.Errorf("买入交易已发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

// RaydiumLaunchpadBuy executes a buy transaction on the Raydium Launchpad
func (t *TraderImpl) RaydiumLaunchpadBuy(ctx context.Context, params types.OrderParams) (string, error) {

	logger.Infof("Starting Raydium Launchpad buy transaction...")
	logger.Infof("Mint address: %s", params.TradeParams.Accounts.Mint)
	logger.Infof("sol amount in: %d", params.TradeParams.Amount)
	logger.Infof("min token out: %d", params.TradeParams.Cost)
	logger.Infof("Time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	botWallet := global.GetConfig().TradeWalletKey
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// Add compute unit and priority fee instructions
	instructions = append(instructions, createSetComputeUnitLimitInstruction(400_000))
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	launchpadAccounts := params.TradeParams.Accounts.LaunchpadAccounts
	// Create Associated Token Account if it doesn't exist
	if t.Config.SkipATACheck {
		logger.Infof("Fast mode: skipping ATA check, creating ATA directly.")
		createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(botWallet.PublicKey()).
			SetWallet(botWallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			Build()
		instructions = append(instructions, createATAIx)
	} else {
		account, err := t.Client.GetAccountInfo(ctx, launchpadAccounts.UserBaseTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA not found, creating new account...")
			createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(botWallet.PublicKey()).
				SetWallet(botWallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint).
				Build()
			instructions = append(instructions, createATAIx)
		}
	}

	// Setup WSOL account
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(botWallet.PublicKey()).
		SetWallet(botWallet.PublicKey()).
		SetMint(WSOL).
		Build()
	instructions = append(instructions, createWsolAccountInstruction)

	transferIx := system.NewTransferInstruction(
		params.TradeParams.MaxSolAmount,
		botWallet.PublicKey(),
		launchpadAccounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, transferIx)

	syncNativeIx := token.NewSyncNativeInstruction(
		launchpadAccounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, syncNativeIx)

	// Build the Raydium Launchpad swap instruction
	buyInstruction := raydium_launchpad.NewBuyExactInInstructionBuilder().
		SetPayerAccount(botWallet.PublicKey()).
		SetAuthorityAccount(launchpadAccounts.Authority).
		SetGlobalConfigAccount(launchpadAccounts.GlobalConfig).
		SetPlatformConfigAccount(launchpadAccounts.PlatformConfig).
		SetPoolStateAccount(launchpadAccounts.PoolState).
		SetUserBaseTokenAccount(launchpadAccounts.UserBaseTokenAccount).
		SetUserQuoteTokenAccount(launchpadAccounts.UserQuoteTokenAccount).
		SetBaseVaultAccount(launchpadAccounts.BaseVault).
		SetQuoteVaultAccount(launchpadAccounts.QuoteVault).
		SetBaseTokenMintAccount(launchpadAccounts.BaseTokenMint).
		SetQuoteTokenMintAccount(launchpadAccounts.QuoteTokenMint).
		SetBaseTokenProgramAccount(solana.TokenProgramID).
		SetQuoteTokenProgramAccount(solana.TokenProgramID).
		SetEventAuthorityAccount(launchpadAccounts.EventAuthority).
		SetProgramAccount(RAYDIUM_LAUNCHPAD_PROGRAM_ID).
		SetSystemProgramAccount(solana.SystemProgramID).
		SetClaimPlatformFeeAccount(launchpadAccounts.ClaimPlatformFee).
		SetClaimCreatorFeeAccount(launchpadAccounts.ClaimCreatorFee).
		SetAmountIn(params.TradeParams.Amount).
		SetMinimumAmountOut(params.TradeParams.Cost).
		Build()

	instructions = append(instructions, buyInstruction)

	// Close WSOL account
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		launchpadAccounts.UserQuoteTokenAccount,
		botWallet.PublicKey(),
		botWallet.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// Create and send transaction
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("Astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[Blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("Blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("买入交易已发送! 交易ID: %s", sig)

		return sig, nil
	case err := <-errChan:
		logger.Errorf("买入交易已发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}

func (t *TraderImpl) RaydiumCPMMSell(ctx context.Context, params types.OrderParams) (string, error) {

	mintAddress := params.TradeParams.Accounts.Mint.String()
	logger.Infof("开始执行RaydiumCPMM卖出交易...")
	logger.Infof("Mint地址: %s", mintAddress)
	logger.Infof("卖出数量: %d", params.TradeParams.Amount)
	logger.Infof("time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	botWallet := global.GetConfig().TradeWalletKey
	// 检查是否可以卖出（3秒内同一个币最多卖出一次）
	if !global.CanSellToken(mintAddress) {
		logger.Warnf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
		return "", fmt.Errorf("代币 %s 卖出频率过高，3秒内只能卖出一次", mintAddress)
	}
	// 构建交易指令
	instructions := []solana.Instruction{}
	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount, // nonce账户
		solana.SysVarRecentBlockHashesPubkey,     // 近期区块哈希系统变量
		botWallet.PublicKey(),                    // 授权者（签名者）
	).Build()
	instructions = append(instructions, advanceNonceIx)
	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(botWallet.PublicKey()).
		SetWallet(botWallet.PublicKey()).
		SetMint(WSOL).
		Build()

	instructions = append(instructions, createWsolAccountInstruction)

	// 构建sell指令
	// 使用raydium_cpmmp包的正确指令
	cpmmAccounts := params.TradeParams.Accounts.RaydiumCPMMAccounts
	//@todo: 请检查 raydium_cpmmp 包的正确性,input 和output 的顺序
	sellInstruction := raydium_cp_swap.NewSwapBaseInputInstructionBuilder().
		SetPayerAccount(botWallet.PublicKey()).
		SetAuthorityAccount(cpmmAccounts.Authority).
		SetAmmConfigAccount(cpmmAccounts.AmmConfig).
		SetPoolStateAccount(cpmmAccounts.PoolState).
		SetInputTokenAccountAccount(cpmmAccounts.OutputTokenAccount).
		SetOutputTokenAccountAccount(cpmmAccounts.InputTokenAccount).
		SetInputVaultAccount(cpmmAccounts.OutputVault).
		SetOutputVaultAccount(cpmmAccounts.InputVault).
		SetInputTokenProgramAccount(solana.TokenProgramID).
		SetOutputTokenProgramAccount(solana.TokenProgramID).
		SetInputTokenMintAccount(cpmmAccounts.OutputTokenMint).
		SetOutputTokenMintAccount(cpmmAccounts.InputTokenMint).
		SetObservationStateAccount(cpmmAccounts.ObservationState).
		SetAmountIn(params.TradeParams.Amount).
		SetMinimumAmountOut(params.TradeParams.Cost)

	instructions = append(instructions, sellInstruction.Build())

	//close wsol account
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		cpmmAccounts.InputTokenAccount,
		botWallet.PublicKey(),
		botWallet.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	// 检查各个模式是否启用并准备发送
	if params.FlashBlockParams != nil && params.FlashBlockParams.Enable {
		logger.Infof("[FlashBlock] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionFlashBlock(
				botWallet,
				instructions,
				params.FlashBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("FlashBlock发送失败: %s", err)
				errChan <- fmt.Errorf("FlashBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.AstralaneParams != nil && params.AstralaneParams.Enable {
		logger.Infof("[Astralane] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionAns(
				botWallet,
				instructions,
				params.AstralaneParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("Astralane发送失败: %s", err)
				errChan <- fmt.Errorf("Astralane发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BlockrazorParams != nil && params.BlockrazorParams.Enable {
		logger.Infof("[blockrazor] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBlockrazor(
				botWallet,
				instructions,
				params.BlockrazorParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("blockrazor发送失败: %s", err)
				errChan <- fmt.Errorf("blockrazor发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Debugf("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				botWallet,
				instructions,
				params.ZeroslotParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("0slot发送失败: %s", err)
				errChan <- fmt.Errorf("0slot发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if params.BloXrouteParams != nil && params.BloXrouteParams.Enable {
		logger.Infof("[BloXroute] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionBloXroute(
				botWallet.PrivateKey,
				instructions,
				*params.BloXrouteParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				logger.Errorf("BloXroute发送失败: %s", err)
				errChan <- fmt.Errorf("BloXroute发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	// 检查各个模式是否启用并准备发送
	if params.NextBlockParams != nil && params.NextBlockParams.Enable {
		logger.Infof("[NextBlock] 准备发送交易...")
		go func() {
			// 构建交易并发送到 NextBlock
			sig, err := swqos.SendTransactionNextBlock(
				botWallet,
				instructions,
				params.NextBlockParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("NextBlock发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.JitoParams != nil && params.JitoParams.Enable {
		logger.Infof("[Jito] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionJito(
				botWallet,
				instructions,
				params.JitoParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("jito发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}
	if params.TemporalParams != nil && params.TemporalParams.Enable {
		logger.Infof("[Temporal] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionTemporal(
				botWallet,
				instructions,
				params.TemporalParams,
				params.TradeParams.Blockhash,
			)
			if err != nil {
				errChan <- fmt.Errorf("temporal发送失败: %w", err)
				return
			}
			resultChan <- sig
		}()
	}

	if t.Config.Normal.Enable {
		logger.Infof("[Normal] 准备发送交易...")
		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(botWallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(botWallet.PublicKey()) {
						return &botWallet.PrivateKey
					}
					return nil
				},
			)
			if err != nil {
				errChan <- fmt.Errorf("签名交易失败: %w", err)
				return
			}

			// 发送交易
			sig, err := t.Client.SendTransaction(ctx, tx)
			if err != nil {
				errChan <- fmt.Errorf("发送交易失败: %w", err)
				return
			}

			resultChan <- sig.String()
		}()
	}

	// 等待结果
	select {
	case sig := <-resultChan:
		logger.Infof("卖出交易已发送! 交易ID: %s", sig)
		global.UpdateTokenSellTime(mintAddress)
		return sig, nil
	case err := <-errChan:
		logger.Errorf("卖出交易已发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}
