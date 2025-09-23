package noncecache

import (
	"context"
	"fmt"
	"time"

	"copytrade/logger"
	"copytrade/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/mr-tron/base58"
)

const NONCE_ACCOUNT_LENGTH = 80

// CreateAccount 创建一个基础账户
func (b *NonceManager) CreateAccount(ctx context.Context, payer solana.PrivateKey, newAccount solana.Wallet, accountSize uint64, owner solana.PublicKey) (string, error) {
	// 获取租金豁免所需的最小余额
	minimumBalance, err := b.rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		accountSize,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		return "", fmt.Errorf("获取租金豁免最小余额失败: %v", err)
	}
	logger.Infof("账户所需最小余额: %d lamports", minimumBalance)

	// 检查付款人余额
	balance, err := b.rpcClient.GetBalance(ctx, payer.PublicKey(), rpc.CommitmentConfirmed)
	if err != nil {
		return "", fmt.Errorf("获取付款人余额失败: %v", err)
	}
	logger.Infof("付款人余额: %d lamports", balance.Value)

	if balance.Value < minimumBalance+5000 { // 额外预留5000 lamports作为交易费
		return "", fmt.Errorf("付款人余额不足: 当前 %d lamports, 需要至少 %d lamports", balance.Value, minimumBalance+5000)
	}

	// 获取最近的区块哈希
	recentBlockhash, err := b.rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return "", fmt.Errorf("获取最近区块哈希失败: %v", err)
	}

	// 创建账户指令
	createAccountIx := system.NewCreateAccountInstruction(
		uint64(minimumBalance),
		accountSize,
		owner,
		payer.PublicKey(),
		newAccount.PublicKey(),
	).Build()

	// 创建交易
	createTx, err := solana.NewTransaction(
		[]solana.Instruction{createAccountIx},
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("创建账户交易失败: %v", err)
	}

	// 签名交易
	createTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer
		}
		if key.Equals(newAccount.PublicKey()) {
			return &newAccount.PrivateKey
		}
		return nil
	})

	// 发送交易
	logger.Infof("发送创建账户交易...")
	createSig, err := b.rpcClient.SendTransactionWithOpts(
		ctx,
		createTx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentConfirmed,
		},
	)
	if err != nil {
		return "", fmt.Errorf("发送创建账户交易失败: %v", err)
	}
	logger.Infof("创建账户交易发送成功: %s", createSig.String())

	return createSig.String(), nil
}

// InitializeNonceAccount 初始化nonce账户
func (b *NonceManager) InitializeNonceAccount(ctx context.Context, payer solana.PrivateKey, nonceAccount solana.PublicKey, authority solana.PublicKey) (string, error) {
	// 获取最近的区块哈希
	recentBlockhash, err := b.rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return "", fmt.Errorf("获取最近区块哈希失败: %v", err)
	}

	// 使用正确参数顺序的InitializeNonceAccount指令
	// 正确的参数顺序: authority, nonce_account, recent_blockhashes, rent
	initNonceIx := system.NewInitializeNonceAccountInstruction(
		authority,                            // 授权者
		nonceAccount,                         // nonce账户
		solana.SysVarRecentBlockHashesPubkey, // 近期区块哈希系统变量
		solana.SysVarRentPubkey,              // 租金系统变量
	).Build()

	// 添加调试信息
	logger.Infof("初始化指令账户数量: %d", len(initNonceIx.Accounts()))
	for i, acc := range initNonceIx.Accounts() {
		logger.Infof("账户 %d: %s (writable: %v, signer: %v)", i, acc.PublicKey.String(), acc.IsWritable, acc.IsSigner)
	}

	// 创建交易
	initTx, err := solana.NewTransaction(
		[]solana.Instruction{initNonceIx},
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return "", fmt.Errorf("创建初始化交易失败: %v", err)
	}

	// 签名交易 - 需要authority签名，不是payer
	initTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer
		}
		if key.Equals(authority) {
			return &payer // authority和payer是同一个账户
		}
		return nil
	})

	// 发送交易
	logger.Infof("发送初始化nonce账户交易...")
	initSig, err := b.rpcClient.SendTransactionWithOpts(
		ctx,
		initTx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentConfirmed,
		},
	)
	if err != nil {
		return "", fmt.Errorf("发送初始化交易失败: %v", err)
	}
	logger.Infof("初始化nonce账户交易发送成功: %s", initSig.String())

	return initSig.String(), nil
}

// GetRecentBlockhash 获取最近的区块哈希
func (b *NonceManager) GetRecentBlockhash(ctx context.Context) (solana.Hash, error) {
	recentBlockhash, err := b.rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return solana.Hash{}, fmt.Errorf("获取最近区块哈希失败: %v", err)
	}
	return recentBlockhash.Value.Blockhash, nil
}

