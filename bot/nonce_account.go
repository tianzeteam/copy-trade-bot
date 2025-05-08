package bot

import (
	"context"
	"fmt"
	"time"

	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"

	"github.com/mr-tron/base58"
)

// CreateNonceAccount 创建一个新的nonce账户
func (b *Bot) CreateNonceAccount(ctx context.Context) (*solana.PrivateKey, string, error) {
	// 生成一个新的密钥对作为nonce账户
	nonceAccount := solana.NewWallet()
	nonceAccountPubkey := nonceAccount.PublicKey()
	logger.Infof("nonce account: %s", nonceAccountPubkey.String())

	// 获取租金豁免所需的最小余额
	minimumBalance, err := b.rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		NONCE_ACCOUNT_LENGTH,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		return nil, "", fmt.Errorf("获取租金豁免最小余额失败: %v", err)
	}

	// 获取最近的区块哈希
	recentBlockhash, err := b.rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, "", fmt.Errorf("获取最近区块哈希失败: %v", err)
	}

	// 创建创建账户的指令
	createAccountIx := system.NewCreateAccountInstruction(
		uint64(minimumBalance),
		NONCE_ACCOUNT_LENGTH,
		system.ProgramID,
		b.privateKey.PublicKey(), // 付款人
		nonceAccountPubkey,       // nonce账户
	).Build()

	// 创建交易
	tx, err := solana.NewTransaction(
		[]solana.Instruction{createAccountIx},
		recentBlockhash.Value.Blockhash,
	)
	if err != nil {
		return nil, "", fmt.Errorf("创建交易失败: %v", err)
	}

	// 签名交易
	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(b.privateKey.PublicKey()) {
			return &b.privateKey
		}

		return nil
	})

	// 发送交易
	sig, err := b.rpcClient.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return nil, "", fmt.Errorf("发送交易失败: %v", err)
	}

	// 等待一段时间确保交易被确认
	time.Sleep(5 * time.Second)

	logger.Infof("创建nonce账户成功，交易ID: %s", sig.String())
	return &nonceAccount.PrivateKey, nonceAccountPubkey.String(), nil
}

// GetNonce 获取nonce账户的信息
func (b *Bot) GetNonce(ctx context.Context, nonceAccountAddress string) (*solana.Hash, error) {
	// 解析nonce账户地址
	nonceAccountPubkey, err := solana.PublicKeyFromBase58(nonceAccountAddress)
	if err != nil {
		return nil, fmt.Errorf("解析nonce账户地址失败: %v", err)
	}

	// 获取租金豁免所需的最小余额
	_, err = b.rpcClient.GetMinimumBalanceForRentExemption(
		ctx,
		NONCE_ACCOUNT_LENGTH,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		logger.Fatalf("GetMinimumBalanceForRentExemption : %v", err)

	}
	/*
		nonce, err := b.nonceClient.GetNonceFromNonceAccount(ctx, nonceAccountAddress)
		if err != nil {
			logger.Fatalf("failed to get nonce account, err: %v", err)
		} */

	offset := uint64(40)
	length := uint64(32)
	accountResult, err := b.rpcClient.GetAccountInfoWithOpts(ctx, nonceAccountPubkey, &rpc.GetAccountInfoOpts{
		DataSlice: &rpc.DataSlice{
			Offset: &offset,
			Length: &length,
		},
	})
	if err != nil {
		logger.Fatalf("failed to get nonce account, err: %v", err)
	}
	// Extract binary data from DataBytesOrJSON before encoding
	data := accountResult.GetBinary()
	nonceValue := solana.HashFromBytes(data[:])

	logger.Infof("nonce::  %s", base58.Encode(data))

	// 获取账户信息
	/* accountInfo, err := b.rpcClient.GetAccountInfo(ctx, nonceAccountPubkey)
	if err != nil {
		return nil, fmt.Errorf("获取nonce账户信息失败: %v", err)
	}

	// 由于当前库可能没有直接的NonceAccount解析函数，我们只显示原始数据
	if accountInfo != nil && accountInfo.Value != nil && accountInfo.Value.Data != nil {
		data := accountInfo.Value.Data.GetBinary()
		logger.Infof("Nonce账户数据长度: %d 字节", len(data))

		// 如果有足够的数据，可以尝试解析出授权者和nonce值
		// 注意：这里的偏移量可能需要根据实际的数据结构进行调整
		if len(data) >= 80 {
			// 尝试解析授权者公钥（假设在偏移量4后的32字节）
			var authPubkeyBytes [32]byte
			copy(authPubkeyBytes[:], data[8:40])
			authPubkey := solana.PublicKeyFromBytes(authPubkeyBytes[:])

			// 尝试解析nonce值（假设在授权者公钥后的32字节）
			var nonceBytes [32]byte
			copy(nonceBytes[:], data[40:72])
			nonceValue := solana.HashFromBytes(nonceBytes[:])

			logger.Infof("授权者: %s", authPubkey.String())
			logger.Infof("Nonce值: %s", nonceValue.String())

			return &nonceValue, nil
		} else {
			return nil, fmt.Errorf("Nonce账户数据长度不足，无法解析")
		}
	} else {
		logger.Warnf("获取到的账户信息为空")
	}
	*/
	return &nonceValue, nil
}

// GetNonceAdvanceInstruction 创建一个推进使用nonce的指令
func (b *Bot) GetNonceAdvanceInstruction(nonceAccountAddress string) (solana.Instruction, error) {
	// 解析nonce账户地址
	nonceAccountPubkey, err := solana.PublicKeyFromBase58(nonceAccountAddress)
	if err != nil {
		return nil, fmt.Errorf("解析nonce账户地址失败: %v", err)
	}
	// 系统程序的ID
	systemProgramID := solana.SystemProgramID

	// 由于当前库可能没有直接的NonceAdvance指令，我们手动实现
	// Nonce Advance指令的索引是4
	data := []byte{4, 0, 0, 0}

	// 创建自定义指令
	return &customInstruction{
		programID: systemProgramID,
		accounts: []solana.AccountMeta{
			// nonce账户 - 可写
			{PublicKey: nonceAccountPubkey, IsWritable: true, IsSigner: false},
			// 近期区块哈希账户 - 只读
			{PublicKey: solana.SysVarRecentBlockHashesPubkey, IsWritable: false, IsSigner: false},
			// 授权者 - 签名者
			{PublicKey: b.privateKey.PublicKey(), IsWritable: false, IsSigner: true},
		},
		data: data,
	}, nil
}

// customInstruction 自定义指令实现solana.Instruction接口
type customInstruction struct {
	programID solana.PublicKey
	accounts  []solana.AccountMeta
	data      []byte
}

// ProgramID 实现solana.Instruction接口
func (ci *customInstruction) ProgramID() solana.PublicKey {
	return ci.programID
}

// Accounts 实现solana.Instruction接口
func (ci *customInstruction) Accounts() []*solana.AccountMeta {
	// 转换为指针切片
	accountPtrs := make([]*solana.AccountMeta, len(ci.accounts))
	for i := range ci.accounts {
		accountPtrs[i] = &ci.accounts[i]
	}
	return accountPtrs
}

// Data 实现solana.Instruction接口
func (ci *customInstruction) Data() ([]byte, error) {
	return ci.data, nil
}
