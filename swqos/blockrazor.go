package swqos

import (
	"bytes"
	"copytrade/logger"
	"copytrade/types"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
)

// HTTP 客户端，全局共享以复用连接
var (
	blockrazorHttpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	healthCheckMutex sync.Mutex
	isWarmedUp       bool
)

// Blockrazor tip 账户列表
var blockrazorTipAccounts = []string{
	"Gywj98ophM7GmkDdaWs4isqZnDdFCW7B46TXmKfvyqSm",
	"FjmZZrFvhnqqb9ThCuMVnENaM3JGVuGWNyCAxRJcFpg9",
	"6No2i3aawzHsjtThw81iq1EXPJN6rh8eSJCLaYZfKDTG",
	"A9cWowVAiHe9pJfKAj3TJiN9VpbzMUq6E4kEvf5mUT22",
	"68Pwb4jS7eZATjDfhmTXgRJjCiZmw1L7Huy4HNpnxJ3o",
	"4ABhJh5rZPjv63RBJBuyWzBK3g9gWMUQdTZP2kiW31V9",
	"B2M4NG5eyZp5SBQrSdtemzk5TqVuaWGQnowGaCBt8GyM",
	"5jA59cXMKQqZAVdtopv8q3yyw9SYfiE3vUCbt7p8MfVf",
	"5YktoWygr1Bp9wiS1xtMtUki1PeYuuzuCF98tqwYxf61",
	"295Avbam4qGShBYK7E9H5Ldew4B3WyJGmgmXfiWdeeyV",
	"EDi4rSy2LZgKJX74mbLTFk4mxoTgT6F7HxxzG2HBAFyK",
	"BnGKHAC386n4Qmv9xtpBVbRaUTKixjBe3oagkPFKtoy6",
	"Dd7K2Fp7AtoN8xCghKDRmyqr5U169t48Tw5fEd3wT9mq",
	"AP6qExwrbRgBAVaehg4b5xHENX815sMabtBzUzVB4v8S",
}

// Blockrazor API 请求相关结构
type BlockrazorSendRequest struct {
	Transaction string `json:"transaction"`
	Mode        string `json:"mode"`
}

type BlockrazorSendResponse struct {
	Signature string `json:"signature"`
}

type BlockrazorHealthResponse struct {
	Result string `json:"result"`
}

// 获取随机的 Blockrazor tip 账户
func getRandomBlockrazorTipAccount() string {
	if len(blockrazorTipAccounts) == 0 {
		return ""
	}

	if len(blockrazorTipAccounts) == 1 {
		return blockrazorTipAccounts[0]
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(blockrazorTipAccounts))))
	if err != nil {
		return blockrazorTipAccounts[0]
	}
	return blockrazorTipAccounts[n.Int64()]
}

// pingBlockrazorHealth 执行健康检查以保持连接活跃
func pingBlockrazorHealth(healthEndpoint, apiKey string) error {
	req, err := http.NewRequest("GET", healthEndpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", apiKey)

	resp, err := blockrazorHttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取完整响应体以启用连接复用
	bodyBytes, _ := io.ReadAll(resp.Body)
	var healthRes BlockrazorHealthResponse
	if err := json.Unmarshal(bodyBytes, &healthRes); err != nil {
		return fmt.Errorf("decode error: %v", err)
	}

	return nil
}

// ensureBlockrazorConnectionWarmed 确保连接已预热
func ensureBlockrazorConnectionWarmed(params *types.BlockrazorParams) {
	healthCheckMutex.Lock()
	defer healthCheckMutex.Unlock()

	if !isWarmedUp {
		// 执行初始健康检查以建立 HTTP 连接
		err := pingBlockrazorHealth(params.HealthEndpoint, params.ApiKey)
		if err != nil {
			logger.Errorf("[Blockrazor] 健康检查失败: %v", err)
		} else {
			isWarmedUp = true
		}

		// 启动后台 goroutine 定期发送健康检查请求
		go func() {
			for {
				time.Sleep(30 * time.Second)
				err := pingBlockrazorHealth(params.HealthEndpoint, params.ApiKey)
				if err != nil {
					logger.Errorf("[Blockrazor] 健康检查失败: %v", err)
				}
			}
		}()
	}
}

// SendTransactionBlockrazor 发送交易到 Blockrazor
func SendTransactionBlockrazor(wallet *solana.Wallet, instructions []solana.Instruction, params *types.BlockrazorParams, blockhash solana.Hash) (string, error) {
	logger.Infof("[Blockrazor] 开始发送交易: %s", time.Now().Format("2006-01-02 15:04:05.000000"))

	// 确保连接已预热
	//ensureBlockrazorConnectionWarmed(params)
	// 获取随机 tip 账户
	tipAccount := getRandomBlockrazorTipAccount()
	var tipIx solana.Instruction

	// 如果有 tip 账户且 tip 金额大于 0，则添加 tip 转账指令
	if tipAccount != "" && params.TipAmount > 0 {
		tipAmount := max(params.TipAmount, MIN_TIP_AMOUNT)
		tipPub := solana.MustPublicKeyFromBase58(tipAccount)
		tipIx = system.NewTransferInstruction(
			tipAmount,
			wallet.PublicKey(),
			tipPub,
		).Build()

		// 合并所有指令
		allInstructions := []solana.Instruction{instructions[0], tipIx}
		allInstructions = append(allInstructions, instructions[1:]...)
		instructions = allInstructions
	}

	// 构建交易
	tx, err := solana.NewTransaction(
		instructions,
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
	txBase64, err := tx.ToBase64()
	if err != nil {
		return "", fmt.Errorf("failed to encode transaction: %w", err)
	}

	// 构造请求体
	reqBody := BlockrazorSendRequest{
		Transaction: txBase64,
		Mode:        params.Mode,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	httpReq, err := http.NewRequest("POST", params.RpcUrl, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", params.ApiKey)

	// 发送请求
	resp, err := blockrazorHttpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("send http error: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// 检查响应状态码
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应
	var sendRes BlockrazorSendResponse
	if err := json.Unmarshal(bodyBytes, &sendRes); err != nil {
		return "", fmt.Errorf("decode error: %v", err)
	}

	logger.Infof("[Blockrazor] 交易发送完成: %s, 签名: %s", time.Now().Format("2006-01-02 15:04:05.000"), sendRes.Signature)
	return sendRes.Signature, nil
}
