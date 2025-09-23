package swqos

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"copytrade/logger"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

// Temporal tip 账户列表
var temporalTipAccounts = []string{
	"mwGELGMgGGrNL1UibNCQeJHDE7qdPptWRYB6noUHmTj",
}

// 获取随机的 tip 账户
func getRandomTemporalTipAccount() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(temporalTipAccounts))))
	if err != nil {
		return temporalTipAccounts[0]
	}
	return temporalTipAccounts[n.Int64()]
}

// SendTransactionTemporal 发送交易到 Temporal
func SendTransactionTemporal(wallet *solana.Wallet, instructions []solana.Instruction, params *types.TemporalParams, blockhash solana.Hash) (string, error) {
	logger.Infof("[Temporal] 开始发送交易: %s", time.Now().Format("2006-01-02 15:04:05.000000"))

	// 获取随机 tip 账户
	tipAccount := getRandomTemporalTipAccount()

	// 构建 tip 转账指令
	tipIx := system.NewTransferInstruction(
		params.TipAmount,
		wallet.PublicKey(),
		solana.MustPublicKeyFromBase58(tipAccount),
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

	// 转为 Base64
	tx64Str := tx.MustToBase64()

	// 构造请求
	type JsonRpcRequest struct {
		Jsonrpc string `json:"jsonrpc"`
		Id      int    `json:"id"`
		Method  string `json:"method"`
		Params  []any  `json:"params"`
	}

	payload := JsonRpcRequest{
		Jsonrpc: "2.0",
		Id:      1,
		Method:  "sendTransaction",
		Params: []any{
			tx64Str,
			map[string]string{"encoding": "base64"},
		},
	}

	// 序列化为 JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 创建 HTTP 请求
	reqUrl := fmt.Sprintf("%s?c=%s", params.BundleUrl, params.ApiKey)
	req, err := http.NewRequest("POST", reqUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// 检查响应
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	logger.Infof("[Temporal] 交易发送完成: %s", time.Now().Format("2006-01-02 15:04:05.000000"))
	return tx.Signatures[0].String(), nil
}
