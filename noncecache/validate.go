package noncecache

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"copytrade/logger"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// nonce账户数据的bincode布局（共80字节）：
//
//	[0:4]   u32 版本号（当前为0）
//	[4:8]   u32 状态（0=未初始化，1=已初始化）
//	[8:40]  authority 公钥
//	[40:72] 当前blockhash
//	[72:80] u64 lamports_per_signature
const (
	nonceStateUninitialized = 0
	nonceStateInitialized   = 1
	nonceAuthorityOffset    = 8
	nonceStateOffset        = 4
)

// decodeNonceState 解析nonce账户数据，返回状态与authority。
func decodeNonceState(data []byte) (uint32, solana.PublicKey, error) {
	if len(data) != NONCE_ACCOUNT_LENGTH {
		return 0, solana.PublicKey{}, fmt.Errorf("nonce账户数据长度不正确，期望: %d, 实际: %d", NONCE_ACCOUNT_LENGTH, len(data))
	}
	state := binary.LittleEndian.Uint32(data[nonceStateOffset:8])
	authority := solana.PublicKeyFromBytes(data[nonceAuthorityOffset:40])
	return state, authority, nil
}

// ValidateTradeWallet 校验交易钱包是一个“干净”的系统账户（无数据）。
//
// 背景：系统程序的 Transfer 指令拒绝从携带数据的账户转出
// （"Transfer: `from` must not carry data"，即nonce账户保护）。
// 若把nonce账户（或其他带数据的账户）的密钥对当作交易钱包写入配置，
// 所有 ATA 创建（ATA程序内部通过 system transfer 注资）与普通SOL转账
// 都会在模拟/执行阶段失败，且报错信息非常隐晦，因此在启动时提前拦截。
//
// 钱包账户不存在（全新的未入金钱包）视为合法：空账户没有数据。
// RPC网络错误仅告警不阻断启动，只有明确的链上证据才判定失败。
func ValidateTradeWallet(ctx context.Context, rpcClient *rpc.Client, wallet solana.PublicKey) error {
	accountInfo, err := rpcClient.GetAccountInfo(ctx, wallet)
	if err != nil {
		if isNotFoundErr(err) {
			// 账户不存在：干净的空账户，合法（但无法支付任何费用，仅提示）
			logger.Warnf("交易钱包 %s 在链上不存在（余额为0），入金前无法交易", wallet.String())
			return nil
		}
		// 网络等临时错误：告警放行，避免RPC抖动导致无法启动
		logger.Warnf("查询交易钱包账户信息失败，跳过钱包校验: %v", err)
		return nil
	}
	if accountInfo == nil || accountInfo.Value == nil {
		logger.Warnf("交易钱包 %s 在链上不存在（余额为0），入金前无法交易", wallet.String())
		return nil
	}

	value := accountInfo.Value
	data := value.Data.GetBinary()
	isSystemOwned := value.Owner.Equals(solana.SystemProgramID)

	if isSystemOwned && len(data) == 0 {
		logger.Infof("交易钱包校验通过: %s (余额: %d lamports)", wallet.String(), value.Lamports)
		return nil
	}

	// 带数据账户：进一步判断是否被初始化成了nonce账户，给出针对性提示
	if isSystemOwned && len(data) == NONCE_ACCOUNT_LENGTH {
		if state, authority, derr := decodeNonceState(data); derr == nil && state == nonceStateInitialized {
			return fmt.Errorf(
				"交易钱包 %s 本身是一个已初始化的nonce账户 (authority: %s)。"+
					"nonce账户携带数据，系统程序禁止从其转出SOL (\"Transfer: `from` must not carry data\")，"+
					"创建ATA、转账、买入出资都会失败；其中lamports只能由authority通过WithdrawNonceAccount取出。"+
					"请在config.yaml的private_key中配置一个普通钱包（非nonce账户）的密钥对并重新入金",
				wallet.String(), authority.String())
		}
	}
	return fmt.Errorf(
		"交易钱包 %s 不是普通系统账户 (owner: %s, 数据长度: %d)，携带数据的账户无法作为出资方创建ATA或转出SOL，"+
			"请在config.yaml的private_key中配置一个普通钱包的密钥对",
		wallet.String(), value.Owner.String(), len(data))
}

