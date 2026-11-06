package core

import (
	"context"
	"fmt"
	"log"
	"time"

	dbcV2 "copytrade/dbcv2"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/swqos"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "copytrade/helpers/associatedtokenaccount"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
)

// MeteoraDbcBuy executes a buy transaction on Meteora DBC
func (t *TraderImpl) MeteoraDbcBuy(ctx context.Context, params types.OrderParams) (string, error) {

	logger.Infof("Starting Meteora DBC buy transaction...")
	logger.Infof("Mint address: %s", params.TradeParams.Accounts.Mint)
	logger.Infof("sol amount in: %d", params.TradeParams.Amount)
	logger.Infof("min token out: %d", params.TradeParams.Cost)
	logger.Infof("Time: %v", time.Now().Format("2006-01-02 15:04:05.000"))
	mintAddress := params.TradeParams.Accounts.Mint.String()
	botWallet := global.GetConfig().TradeWalletKey
	instructions := []solana.Instruction{}

	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		params.TradeParams.Accounts.NonceAccount,
		solana.SysVarRecentBlockHashesPubkey,
		botWallet.PublicKey(),
	).Build()
	instructions = append(instructions, advanceNonceIx)

	// Add compute unit and priority fee instructions
	instructions = append(instructions, createSetComputeUnitLimitInstruction(300_000))
	instructions = append(instructions, createSetComputeUnitPriceInstruction(params.TradeParams.FeeConfig.PriorityFee))

	// Create Associated Token Account if it doesn't exist
	dbcAccounts := params.TradeParams.Accounts.MeteoraDbcAccounts
	if t.Config.SkipATACheck {
		logger.Infof("Fast mode: skipping ATA check, creating ATA directly.")
		createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
			SetPayer(botWallet.PublicKey()).
			SetWallet(botWallet.PublicKey()).
			SetMint(params.TradeParams.Accounts.Mint).
			Build()
		instructions = append(instructions, createATAIx)
	} else {
		account, err := t.Client.GetAccountInfo(ctx, dbcAccounts.OutputTokenAccount)
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
		dbcAccounts.InputTokenAccount,
	).Build()
	instructions = append(instructions, transferIx)

	syncNativeIx := token.NewSyncNativeInstruction(
		dbcAccounts.InputTokenAccount,
	).Build()
	instructions = append(instructions, syncNativeIx)

	// Build the Meteora DBC swap instruction using dbcv2.Swap
	buyInstruction := dbcV2.Swap(
		dbcAccounts.Config,
		dbcAccounts.Pool,
		dbcAccounts.InputTokenAccount,
		dbcAccounts.OutputTokenAccount,
		dbcAccounts.OutputVault,
		dbcAccounts.InputVault,
		dbcAccounts.OutputTokenMint,
		dbcAccounts.InputTokenMint,
		botWallet.PublicKey(),
		dbcAccounts.ReferralTokenAccount,
		params.TradeParams.Amount,
		params.TradeParams.Cost,
	)
	instructions = append(instructions, buyInstruction)

	// Close WSOL account
	closeWsolAccountInstruction := token.NewCloseAccountInstruction(
		dbcAccounts.InputTokenAccount,
		botWallet.PublicKey(),
		botWallet.PublicKey(),
		[]solana.PublicKey{},
	).Build()
	instructions = append(instructions, closeWsolAccountInstruction)

	// Create and send transaction
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
		logger.Infof("Meteora DBC 买入交易已发送! 交易ID: %s", sig)
		global.UpdateTokenSellTime(mintAddress)
		return sig, nil
	case err := <-errChan:
		logger.Errorf("Meteora DBC 买入交易发送失败!: %v", err)
		return "", err
	case <-timeout:
		return "", fmt.Errorf("transaction timeout")
	}
}
