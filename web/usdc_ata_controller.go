package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"copytrade/helpers/associatedtokenaccount"
	"copytrade/pumpV3"
	"copytrade/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gin-gonic/gin"
)

// USDCATAController 提供机器人 USDC ATA 的状态查询和幂等创建。
type USDCATAController struct {
	ConfigPath string
}

func NewUSDCATAController(configPath string) *USDCATAController {
	return &USDCATAController{ConfigPath: configPath}
}

func (c *USDCATAController) RegisterRoutesWithAuth(router *gin.Engine) {
	api := router.Group("/api")
	api.Use(AuthMiddleware())
	api.GET("/usdc-ata", c.handleStatus)
	api.POST("/usdc-ata", c.handleCreate)
}

type usdcATAContext struct {
	client  *rpc.Client
	wallet  solana.Wallet
	ata     solana.PublicKey
	balance uint64
	exists  bool
}

func (c *USDCATAController) load(ctx context.Context) (*usdcATAContext, error) {
	config, err := utils.LoadConfig(c.ConfigPath)
	if err != nil {
		return nil, err
	}
	privateKey, err := solana.PrivateKeyFromBase58(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("解析交易钱包私钥失败: %w", err)
	}
	wallet := solana.Wallet{PrivateKey: privateKey}
	ata, err := pumpV3.AssociatedTokenAccount(wallet.PublicKey(), pumpV3.USDCMint, solana.TokenProgramID)
	if err != nil {
		return nil, fmt.Errorf("派生 USDC ATA 失败: %w", err)
	}
	client := rpc.New(config.RPCUrl)
	info, err := client.GetAccountInfoWithOpts(ctx, ata, &rpc.GetAccountInfoOpts{Encoding: solana.EncodingBase64})
	if err != nil && !errors.Is(err, rpc.ErrNotFound) {
		return nil, fmt.Errorf("查询 USDC ATA 失败: %w", err)
	}
	result := &usdcATAContext{
		client: client,
		wallet: wallet,
		ata:    ata,
		exists: info != nil && info.Value != nil,
	}
	if result.exists {
		balance, err := client.GetTokenAccountBalance(ctx, ata, rpc.CommitmentFinalized)
		if err != nil || balance == nil || balance.Value == nil {
			return nil, fmt.Errorf("查询 USDC 余额失败: %w", err)
		}
		result.balance, err = strconv.ParseUint(balance.Value.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("解析 USDC 余额失败: %w", err)
		}
	}
	return result, nil
}

func (c *USDCATAController) handleStatus(ctx *gin.Context) {
	state, err := c.load(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"wallet":     state.wallet.PublicKey().String(),
		"quote_mint": pumpV3.USDCMint.String(),
		"ata":        state.ata.String(),
		"exists":     state.exists,
		"balance":    strconv.FormatUint(state.balance, 10),
		"balance_ui": fmt.Sprintf("%.6f", float64(state.balance)/1e6),
		"decimals":   pumpV3.QuoteDecimals(pumpV3.USDCMint),
		"ready":      state.exists && state.balance > 0,
	})
}

func (c *USDCATAController) handleCreate(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), 30_000_000_000)
	defer cancel()
	state, err := c.load(requestCtx)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if state.exists {
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "already_exists",
			"message": "USDC ATA 已存在",
			"ata":     state.ata.String(),
			"balance": strconv.FormatUint(state.balance, 10),
		})
		return
	}

	instruction := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
		SetPayer(state.wallet.PublicKey()).
		SetWallet(state.wallet.PublicKey()).
		SetMint(pumpV3.USDCMint).
		SetTokenProgram(solana.TokenProgramID).
		Build()
	blockhash, err := state.client.GetLatestBlockhash(requestCtx, rpc.CommitmentFinalized)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("获取区块哈希失败: %v", err)})
		return
	}
	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		blockhash.Value.Blockhash,
		solana.TransactionPayer(state.wallet.PublicKey()),
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建 ATA 交易失败: %v", err)})
		return
	}
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(state.wallet.PublicKey()) {
			return &state.wallet.PrivateKey
		}
		return nil
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("签名 ATA 交易失败: %v", err)})
		return
	}
	signature, err := state.client.SendTransaction(requestCtx, tx)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("提交 ATA 交易失败: %v", err)})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"status":    "submitted",
		"message":   "USDC ATA 创建交易已提交，请稍后刷新状态",
		"ata":       state.ata.String(),
		"signature": signature.String(),
	})
}
