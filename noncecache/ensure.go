package noncecache

import (
	"context"
	"errors"
	"fmt"

	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
)

// EnsureNonceAccount 按“配置地址优先”确保 nonce 账户可用：
//  1. 配置的 nonce_account_pubkey 已初始化且 authority 等于交易钱包 → 直接复用；
//  2. 地址为空、不存在、未初始化或 authority 不匹配：
//     - createIfMissing=false → 返回错误；
//     - createIfMissing=true  → 创建并初始化新的 nonce 账户。
//
// 与旧 Initialize 不同，本方法不读取 authority_pubkey.txt / nonce_pubkey.txt，
// 也不会在复用时额外 AdvanceNonce，避免影响正在使用同一 nonce 的其它进程。
func (nm *NonceManager) EnsureNonceAccount(ctx context.Context, tradeWalletKey string, createIfMissing bool) (bool, error) {
	wallet, err := solana.PrivateKeyFromBase58(tradeWalletKey)
	if err != nil {
		return false, fmt.Errorf("解析交易钱包私钥失败: %w", err)
	}
	walletPubkey := wallet.PublicKey()
	nm.authorityPubkey = &walletPubkey

	// 交易钱包本身必须是普通系统账户，否则创建 ATA/转账会失败。
	if err := ValidateTradeWallet(ctx, nm.rpcClient, walletPubkey); err != nil {
		return false, err
	}

	configured := solana.PublicKey{}
	if nm.nonceAccountPubkey != nil {
		configured = *nm.nonceAccountPubkey
	}

	if !configured.IsZero() {
		validateErr := ValidateNonceAccountStrict(ctx, nm.rpcClient, configured, walletPubkey)
		if validateErr == nil {
			nm.nonceAccountPubkey = &configured
			nm.nonceAccountAddress = configured.String()
			if err := nm.RefreshGlobalBlockhash(ctx); err != nil {
				return false, fmt.Errorf("复用 nonce 后读取 blockhash 失败: %w", err)
			}
			logger.Infof("复用已配置的 nonce 账户: %s (authority: %s)", configured, walletPubkey)
			return false, nil
		}
		if errors.Is(validateErr, ErrNonceValidationUnavailable) {
			return false, fmt.Errorf("无法确认已配置 nonce 的状态，暂不创建新账户: %w", validateErr)
		}
		if !createIfMissing {
			return false, fmt.Errorf("配置的 nonce 账户不可用，且自动创建已关闭: %w", validateErr)
		}
		logger.Warnf("配置的 nonce 账户不可用，将创建新账户: %s: %v", configured, validateErr)
	} else if !createIfMissing {
		return false, fmt.Errorf("nonce_account_pubkey 为空，且自动创建已关闭")
	}

	created, err := nm.CreateNonceAccount(ctx, tradeWalletKey)
	if err != nil {
		return false, err
	}
	nm.nonceAccountPubkey = created
	nm.nonceAccountAddress = created.String()
	if err := nm.RefreshGlobalBlockhash(ctx); err != nil {
		return false, fmt.Errorf("新 nonce 创建后读取 blockhash 失败: %w", err)
	}
	logger.Infof("已创建并初始化新的 nonce 账户: %s (authority: %s)", created, walletPubkey)
	return true, nil
}
