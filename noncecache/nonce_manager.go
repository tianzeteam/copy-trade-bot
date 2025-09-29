package noncecache

import (
	"context"
	"fmt"

	"os"
	"strings"
	"sync"
	"time"

	"copytrade/logger"
	"copytrade/utils"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	nonceManager *NonceManager
	once         sync.Once
)

// NonceManager 管理nonce账户的结构体
type NonceManager struct {
	authorityPubkey     *solana.PublicKey
	nonceAccountPubkey  *solana.PublicKey
	globalBlockhash     *solana.Hash
	nonceAccountAddress string
	refreshInterval     time.Duration
	stopChan            chan struct{}
	mutex               sync.RWMutex
	rpcClient           *rpc.Client
	nonceClient         *client.Client
}

// NewNonceManager 创建新的NonceManager实例
func NewNonceManager(config *utils.Config) *NonceManager {

	once.Do(func() {
		rpcClient := rpc.New(config.RPCUrl)
		nonceClient := client.NewClient(config.RPCUrl)
		nonceManager = &NonceManager{
			rpcClient:       rpcClient,
			nonceClient:     nonceClient,
			refreshInterval: 350 * time.Millisecond, // 默认150毫秒刷新一次
			stopChan:        make(chan struct{}),
		}
	})
	return nonceManager
}

func GetNonceManager() *NonceManager {
	if nonceManager == nil {
		panic("NonceManager not initialized")
	}
	return nonceManager
}

// Initialize 初始化nonce账户
func (nm *NonceManager) Initialize(ctx context.Context, tradeWalletKey string) error {
	// 从本地文件加载或创建nonce账户
	authorityData, readErr := os.ReadFile("authority_pubkey.txt")
	nonceData, readErr2 := os.ReadFile("nonce_pubkey.txt")

	ioAuthorityPubKey := ""
	ioNoncePubKey := ""
	_authorityWallet := solana.MustPrivateKeyFromBase58(tradeWalletKey)

	authorityPubkey := _authorityWallet.PublicKey()
	nm.authorityPubkey = &authorityPubkey

	if readErr != nil || readErr2 != nil {
		logger.Warnf("无法读取nonce相关文件，使用配置中的私钥创建新的nonce账户")
	} else {
		ioAuthorityPubKey = strings.TrimSpace(string(authorityData))
		ioNoncePubKey = strings.TrimSpace(string(nonceData))
	}

	var createErr error
	if ioAuthorityPubKey == "" || ioNoncePubKey == "" || ioAuthorityPubKey != nm.authorityPubkey.String() {
		logger.Infof("创建新的nonce账户...")
		nm.nonceAccountPubkey, createErr = nm.CreateNonceAccount(ctx, tradeWalletKey)
		if createErr != nil {
			return fmt.Errorf("创建nonce账户失败: %v", createErr)
		}

		// 将nonce账户地址写入文件
		os.WriteFile("nonce_pubkey.txt", []byte(nm.nonceAccountPubkey.String()), 0644)
		os.WriteFile("authority_pubkey.txt", []byte(nm.authorityPubkey.String()), 0644)

		logger.Infof("新nonce账户创建成功: %s", nm.nonceAccountPubkey.String())
	} else {
		_noncePubkey, parseErr := solana.PublicKeyFromBase58(ioNoncePubKey)

		if parseErr != nil {
			return fmt.Errorf("解析nonce账户地址失败: %v", parseErr)
		}
		nm.nonceAccountPubkey = &_noncePubkey
		logger.Infof("加载已存在的nonce账户: %s", nm.nonceAccountPubkey.String())
	}

	nm.nonceAccountAddress = nm.nonceAccountPubkey.String()
	time.Sleep(10 * time.Second)
	// 验证nonce账户是否已正确初始化
	if err := nm.verifyNonceAccount(ctx); err != nil {
		return fmt.Errorf("nonce账户验证失败: %v", err)
	}

	// 程序启动时，调用一次AdvanceNonceAccount方法
	if err := nm.AdvanceNonce(ctx, _authorityWallet); err != nil {
		logger.Warnf("初始化时推进nonce失败，退出: %v", err)
		return fmt.Errorf("初始化时推进nonce失败，退出: %v", err)
	}

	// 获取当前nonce hash
	if err := nm.RefreshGlobalBlockhash(ctx); err != nil {
		logger.Warnf("初始化时获取nonce当前hash失败，继续: %v", err)
	}

	//TODO disable启动周期性刷新
	//go nm.startPeriodicRefresh(ctx)

	logger.Infof("Nonce管理器初始化成功")
	logger.Infof("Authority account: %s", nm.authorityPubkey.String())
	logger.Infof("Nonce account: %s", nm.nonceAccountAddress)
	logger.Infof("Global blockhash: %s", nm.globalBlockhash.String())
	logger.Infof("周期性刷新已启动，间隔: %v", nm.refreshInterval)

	return nil
}

// AdvanceNonce 推进nonce账户
func (nm *NonceManager) AdvanceNonce(ctx context.Context, _authorityWallet solana.PrivateKey) error {
	recentBlockhash, err := nm.GetRecentBlockhash(ctx)
	if err != nil {
		return fmt.Errorf("获取RecentBlockhash失败: %v", err)
	}

	advanceErr := nm.AdvanceNonceAccount(ctx, recentBlockhash, nm.nonceAccountAddress, _authorityWallet)
	if advanceErr != nil {
		return fmt.Errorf("AdvanceNonceAccount失败: %v", advanceErr)
	}

	logger.Infof("Nonce账户推进成功")
	return nil
}

