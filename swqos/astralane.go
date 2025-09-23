package swqos

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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

const (
	// Minimum tip amount for spam prevention (100,000 lamports = 0.0001 SOL)
	MIN_TIP_AMOUNT = 100000
)

// Astralane tip wallet addresses - multiple wallets for load balancing
var ANS_TIP_WALLETS = []string{
	"astrazznxsGUhWShqgNtAdfrzP2G83DzcWVJDxwV9bF",
	"astra4uejePWneqNaJKuFFA8oonqCE1sqF6b45kDMZm",
	"astra9xWY93QyfG6yM8zwsKsRodscjQ2uU2HKNL5prk",
	"astraRVUuTHjpwEVvNBeQEgwYx9w9CFyfxjYoobCZhL",
	"astraEJ2fEj8Xmy6KLG7B3VfbKfsHXhHrNdCQx7iGJK",
	"astraubkDw81n4LuutzSQ8uzHCv4BhPVhfvTcYv8SKC",
	"astraZW5GLFefxNPAatceHhYjfA1ciq9gvfEg2S47xk",
	"astrawVNP4xDBKT7rAdxrLYiTSTdqtUr63fSMduivXK",
}

// getRandomTipWallet 随机选择一个tip钱包地址
func getRandomTipWallet() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(ANS_TIP_WALLETS))))
	if err != nil {
		// 如果随机数生成失败，使用第一个钱包
		return ANS_TIP_WALLETS[0]
	}
	return ANS_TIP_WALLETS[n.Int64()]
}

// SendTransactionAns 通过 Astralane 发送交易
func SendTransactionAns(
	wallet *solana.Wallet,
	instructions []solana.Instruction,
	params *types.AstralaneParams,
	blockhash solana.Hash,
) (string, error) {
	logger.Infof("[ANS] 开始发送交易: %v", time.Now().Format("2006-01-02 15:04:05.000"))

	// 添加 tip 指令到交易末尾
	tipWalletAddress := getRandomTipWallet()
	tipWallet, err := solana.PublicKeyFromBase58(tipWalletAddress)
	if err != nil {
		return "", fmt.Errorf("解析tip钱包地址失败: %v", err)
	}

	logger.Debugf("[ANS] 使用tip钱包: %s", tipWalletAddress)

	tipAmount := max(params.TipAmount, MIN_TIP_AMOUNT)

	// 创建 tip 转账指令
	tipInstruction := system.NewTransferInstruction(
		tipAmount,
		wallet.PublicKey(),
		tipWallet,
	).Build()

	// 将 tip 指令添加到指令列表末尾
	allInstructions := append(instructions, tipInstruction)

	// 构建交易
	tx, err := solana.NewTransaction(
		allInstructions,
		blockhash,
		solana.TransactionPayer(wallet.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("创建交易失败: %v", err)
	}

	// 签名交易
	_, err = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(wallet.PublicKey()) {
				return &wallet.PrivateKey
			}
			return nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("签名交易失败: %v", err)
	}

	// 序列化交易为 base64
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("序列化交易失败: %v", err)
	}
	encodedTx := base64.StdEncoding.EncodeToString(txBytes)

	// 构建 JSON-RPC 请求
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []any{
			encodedTx,
			map[string]any{
				"encoding":      "base64",
				"skipPreflight": true,
			},
			map[string]any{
				"mevProtect": true, // 启用 MEV 保护
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	logger.Debugf("[ANS] 发送请求到: %s", params.RpcUrl)
	logger.Debugf("[ANS] 请求体: %s", string(jsonData))

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(context.Background(), "POST", params.RpcUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_key", params.ApiKey)

	// 发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	logger.Debugf("[ANS] 响应状态: %s", resp.Status)
	logger.Debugf("[ANS] 响应体: %s", string(body))

	// 解析响应
	var response struct {
		Result string `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if response.Error != nil {
		return "", fmt.Errorf("ANS RPC错误 [%d]: %s", response.Error.Code, response.Error.Message)
	}

	if response.Result == "" {
		return "", fmt.Errorf("ANS返回空的交易签名")
	}

	logger.Infof("[ANS] 交易发送成功: %s", response.Result)
	return response.Result, nil
}
