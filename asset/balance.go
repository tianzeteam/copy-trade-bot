package asset

import (
	"context"
	"copytrade/logger"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// GetSolBalance 查询指定钱包的SOL余额
// 参数:
//   - walletAddress: 钱包地址字符串 (base58编码)
//   - rpcUrl: Solana RPC节点URL
// 返回:
//   - float64: SOL余额 (单位: SOL)
//   - error: 错误信息
func GetSolBalance(walletAddress string, rpcUrl string) (float64, error) {
	// 解析钱包地址
	pubKey, err := solana.PublicKeyFromBase58(walletAddress)
	if err != nil {
		logger.Errorf("解析钱包地址失败: %v", err)
		return 0, fmt.Errorf("解析钱包地址失败: %w", err)
	}

	// 创建RPC客户端
	client := rpc.New(rpcUrl)

	// 查询余额
	balance, err := client.GetBalance(
		context.Background(),
		pubKey,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		logger.Errorf("查询SOL余额失败: %v", err)
		return 0, fmt.Errorf("查询SOL余额失败: %w", err)
	}

	// 将lamports转换为SOL (1 SOL = 10^9 lamports)
	solBalance := float64(balance.Value) / 1e9

	return solBalance, nil
}

// GetSolBalanceWithClient 使用已有的RPC客户端查询指定钱包的SOL余额
// 参数:
//   - walletAddress: 钱包地址字符串 (base58编码)
//   - client: 已初始化的Solana RPC客户端
// 返回:
//   - float64: SOL余额 (单位: SOL)
//   - error: 错误信息
func GetSolBalanceWithClient(walletAddress string, client *rpc.Client) (float64, error) {
	// 解析钱包地址
	pubKey, err := solana.PublicKeyFromBase58(walletAddress)
	if err != nil {
		logger.Errorf("解析钱包地址失败: %v", err)
		return 0, fmt.Errorf("解析钱包地址失败: %w", err)
	}

	// 查询余额
	balance, err := client.GetBalance(
		context.Background(),
		pubKey,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		logger.Errorf("查询SOL余额失败: %v", err)
		return 0, fmt.Errorf("查询SOL余额失败: %w", err)
	}

	// 将lamports转换为SOL (1 SOL = 10^9 lamports)
	solBalance := float64(balance.Value) / 1e9

	return solBalance, nil
}
