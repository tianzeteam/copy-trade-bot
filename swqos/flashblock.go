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

var FLASHBLOCK_TIP_ADDRESS = []string{
	"FLaShB3iXXTWE1vu9wQsChUKq3HFtpMAhb8kAh1pf1wi",
}

// getRandomTipWallet 随机选择一个tip钱包地址
func getRandomFlashTipWallet() string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(FLASHBLOCK_TIP_ADDRESS))))
	if err != nil {
		// 如果随机数生成失败，使用第一个钱包
		return FLASHBLOCK_TIP_ADDRESS[0]
	}
	return FLASHBLOCK_TIP_ADDRESS[n.Int64()]
}

// FlashBlockTipResponse API响应结构
type FlashBlockTipResponse struct {
	Success bool `json:"success"`
	Data    struct {
		TipAccount string `json:"tipAccount"`
	} `json:"data"`
	Message string `json:"message"`
}

// FlashBlockBatchResponse API响应结构
type FlashBlockBatchResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Signatures []string `json:"signatures"`
	} `json:"data"`
}

// getRandomTipAddress 获取随机tip地址
func getRandomTipAddress(params *types.FlashBlockParams) (string, error) {
	tipURL := params.RpcUrl + "/api/v2/randomTipAddress"

	req, err := http.NewRequestWithContext(context.Background(), "GET", tipURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建tip请求失败: %v", err)
	}

	req.Header.Set("Authorization", params.ApiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取tip地址失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取tip响应失败: %v", err)
	}

	var tipResp FlashBlockTipResponse
	if err := json.Unmarshal(body, &tipResp); err != nil {
		return "", fmt.Errorf("解析tip响应失败: %v", err)
	}

	if !tipResp.Success {
		return "", fmt.Errorf("获取tip地址失败: %s", tipResp.Message)
	}

	return tipResp.Data.TipAccount, nil
}

// SendTransactionFlashBlock 通过 FlashBlock 发送交易束
func SendTransactionFlashBlock(
	wallet *solana.Wallet,
	instructions []solana.Instruction,
	params *types.FlashBlockParams,
	blockhash solana.Hash,
) (string, error) {
	logger.Infof("[FlashBlock] 开始发送交易: %v", time.Now().Format("2006-01-02 15:04:05.000"))

	// 1. 获取随机tip地址
	tipAddress := getRandomFlashTipWallet()

	logger.Debugf("[FlashBlock] 使用tip地址: %s", tipAddress)

	// 2. 解析tip地址
	tipPubkey, err := solana.PublicKeyFromBase58(tipAddress)
	if err != nil {
		return "", fmt.Errorf("解析tip地址失败: %v", err)
	}

	// 3. 计算tip金额
	tipAmount := max(params.TipAmount, MIN_TIP_AMOUNT)

	// 4. 创建tip转账指令
	tipInstruction := system.NewTransferInstruction(
		tipAmount,
		wallet.PublicKey(),
		tipPubkey,
	).Build()

	// 5. 将tip指令添加到交易末尾
	allInstructions := append(instructions, tipInstruction)

	// 6. 构建交易
	tx, err := solana.NewTransaction(
		allInstructions,
		blockhash,
		solana.TransactionPayer(wallet.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("创建交易失败: %v", err)
	}

	// 7. 签名交易
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

	// 8. 序列化交易为 base64
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("序列化交易失败: %v", err)
	}
	encodedTx := base64.StdEncoding.EncodeToString(txBytes)

	// 9. 使用新的submit-batch API
	batchURL := params.RpcUrl + "/api/v2/submit-batch"

	requestBody := map[string]any{
		"transactions": []string{encodedTx},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	logger.Debugf("[FlashBlock] 发送请求到: %s", batchURL)
	logger.Debugf("[FlashBlock] 请求体: %s", string(jsonData))

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(context.Background(), "POST", batchURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", params.ApiKey)

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

	logger.Debugf("[FlashBlock] 响应状态: %s", resp.Status)
	logger.Debugf("[FlashBlock] 响应体: %s", string(body))

	// 解析响应
	var batchResp FlashBlockBatchResponse
	if err := json.Unmarshal(body, &batchResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if !batchResp.Success {
		return "", fmt.Errorf("FlashBlock批量提交失败 [%d]: %s", batchResp.Code, batchResp.Message)
	}

	if len(batchResp.Data.Signatures) == 0 {
		return "", fmt.Errorf("FlashBlock返回空的交易签名")
	}

	// 返回第一个交易签名
	firstSignature := batchResp.Data.Signatures[0]
	logger.Infof("[FlashBlock] 批量交易提交成功 - 签名: %s, 状态: %s",
		firstSignature, batchResp.Message)

	return firstSignature, nil
}
