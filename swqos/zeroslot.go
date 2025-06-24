package swqos

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"copytrade/logger"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
)

// ZeroSlot tip 账户列表
var zeroSlotTipAccounts = []string{
	"4HiwLEP2Bzqj3hM2ENxJuzhcPCdsafwiet3oGkMkuQY4",
	"7toBU3inhmrARGngC7z6SjyP85HgGMmCTEwGNRAcYnEK",
	"8mR3wB1nh4D6J9RUCugxUpc6ya8w38LPxZ3ZjcBhgzws",
	"6SiVU5WEwqfFapRuYCndomztEwDjvS5xgtEof3PLEGm9",
	"TpdxgNJBWZRL8UXF5mrEsyWxDWx9HQexA9P1eTWQ42p",
	"D8f3WkQu6dCF33cZxuAsrKHrGsqGP2yvAHf8mX6RXnwf",
	"GQPFicsy3P3NXxB5piJohoxACqTvWE9fKpLgdsMduoHE",
	"Ey2JEr8hDkgN8qKJGrLf2yFjRhW7rab99HVxwi5rcvJE",
	"4iUgjMT8q2hNZnLuhpqZ1QtiV8deFPy2ajvvjEpKKgsS",
	"3Rz8uD83QsU8wKvZbgWAPvCNDU6Fy8TSZTMcPm3RB6zt",
}

// SendTransactionZeroSlot 发送交易到 0Slot
func SendTransactionZeroSlot(wallet *solana.Wallet, instructions []solana.Instruction, params *types.ZeroslotParams, blockhash solana.Hash) (string, error) {
	logger.Infof("[0Slot] 开始发送交易: %s", time.Now().Format("2006-01-02 15:04:05.000"))

	// 获取随机 tip 账户
	tipAccount := getRandomZeroSlotTipAccount()

	// 构建 tip 转账指令
	tipIx := system.NewTransferInstruction(
		params.TipAmount,
		wallet.PublicKey(),
		solana.MustPublicKeyFromBase58(tipAccount),
	).Build()

	// 合并所有指令
	allInstructions := []solana.Instruction{tipIx}
	allInstructions = append(allInstructions, instructions...)

	// 构建交易
	tx, err := solana.NewTransaction(
		allInstructions,
		blockhash,
		solana.TransactionPayer(wallet.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	// 签名交易
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(wallet.PublicKey()) {
			return &wallet.PrivateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// 选择一个 API Key
	apiKey := selectRandomApiKey(params.ApiKeys)

	// 创建 0Slot 客户端
	zeroSlotClient := rpc.New(fmt.Sprintf("%s?api-key=%s", params.RpcUrl, apiKey))

	// 发送交易
	txSig, err := zeroSlotClient.SendTransactionWithOpts(
		context.Background(),
		tx,
		rpc.TransactionOpts{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	logger.Infof("[0Slot] 交易发送完成: %s", time.Now().Format("2006-01-02 15:04:05.000"))
	return txSig.String(), nil
}

// 获取随机的 0Slot tip 账户
func getRandomZeroSlotTipAccount() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(zeroSlotTipAccounts))))
	if err != nil {
		return zeroSlotTipAccounts[0]
	}
	return zeroSlotTipAccounts[n.Int64()]
}

// 从多个 API Keys 中随机选择一个
func selectRandomApiKey(apiKeys []string) string {
	if len(apiKeys) == 0 {
		return ""
	}

	if len(apiKeys) == 1 {
		return apiKeys[0]
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(apiKeys))))
	if err != nil {
		return apiKeys[0]
	}

	return apiKeys[n.Int64()]
}
