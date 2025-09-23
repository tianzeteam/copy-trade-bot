package swqos

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"

	"github.com/bloXroute-Labs/solana-trader-client-go/provider"
	pb "github.com/bloXroute-Labs/solana-trader-proto/api"

	"copytrade/types"
)

const BloxTipAddress = "HWEoBxYs7ssKuudEjzjmpfJVX7Dvi7wescFsVx2L5yoY"

var recipient = solana.MustPublicKeyFromBase58(BloxTipAddress)

// BloXrouteParams contains parameters for bloXroute transactions
type BloXrouteParams struct {
	TipAmount  uint64
	TipAccount string
}

// BloXroute 结构体
var bloXrouteClient, _ = provider.NewGRPCClientWithOpts(provider.RPCOpts{
	Endpoint:   "germany.solana.dex.blxrbdn.com:80",
	UseTLS:     false,
	AuthHeader: "YTU2YjIzNjktMzA1Zi00OWU3LTlkMDEtOGIwNWI4ODk5YmQ3OjQ0ZDk2NTAzNDgwNTk0ZGE1NWQ1YTVlYTc4NDhiNTZh",
})

// CreateBloxrouteTipTransactionToUseBundles 创建用于使用 bundles 的 bloXroute tip 交易
func CreateBloxrouteTipTransactionToUseBundles(privateKey solana.PrivateKey, tipAmount uint64, recentBlockHash solana.Hash) (*solana.Transaction, error) {

	tx, err := solana.NewTransaction([]solana.Instruction{
		system.NewTransferInstruction(tipAmount, privateKey.PublicKey(), recipient).Build()}, recentBlockHash)
	if err != nil {
		logger.Errorf("failed to create transaction: %s", err)
		return nil, err
	}

	signatures, err := tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(privateKey.PublicKey()) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		logger.Errorf("failed to sign transaction: %s", err)
		return nil, err
	}

	tx.Signatures = signatures

	return tx, nil
}
func SendTransactionBloXroute(wallet solana.PrivateKey, instructions []solana.Instruction, params types.BloXrouteParams, blockhash solana.Hash) (string, error) {
	logger.Infof("[BloXroute] 开始发送交易: %s", time.Now().Format("2006-01-02 15:04:05.000"))
	// 构建 tip 转账指令
	tipAmount := max(params.TipAmount, MIN_TIP_AMOUNT)
	tipIx := system.NewTransferInstruction(
		tipAmount,
		wallet.PublicKey(),
		recipient,
	).Build()

	// 合并所有指令
	allInstructions := []solana.Instruction{instructions[0], tipIx}
	allInstructions = append(allInstructions, instructions[1:]...)

	// 构建交易
	tx, err := solana.NewTransaction(
		allInstructions,
		blockhash,
		solana.TransactionPayer(wallet.PublicKey()),
	)
	if err != nil {
		logger.Errorf("failed to create transaction: %s", err)
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	// 签名交易
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(wallet.PublicKey()) {
			return &wallet
		}
		return nil
	})
	if err != nil {
		logger.Errorf("failed to sign transaction: %s", err)
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// 序列化交易
	serializedTx, err := tx.MarshalBinary()
	if err != nil {
		logger.Errorf("failed to serialize transaction: %s", err)
		return "", fmt.Errorf("failed to serialize transaction: %w", err)
	}
	base64EncodedTx := base64.StdEncoding.EncodeToString(serializedTx)

	// 发送交易
	opts := provider.PostSubmitOpts{
		SkipPreFlight: false,
	}
	result, err := bloXrouteClient.PostSubmit(context.Background(), &pb.TransactionMessage{Content: base64EncodedTx}, opts)
	if err != nil {
		logger.Errorf("failed to send transaction: %s", err)
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}
	if result == nil {
		logger.Errorf("transaction result is nil")
		return "", fmt.Errorf("transaction result is nil")
	}

	logger.Infof("[BloXroute] 交易发送完成: %s", time.Now().Format("2006-01-02 15:04:05.000"))
	return tx.Signatures[0].String(), nil
}