// RefreshGlobalBlockhash 刷新全局区块哈希
func (nm *NonceManager) RefreshGlobalBlockhash(ctx context.Context) error {
	globalBlockhash, err := nm.GetNonce(ctx, nm.nonceAccountAddress)
	if err != nil {
		return fmt.Errorf("获取Nonce当前hash失败: %v", err)
	}

	nm.mutex.Lock()
	nm.globalBlockhash = globalBlockhash
	nm.mutex.Unlock()

	logger.Debugf("全局区块哈希已刷新: %s", nm.globalBlockhash.String())
	return nil
}

// startPeriodicRefresh 启动周期性刷新
func (nm *NonceManager) startPeriodicRefresh(ctx context.Context) {
	ticker := time.NewTicker(nm.refreshInterval)
	defer ticker.Stop()

	logger.Infof("开始周期性刷新全局区块哈希，间隔: %v", nm.refreshInterval)

	for {
		select {
		case <-ticker.C:
			if err := nm.RefreshGlobalBlockhash(ctx); err != nil {
				logger.Errorf("周期性刷新全局区块哈希失败: %v", err)
			}
		case <-nm.stopChan:
			logger.Infof("停止周期性刷新全局区块哈希")
			return
		case <-ctx.Done():
			logger.Infof("上下文取消，停止周期性刷新")
			return
		}
	}
}

// Stop 停止周期性刷新
func (nm *NonceManager) Stop() {
	close(nm.stopChan)
}

// verifyNonceAccount 验证nonce账户是否已正确初始化
func (nm *NonceManager) verifyNonceAccount(ctx context.Context) error {
	// 首先检查账户是否存在并获取账户信息
	nonceAccountPubkey, err := solana.PublicKeyFromBase58(nm.nonceAccountAddress)
	if err != nil {
		return fmt.Errorf("解析nonce账户地址失败: %v", err)
	}

	accountInfo, err := nm.rpcClient.GetAccountInfo(ctx, nonceAccountPubkey)
	if err != nil {
		return fmt.Errorf("获取nonce账户信息失败: %v", err)
	}

	if accountInfo.Value == nil {
		return fmt.Errorf("nonce账户不存在，需要重新创建")
	}

	// 检查账户所有者是否为系统程序
	systemProgramID := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	if !accountInfo.Value.Owner.Equals(systemProgramID) {
		return fmt.Errorf("nonce账户所有者不正确，期望: %s, 实际: %s", systemProgramID.String(), accountInfo.Value.Owner.String())
	}

	// 检查账户数据长度（nonce账户应该是80字节）
	data := accountInfo.Value.Data.GetBinary()
	if len(data) != 80 {
		logger.Warnf("nonce账户数据长度不正确，期望: 80, 实际: %d，尝试重新初始化", len(data))
		return fmt.Errorf("nonce账户数据长度不正确，期望: 80, 实际: %d", len(data))
	}

	// 检查nonce账户状态（前4字节表示状态）
	if len(data) >= 4 {
		// 状态应该是1（已初始化）
		state := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
		if state != 1 {
			logger.Warnf("nonce账户状态无效，状态: %d，期望: 1，尝试重新初始化", state)
			return fmt.Errorf("nonce账户状态无效，状态: %d，期望: 1", state)
		}
	}

	// 最后通过GetNonce方法验证账户是否可用
	_, err = nm.GetNonce(ctx, nm.nonceAccountAddress)
	if err != nil {
		if strings.Contains(err.Error(), "empty string") || strings.Contains(err.Error(), "not be properly initialized") {
			logger.Warnf("nonce账户GetNonce失败，尝试重新初始化: %v", err)
			return fmt.Errorf("nonce账户GetNonce失败，尝试重新初始化: %v", err)
		}
		return fmt.Errorf("nonce账户验证失败: %v", err)
	}

	logger.Infof("nonce账户验证通过: %s", nm.nonceAccountAddress)
	logger.Infof("nonce账户余额: %d lamports", accountInfo.Value.Lamports)
	logger.Infof("nonce账户数据长度: %d bytes", len(data))
	return nil
}

// SetRefreshInterval 设置刷新间隔
func (nm *NonceManager) SetRefreshInterval(interval time.Duration) {
	nm.mutex.Lock()
	nm.refreshInterval = interval
	nm.mutex.Unlock()
	logger.Infof("刷新间隔已更新为: %v", interval)
}

// GetGlobalBlockhash 获取当前全局区块哈希
func (nm *NonceManager) GetGlobalBlockhash() *solana.Hash {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()
	return nm.globalBlockhash
}

// GetNonceAccountAddress 获取nonce账户地址
func (nm *NonceManager) GetNonceAccountAddress() string {
	return nm.nonceAccountAddress
}

// GetNonceAccountPubkey 获取nonce账户公钥
func (nm *NonceManager) GetNonceAccountPubkey() *solana.PublicKey {
	return nm.nonceAccountPubkey
}

// UpdateNonce 更新全局区块哈希（从流更新中调用）
func (nm *NonceManager) UpdateNonce(newNonce solana.Hash) {
	nm.mutex.Lock()
	nm.globalBlockhash = &newNonce
	nm.mutex.Unlock()

	logger.Debugf("通过流更新全局区块哈希: %s", newNonce.String())
}
