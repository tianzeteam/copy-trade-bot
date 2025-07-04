package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
)

func (b *Bot) fetchBlockhashLoop() {
	go func() {
		for {
			err := b.fetchLatestBlockhash()
			if err != nil {
				b.statusr(err)
				continue
			}
			err = b.fetchBalanceNeeded()
			if err != nil {
				b.statusr(err)
				continue
			}

			time.Sleep(400 * time.Millisecond)
		}
	}()
}

func (b *Bot) fetchLatestBlockhash() error {
	recent, err := b.rpcClient.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return err
	}

	b.Blockhash = &recent.Value.Blockhash
	return nil
}
func (b *Bot) CacheLatestBlockhash(blockhash string) error {
	// 将 Base58 编码的字符串解码为字节数组
	hashBytes, err := base58.Decode(blockhash)
	if err != nil {
		return fmt.Errorf("failed to decode blockhash: %w", err)
	}

	// 确保哈希长度正确
	if len(hashBytes) != 32 {
		return fmt.Errorf("invalid blockhash length: %d, expected 32", len(hashBytes))
	}
	// 创建一个新的 solana.Hash 对象
	hash := solana.HashFromBytes(hashBytes)
	b.Blockhash = &hash
	return nil
}
func (b *Bot) fetchBalanceNeeded() error {
	// 获取最低租金余额
	balanceNeeded, err := b.rpcClient.GetMinimumBalanceForRentExemption(
		context.Background(),
		uint64(165),
		rpc.CommitmentProcessed,
	)
	if err != nil {
		return err
	}

	b.balanceNeeded = balanceNeeded * 12 / 10
	return nil
}