// CreateNonceAccount 创建一个新的nonce账户（调用上面两个函数）
func (b *NonceManager) CreateNonceAccount(ctx context.Context, config *utils.Config) (*solana.PublicKey, error) {
	// 生成一个新的密钥对作为nonce账户
	tradeWallet := solana.MustPrivateKeyFromBase58(config.PrivateKey)
	nonceAccount := solana.NewWallet()
	nonceAccountPubkey := nonceAccount.PublicKey()
	logger.Infof("创建nonce账户: %s", nonceAccountPubkey.String())

	// 第一步：创建账户
	createSig, err := b.CreateAccount(ctx, tradeWallet, *nonceAccount, NONCE_ACCOUNT_LENGTH, system.ProgramID)
	if err != nil {
		return nil, fmt.Errorf("创建账户失败: %v", err)
	}

	// 等待账户创建确认并验证账户存在
	logger.Infof("等待账户创建确认...")
	time.Sleep(5 * time.Second)

	// 验证账户是否已创建
	for i := range 20 {
		accountInfo, err := b.rpcClient.GetAccountInfo(ctx, nonceAccountPubkey)
		if err == nil && accountInfo.Value != nil {
			logger.Infof("账户创建确认成功，账户余额: %d lamports", accountInfo.Value.Lamports)
			logger.Infof("账户所有者: %s", accountInfo.Value.Owner.String())
			logger.Infof("账户数据长度: %d bytes", len(accountInfo.Value.Data.GetBinary()))

			// 检查账户数据是否为空（nonce账户初始化需要空数据）
			data := accountInfo.Value.Data.GetBinary()
			allZero := true
			for _, b := range data {
				if b != 0 {
					allZero = false
					break
				}
			}
			logger.Infof("账户数据是否全为零: %v", allZero)
			break
		}
		if i == 19 {
			return nil, fmt.Errorf("账户创建确认超时，账户可能未成功创建")
		}
		logger.Infof("等待账户创建确认... (%d/20)", i+1)
		time.Sleep(2 * time.Second)
	}

	// 第二步：初始化nonce账户
	initSig, err := b.InitializeNonceAccount(ctx, tradeWallet, nonceAccountPubkey, tradeWallet.PublicKey())
	if err != nil {
		return nil, fmt.Errorf("初始化nonce账户失败: %v", err)
	}

	logger.Infof("nonce账户创建成功!")
	logger.Infof("创建账户交易: %s", createSig)
	logger.Infof("初始化交易: %s", initSig)
	logger.Infof("nonce账户地址: %s", nonceAccountPubkey.String())

	return &nonceAccountPubkey, nil
}

// GetNonce 获取nonce账户的信息
func (b *NonceManager) GetNonce(ctx context.Context, nonceAccountAddress string) (*solana.Hash, error) {

	nonce, err := b.nonceClient.GetNonceFromNonceAccount(ctx, nonceAccountAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce account: %v", err)
	}

	//logger.Infof("nonce::  %s", nonce)

	// Check if nonce is empty
	if nonce == "" {
		return nil, fmt.Errorf("nonce account returned empty string, account may not be properly initialized")
	}

	// Decode the base58-encoded nonce string to bytes
	nonceBytes, err := base58.Decode(nonce)
	if err != nil {
		return nil, fmt.Errorf("failed to decode nonce: %v", err)
	}
	nonceValue := solana.HashFromBytes(nonceBytes)
	return &nonceValue, nil
}

// AdvanceNonceAccount 推进nonce账户
func (b *NonceManager) AdvanceNonceAccount(ctx context.Context, recentBlockhash solana.Hash, nonceAccountAddress string, authorizer solana.PrivateKey) error {
	// 解析nonce账户地址
	nonceAccountPubkey, err := solana.PublicKeyFromBase58(nonceAccountAddress)
	if err != nil {
		return fmt.Errorf("解析nonce账户地址失败: %v", err)
	}

	// 创建nonce advance指令
	advanceNonceIx := system.NewAdvanceNonceAccountInstruction(
		nonceAccountPubkey,                   // nonce账户
		solana.SysVarRecentBlockHashesPubkey, // 近期区块哈希系统变量
		authorizer.PublicKey(),               // 授权者（签名者）
	).Build()

	// 创建交易
	advanceTx, err := solana.NewTransaction(
		[]solana.Instruction{advanceNonceIx},
		recentBlockhash,
		solana.TransactionPayer(authorizer.PublicKey()),
	)
	if err != nil {
		return fmt.Errorf("创建advance nonce交易失败: %v", err)
	}

	// 签名交易
	advanceTx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(authorizer.PublicKey()) {
			return &authorizer
		}
		return nil
	})

	// 发送交易
	logger.Infof("发送advance nonce账户交易...")
	advanceSig, err := b.rpcClient.SendTransactionWithOpts(
		ctx,
		advanceTx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentConfirmed,
		},
	)
	if err != nil {
		return fmt.Errorf("发送advance nonce交易失败: %v", err)
	}
	logger.Infof("advance nonce账户交易发送成功: %s", advanceSig.String())

	return nil
}