// ValidateNonceAccount 校验nonce账户可用于当前交易钱包：
// 已初始化，且authority等于交易钱包（AdvanceNonce必须由authority签名）。
func ValidateNonceAccount(ctx context.Context, rpcClient *rpc.Client, nonceAccount, wallet solana.PublicKey) error {
	accountInfo, err := rpcClient.GetAccountInfo(ctx, nonceAccount)
	if err != nil || accountInfo == nil || accountInfo.Value == nil {
		if isNotFoundErr(err) || (err == nil && (accountInfo == nil || accountInfo.Value == nil)) {
			return fmt.Errorf("nonce账户 %s 不存在，无法用于AdvanceNonce；请修正config.yaml的nonce_account_pubkey，"+
				"或设置skip_create_nonce_account=false由程序自动创建", nonceAccount.String())
		}
		logger.Warnf("查询nonce账户信息失败，跳过nonce校验: %v", err)
		return nil
	}

	value := accountInfo.Value
	if !value.Owner.Equals(solana.SystemProgramID) {
		return fmt.Errorf("nonce账户 %s 所有者不正确，期望: %s, 实际: %s",
			nonceAccount.String(), solana.SystemProgramID.String(), value.Owner.String())
	}

	state, authority, err := decodeNonceState(value.Data.GetBinary())
	if err != nil {
		return fmt.Errorf("nonce账户 %s 校验失败: %w", nonceAccount.String(), err)
	}
	if state != nonceStateInitialized {
		return fmt.Errorf("nonce账户 %s 未初始化 (state=%d)，无法用于AdvanceNonce；"+
			"请修正config.yaml的nonce_account_pubkey，或设置skip_create_nonce_account=false由程序自动创建",
			nonceAccount.String(), state)
	}
	if !authority.Equals(wallet) {
		return fmt.Errorf("nonce账户 %s 的authority是 %s，而不是当前交易钱包 %s。"+
			"AdvanceNonce必须由authority签名，当前配置下所有交易都会失败(MissingRequiredSignature)；"+
			"请修正config.yaml的nonce_account_pubkey为authority等于交易钱包的nonce账户，"+
			"或设置skip_create_nonce_account=false由程序自动创建",
			nonceAccount.String(), authority.String(), wallet.String())
	}

	logger.Infof("nonce账户校验通过: %s (authority: %s, 余额: %d lamports)",
		nonceAccount.String(), authority.String(), value.Lamports)
	return nil
}

// ValidateStartupAccounts 启动时统一校验交易钱包与nonce账户的链上状态一致性。
// 任何一项存在明确的链上证据表明配置错误时返回error，由调用方决定是否退出。
func ValidateStartupAccounts(ctx context.Context, rpcURL, privateKey string, nonceAccount *solana.PublicKey) error {
	walletKey, err := solana.PrivateKeyFromBase58(privateKey)
	if err != nil {
		return fmt.Errorf("解析交易钱包私钥失败: %w", err)
	}
	wallet := walletKey.PublicKey()

	rpcClient := rpc.New(rpcURL)
	if err := ValidateTradeWallet(ctx, rpcClient, wallet); err != nil {
		return err
	}
	if nonceAccount != nil && !nonceAccount.IsZero() {
		if err := ValidateNonceAccount(ctx, rpcClient, *nonceAccount, wallet); err != nil {
			return err
		}
	}
	return nil
}

// isNotFoundErr 判断RPC错误是否为“账户不存在”。
func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, rpc.ErrNotFound) {
		return true
	}
	// solana-go在不同版本下可能包装错误，这里做一次宽松的字符串匹配
	return strings.Contains(err.Error(), "not found")
}
