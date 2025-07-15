package core

import (
	"context"

	"copytrade/calculator"
	//"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/pump"
	"copytrade/pumpamm"
	"copytrade/raydium"
	"copytrade/raydium_launchpad"
	"copytrade/swqos"
	"copytrade/types"
	"copytrade/utils"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
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

// Buy 执行买入交易
func (t *TraderImpl) Buy(ctx context.Context, params types.OrderParams) (string, error) {

	logger.Infof("开始执行买入交易...")
	logger.Infof("Mint地址: %s", params.TradeParams.Accounts.Mint)
	logger.Infof("购买数量: %d", params.TradeParams.Amount)
	logger.Infof("构建指令时间: %v", time.Now().Format("2006-01-02 15:04:05.000000"))

	// 构建交易指令
	instructions := []solana.Instruction{}

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

	// 构建Buy指令

	buyInstruction := pump.NewBuyInstructionBuilder().
		SetGlobalAccount(GLOBAL_ACCOUNT).
		SetFeeRecipientAccount(FEE_RECIPIENT).
		SetMintAccount(params.TradeParams.Accounts.Mint).
		SetBondingCurveAccount(params.TradeParams.Accounts.BondingCurve).
		SetAssociatedBondingCurveAccount(params.TradeParams.Accounts.AssociatedBondingCurve).
		SetAssociatedUserAccount(params.TradeParams.Accounts.AssociatedTokenAccount).
		SetUserAccount(params.TradeParams.Accounts.User).
		SetSystemProgramAccount(solana.SystemProgramID).
		SetTokenProgramAccount(solana.TokenProgramID).
		SetCreatorVaultAccount(params.TradeParams.Accounts.CreatorVault).
		SetEventAuthorityAccount(EVENT_AUTHORITY).
		SetProgramAccount(PUMP_PROGRAM_ID).
		SetAmount(params.TradeParams.Amount).
		SetMaxSolCost(params.TradeParams.Cost)
	buildInstruction, err := buyInstruction.ValidateAndBuild()
	if err != nil {
		logger.Errorf("构建Buy指令失败: %v", err)
		return "", err
	}

	instructions = append(instructions, buildInstruction)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	logger.Infof("构建指令时间结束: %v", time.Now().Format("2006-01-02 15:04:05.000000"))

	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
		logger.Infof("latest blockhash", params.TradeParams.Blockhash.String())

		go func() {
			// 构建交易
			tx, err := solana.NewTransaction(
				instructions,
				params.TradeParams.Blockhash,
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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

	// 构建交易指令
	instructions := []solana.Instruction{}

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))
	if t.Config.SkipATACheck {
		// 快速模式：直接添加创建ATA指令
		logger.Infof("快速模式：跳过ATA检查，直接创建ATA账户")
		createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(t.Wallet.PublicKey()).
			SetWallet(t.Wallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			Build()
		instructions = append(instructions, createATAIx)
	} else {
		// 安全模式：先检查再创建
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.AssociatedTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA账户不存在，创建新账户...")
			createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(t.Wallet.PublicKey()).
				SetWallet(t.Wallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint).
				Build()
			instructions = append(instructions, createATAIx)
		}
	}
	logger.Info("PumpAmm Buy: Idempotent模式创建并初始化WSOL账户...")
	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(t.Wallet.PublicKey()).
		SetWallet(t.Wallet.PublicKey()).
		SetMint(WSOL).
		Build()
	// 初始化WSOL账户指令
	instructions = append(instructions, createWsolAccountInstruction)

	// 将SOL转入WSOL账户
	transferIx := system.NewTransferInstruction(
		params.TradeParams.Cost,
		t.Wallet.PublicKey(),
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, transferIx)

	// 同步WSOL余额
	syncNativeIx := token.NewSyncNativeInstruction(
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, syncNativeIx)

	buyInstruction := pumpamm.NewBuyInstructionBuilder().
		SetPoolAccount(params.TradeParams.Accounts.Pool).
		SetUserAccount(params.TradeParams.Accounts.User).
		SetGlobalConfigAccount(GLOBAL_CONFIG_ACCOUNT).
		SetBaseMintAccount(params.TradeParams.Accounts.BaseMint).
		SetQuoteMintAccount(params.TradeParams.Accounts.QuoteMint).
		SetUserBaseTokenAccountAccount(params.TradeParams.Accounts.UserBaseTokenAccount).
		SetUserQuoteTokenAccountAccount(params.TradeParams.Accounts.UserQuoteTokenAccount).
		SetPoolBaseTokenAccountAccount(params.TradeParams.Accounts.PoolBaseTokenAccount).
		SetPoolQuoteTokenAccountAccount(params.TradeParams.Accounts.PoolQuoteTokenAccount).
		SetProtocolFeeRecipientAccount(PROTOCOL_FEE_RECIPIENT).
		SetProtocolFeeRecipientTokenAccountAccount(PROTOCOL_FEE_RECIPIENT_TOKEN_ACCOUNT).
		SetBaseTokenProgramAccount(solana.TokenProgramID).
		SetQuoteTokenProgramAccount(solana.TokenProgramID).
		SetSystemProgramAccount(solana.SystemProgramID).
		SetCoinCreatorVaultAtaAccount(params.TradeParams.Accounts.CreatorVault).
		SetCoinCreatorVaultAuthorityAccount(params.TradeParams.Accounts.CoinCreatorVaultAuthority).
		SetEventAuthorityAccount(AMM_EVENT_AUTHORITY).
		SetProgramAccount(pumpamm.ProgramID).
		SetBaseAmountOut(params.TradeParams.Amount).
		SetMaxQuoteAmountIn(params.TradeParams.Cost)

	instructions = append(instructions, buyInstruction.Build())

	// 关闭WSOL账户
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		params.TradeParams.Accounts.UserQuoteTokenAccount, // 要关闭的账户
		t.Wallet.PublicKey(),                              // 接收剩余SOL的地址
		t.Wallet.PublicKey(),                              // 关闭账户的所有者
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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

	// 构建交易指令
	instructions := []solana.Instruction{}

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	if t.Config.SkipATACheck {
		// 快速模式：直接添加创建ATA指令
		logger.Infof("快速模式：跳过ATA检查，直接创建ATA账户")
		createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(t.Wallet.PublicKey()).
			SetWallet(t.Wallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			Build()
		instructions = append(instructions, createATAIx)
	} else {
		// 安全模式：先检查再创建
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.AssociatedTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA账户不存在，创建新账户...")
			createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(t.Wallet.PublicKey()).
				SetWallet(t.Wallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint).
				Build()
			instructions = append(instructions, createATAIx)
		}
	}
	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(t.Wallet.PublicKey()).
		SetWallet(t.Wallet.PublicKey()).
		SetMint(WSOL).
		Build()
	// 初始化WSOL账户指令
	instructions = append(instructions, createWsolAccountInstruction)

	// 将SOL转入WSOL账户
	transferIx := system.NewTransferInstruction(
		params.TradeParams.Cost,
		t.Wallet.PublicKey(),
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

	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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

	// 构建交易指令
	instructions := []solana.Instruction{}

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(t.Wallet.PublicKey()).
		SetWallet(t.Wallet.PublicKey()).
		SetMint(WSOL).
		Build()

	instructions = append(instructions, createWsolAccountInstruction)

	// 构建sell指令
	// 使用raydium_launchpad包的正确API
	sellInstruction := raydium_launchpad.NewSellExactInInstructionBuilder().
		SetAmountIn(params.TradeParams.Amount).
		SetMinimumAmountOut(params.TradeParams.Cost).
		SetProgramAccount(RAYDIUM_LAUNCHPAD_PROGRAM_ID).
		SetPayerAccount(t.Wallet.PublicKey()).
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
		SetEventAuthorityAccount(params.Accounts.LaunchpadAccounts.EventAuthority)

	instructions = append(instructions, sellInstruction.Build())

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	// 检查各个模式是否启用并准备发送
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Debugf("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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

	// 构建交易指令
	instructions := []solana.Instruction{}

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
	// 检查各个模式是否启用并准备发送
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Debugf("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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

	// 检查是否可以卖出（5秒内同一个币最多卖出一次）
	if !global.CanSellToken(mintAddress) {
		logger.Warnf("代币 %s 卖出频率过高，5秒内只能卖出一次", mintAddress)
		return "", fmt.Errorf("代币 %s 卖出频率过高，5秒内只能卖出一次", mintAddress)
	}

	// 构建交易指令
	instructions := []solana.Instruction{}

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// 构建Sell指令
	sellInstruction := pump.NewSellInstructionBuilder().
		SetGlobalAccount(GLOBAL_ACCOUNT).
		SetFeeRecipientAccount(FEE_RECIPIENT).
		SetMintAccount(params.TradeParams.Accounts.Mint).
		SetBondingCurveAccount(params.TradeParams.Accounts.BondingCurve).
		SetAssociatedBondingCurveAccount(params.TradeParams.Accounts.AssociatedBondingCurve).
		SetAssociatedUserAccount(params.TradeParams.Accounts.AssociatedTokenAccount).
		SetUserAccount(params.TradeParams.Accounts.User).
		SetSystemProgramAccount(solana.SystemProgramID).
		SetCreatorVaultAccount(params.TradeParams.Accounts.CreatorVault).
		SetTokenProgramAccount(solana.TokenProgramID).
		SetEventAuthorityAccount(EVENT_AUTHORITY).
		SetProgramAccount(PUMP_PROGRAM_ID).
		SetAmount(params.TradeParams.Amount).
		SetMinSolOutput(params.TradeParams.Cost)

	ix, err := sellInstruction.ValidateAndBuild()
	if err != nil {
		logger.Errorf("构建Sell指令失败: %v", err)
		return "", fmt.Errorf("构建Sell指令失败: %w", err)
	}

	instructions = append(instructions, ix)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)
	// 检查各个模式是否启用并准备发送
	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Debugf("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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

// AmmSell 实现卖出交易
func (t *TraderImpl) AmmSell(ctx context.Context, params types.OrderParams, balanceNeeded uint64) (string, error) {
	logger.Infof("开始执行 AMM 卖出交易...")

	mintAddress := params.TradeParams.Accounts.Mint.String()
	logger.Infof("Mint地址: %s, 卖出数量: %d, 最小获得: %.6f SOL, time: %v",
		mintAddress, params.TradeParams.Amount, float64(params.TradeParams.Cost)/1e9, time.Now().Format("2006-01-02 15:04:05.000000"))

	// 检查是否可以卖出（5秒内同一个币最多卖出一次）
	if !global.CanSellToken(mintAddress) {
		logger.Warnf("代币 %s 卖出频率过高，5秒内只能卖出一次", mintAddress)
		return "", fmt.Errorf("代币 %s 卖出频率过高，5秒内只能卖出一次", mintAddress)
	}

	// 构建交易指令
	instructions := []solana.Instruction{}

	// 添加计算单元限制指令（通常设置为 200,000 units）
	instructions = append(instructions, createSetComputeUnitLimitInstruction(200_000))

	// 添加优先级费用指令
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))
	// 检查WSOL账户是否已存在
	userQuoteTokenAccount := params.TradeParams.Accounts.UserQuoteTokenAccount

	logger.Info("AmmSell: Idempotent模式创建并初始化WSOL账户...")
	// 创建WSOL账户指令
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(t.Wallet.PublicKey()).
		SetWallet(t.Wallet.PublicKey()).
		SetMint(WSOL).
		Build()

	instructions = append(instructions, createWsolAccountInstruction)

	sellInstruction := pumpamm.NewSellInstructionBuilder().
		SetGlobalConfigAccount(GLOBAL_CONFIG_ACCOUNT).
		SetProtocolFeeRecipientAccount(PROTOCOL_FEE_RECIPIENT).
		SetProtocolFeeRecipientTokenAccountAccount(PROTOCOL_FEE_RECIPIENT_TOKEN_ACCOUNT).
		SetBaseMintAccount(params.TradeParams.Accounts.Mint).
		SetQuoteMintAccount(params.TradeParams.Accounts.QuoteMint).
		SetPoolAccount(params.TradeParams.Accounts.Pool).
		SetPoolBaseTokenAccountAccount(params.TradeParams.Accounts.PoolBaseTokenAccount).
		SetPoolQuoteTokenAccountAccount(params.TradeParams.Accounts.PoolQuoteTokenAccount).
		SetAssociatedTokenProgramAccount(associatedtokenaccount.ProgramID).
		SetUserAccount(params.TradeParams.Accounts.User).
		SetUserBaseTokenAccountAccount(params.TradeParams.Accounts.UserBaseTokenAccount).
		SetUserQuoteTokenAccountAccount(userQuoteTokenAccount).
		SetBaseTokenProgramAccount(solana.TokenProgramID).
		SetQuoteTokenProgramAccount(solana.TokenProgramID).
		SetSystemProgramAccount(solana.SystemProgramID).
		SetEventAuthorityAccount(AMM_EVENT_AUTHORITY).
		SetProgramAccount(AMM_PROGRAM_ID).
		SetCoinCreatorVaultAtaAccount(params.TradeParams.Accounts.CreatorVault).
		SetCoinCreatorVaultAuthorityAccount(params.TradeParams.Accounts.CoinCreatorVaultAuthority).
		SetBaseAmountIn(params.TradeParams.Amount).
		SetMinQuoteAmountOut(params.TradeParams.Cost)

	ix, err := sellInstruction.ValidateAndBuild()
	if err != nil {
		logger.Errorf("AmmSell构建指令失败: %v", err)
		return "", fmt.Errorf("AmmSell构建指令失败: %w", err)
	}

	instructions = append(instructions, ix)
	// 关闭WSOL账户
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		userQuoteTokenAccount, // 要关闭的账户
		t.Wallet.PublicKey(),  // 接收剩余SOL的地址
		t.Wallet.PublicKey(),  // 关闭账户的所有者
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// 创建通道
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)

	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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

// RaydiumLaunchpadBuy executes a buy transaction on the Raydium Launchpad
func (t *TraderImpl) RaydiumLaunchpadBuy(ctx context.Context, params types.OrderParams) (string, error) {
	if t.Config.SimulateMode {
		logger.Infof("Simulate mode: skipping buy transaction for %s", params.TradeParams.Accounts.Mint)
		return "mock_tx_signature", nil
	}

	logger.Infof("Starting Raydium Launchpad buy transaction...")
	logger.Infof("Mint address: %s", params.TradeParams.Accounts.Mint)
	logger.Infof("sol amount in: %d", params.TradeParams.Amount)
	logger.Infof("min token out: %d", params.TradeParams.Cost)
	logger.Infof("Time: %v", time.Now().Format("2006-01-02 15:04:05.000"))

	instructions := []solana.Instruction{}

	// Add compute unit and priority fee instructions
	instructions = append(instructions, createSetComputeUnitLimitInstruction(400_000))
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// Create Associated Token Account if it doesn't exist
	if t.Config.SkipATACheck {
		logger.Infof("Fast mode: skipping ATA check, creating ATA directly.")
		createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(t.Wallet.PublicKey()).
			SetWallet(t.Wallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			Build()
		instructions = append(instructions, createATAIx)
	} else {
		account, err := t.Client.GetAccountInfo(ctx, params.TradeParams.Accounts.AssociatedTokenAccount)
		if err != nil || account.Value == nil {
			logger.Infof("ATA not found, creating new account...")
			createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
				SetPayer(t.Wallet.PublicKey()).
				SetWallet(t.Wallet.PublicKey()).
				SetMint(params.TradeParams.Accounts.Mint).
				Build()
			instructions = append(instructions, createATAIx)
		}
	}

	// Setup WSOL account
	createWsolAccountInstruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(t.Wallet.PublicKey()).
		SetWallet(t.Wallet.PublicKey()).
		SetMint(WSOL).
		Build()
	instructions = append(instructions, createWsolAccountInstruction)

	transferIx := system.NewTransferInstruction(
		params.TradeParams.Cost,
		t.Wallet.PublicKey(),
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, transferIx)

	syncNativeIx := token.NewSyncNativeInstruction(
		params.TradeParams.Accounts.UserQuoteTokenAccount,
	).Build()
	instructions = append(instructions, syncNativeIx)

	// Build the Raydium Launchpad swap instruction
	launchpadAccounts := params.TradeParams.Accounts.LaunchpadAccounts
	buyInstruction := raydium_launchpad.NewBuyExactInInstructionBuilder().
		SetPayerAccount(t.Wallet.PublicKey()).
		SetAuthorityAccount(launchpadAccounts.Authority).
		SetGlobalConfigAccount(launchpadAccounts.GlobalConfig).
		SetPlatformConfigAccount(launchpadAccounts.PlatformConfig).
		SetPoolStateAccount(launchpadAccounts.PoolState).
		SetUserBaseTokenAccount(params.TradeParams.Accounts.UserBaseTokenAccount).
		SetUserQuoteTokenAccount(params.TradeParams.Accounts.UserQuoteTokenAccount).
		SetBaseVaultAccount(launchpadAccounts.BaseVault).
		SetQuoteVaultAccount(launchpadAccounts.QuoteVault).
		SetBaseTokenMintAccount(launchpadAccounts.BaseTokenMint).
		SetQuoteTokenMintAccount(launchpadAccounts.QuoteTokenMint).
		SetBaseTokenProgramAccount(solana.TokenProgramID).
		SetQuoteTokenProgramAccount(solana.TokenProgramID).
		SetEventAuthorityAccount(launchpadAccounts.EventAuthority).
		SetProgramAccount(raydium_launchpad.ProgramID).
		SetAmountIn(params.TradeParams.Amount).
		SetMinimumAmountOut(params.TradeParams.Cost).
		Build()

	instructions = append(instructions, buyInstruction)

	// Close WSOL account
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		params.TradeParams.Accounts.UserQuoteTokenAccount,
		t.Wallet.PublicKey(),
		t.Wallet.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// Create and send transaction
	resultChan := make(chan string, 4)
	errChan := make(chan error, 4)
	timeout := time.After(30 * time.Second)

	if params.ZeroslotParams != nil && params.ZeroslotParams.Enable {
		logger.Infof("[0slot] 准备发送交易...")
		go func() {
			sig, err := swqos.SendTransactionZeroSlot(
				t.Wallet,
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
				t.Wallet.PrivateKey,
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
				t.Wallet,
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
				t.Wallet,
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
				t.Wallet,
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
				solana.TransactionPayer(t.Wallet.PublicKey()),
			)
			if err != nil {
				errChan <- fmt.Errorf("创建交易失败: %w", err)
				return
			}

			// 签名交易
			_, err = tx.Sign(
				func(key solana.PublicKey) *solana.PrivateKey {
					if key.Equals(t.Wallet.PublicKey()) {
						return &t.Wallet.PrivateKey
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
