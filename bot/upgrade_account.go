package bot

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
)

func (b *Bot) UpdateNonceAccount() (*solana.Hash, error) {
	// recent blockhash
	/* 	// 获取最近的区块哈希
	   	ctx := context.Background()
	   	// Use the bot's private key as the fee payer
	   	nonceWallet := b.privateKey

	   	recentBlockhash, err := b.rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	   	if err != nil {
	   		return nil, fmt.Errorf("获取最近区块哈希失败: %v", err)
	   	}

	   	// 创建创建账户的指令
	   	upgradeNonceIns, _ := b.UpgradeNonceAdvanceInstruction(nonceWallet.PublicKey().String())

	   	// 创建交易 - 明确指定费用支付者
	   	tx, err := solana.NewTransaction(
	   		[]solana.Instruction{upgradeNonceIns},
	   		recentBlockhash.Value.Blockhash,
	   		solana.TransactionPayer(b.privateKey.PublicKey()),
	   	)
	   	if err != nil {
	   		return nil, fmt.Errorf("创建交易失败: %v", err)
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
	   		return nil, fmt.Errorf("发送交易失败: %v", err)
	   	}
	   	logger.Info("txhash", sig) */
	return nil, nil

}

// UpgradeNonceAdvanceInstruction 创建一个升级nonce账户的指令
func (b *Bot) UpgradeNonceAdvanceInstruction(nonceAccountAddress string) (solana.Instruction, error) {
	// 解析nonce账户地址
	nonceAccountPubkey, err := solana.PublicKeyFromBase58(nonceAccountAddress)
	if err != nil {
		return nil, fmt.Errorf("解析nonce账户地址失败: %v", err)
	}

	// 系统指令的索引12是升级nonce账户 (UpgradeNonceAccount)
	// 创建指令数据 - 使用小端字节序表示指令索引
	data := []byte{12, 0, 0, 0}

	// 使用自定义指令实现
	return &customInstruction{
		programID: solana.SystemProgramID,
		accounts: []solana.AccountMeta{
			// nonce账户 - 可写
			{PublicKey: nonceAccountPubkey, IsWritable: true, IsSigner: false},
		},
		data: data,
	}, nil
}
