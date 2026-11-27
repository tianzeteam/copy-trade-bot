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

const tradeNonceAdvanceTimeout = 2 * time.Second

var (
	nonceManager *NonceManager
	once         sync.Once
)

type tradeNonceWaiter struct {
	ready    chan struct{}
	priority bool
	granted  bool
}

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

	tradeGate         chan struct{}
	tradeNonceWaiters []*tradeNonceWaiter
	nonceUpdateCh     chan struct{}
	nonceVersion      uint64
	lastTradeNonce    solana.Hash
	hasTradeNonce     bool
}

// NewNonceManager 创建新的NonceManager实例
func NewNonceManager(config *utils.Config) *NonceManager {
	once.Do(func() {
		// 自动创建模式下允许配置为空；非法地址按“未配置”处理，由 EnsureNonceAccount 决策。
		nonceAccountPubkey := parseNonceAccountPubkey(config.NonceAccountPubkey)
		rpcClient := rpc.New(config.RPCUrl)
		nonceClient := client.NewClient(config.RPCUrl)
		nonceManager = &NonceManager{
			rpcClient:          rpcClient,
			nonceClient:        nonceClient,
			nonceAccountPubkey: &nonceAccountPubkey,
			refreshInterval:    350 * time.Millisecond,
			stopChan:           make(chan struct{}),
		}
	})
	return nonceManager
}

func parseNonceAccountPubkey(raw string) solana.PublicKey {
	if raw == "" {
		return solana.PublicKey{}
	}
	pk, err := solana.PublicKeyFromBase58(raw)
	if err != nil {
		logger.Warnf("nonce_account_pubkey 非法，将按未配置处理: %v", err)
		return solana.PublicKey{}
	}
	return pk
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

	//获取当前nonce hash,并设置到全局变量中
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
	changed := nm.setGlobalBlockhashLocked(globalBlockhash)
	nm.mutex.Unlock()

	if changed {
		logger.Debugf("全局区块哈希已刷新: %s", globalBlockhash.String())
	}
	return nil
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
	/* 	if len(data) >= 4 {
		// 状态应该是1（已初始化）
		state := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
		if state != 1 {
			logger.Warnf("nonce账户状态无效，状态: %d，期望: 1，尝试重新初始化", state)
			return fmt.Errorf("nonce账户状态无效，状态: %d，期望: 1", state)
		}
	} */

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

// AcquireTradeNonce 串行预留一笔交易使用的 nonce。
// 调用方必须在广播结束（无论成功或失败）后调用返回的 release 函数。
func (nm *NonceManager) AcquireTradeNonce(ctx context.Context) (solana.Hash, func(), error) {
	return nm.acquireTradeNonce(ctx, false)
}

// AcquirePriorityTradeNonce 以高优先级串行预留一笔交易使用的 nonce。
// 高优先级等待者先于普通等待者获得门禁；调用方必须调用返回的 release 函数。
func (nm *NonceManager) AcquirePriorityTradeNonce(ctx context.Context) (solana.Hash, func(), error) {
	return nm.acquireTradeNonce(ctx, true)
}

func (nm *NonceManager) acquireTradeNonce(ctx context.Context, priority bool) (solana.Hash, func(), error) {
	release, err := nm.acquireTradeNonceGate(ctx, priority)
	if err != nil {
		return solana.Hash{}, nil, err
	}

	var timer *time.Timer
	var timeout <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		if err := ctx.Err(); err != nil {
			release()
			return solana.Hash{}, nil, err
		}

		nm.mutex.Lock()
		if nonce, ok := nm.reserveTradeNonceLocked(); ok {
			nm.mutex.Unlock()
			return nonce, release, nil
		}
		if nm.globalBlockhash == nil {
			nm.mutex.Unlock()
			if err := nm.RefreshGlobalBlockhash(ctx); err != nil {
				release()
				return solana.Hash{}, nil, err
			}
			continue
		}
		if nm.nonceUpdateCh == nil {
			nm.nonceUpdateCh = make(chan struct{})
		}
		updateCh := nm.nonceUpdateCh
		nm.mutex.Unlock()

		if timer == nil {
			timer = time.NewTimer(tradeNonceAdvanceTimeout)
			timeout = timer.C
		}
		select {
		case <-ctx.Done():
			release()
			return solana.Hash{}, nil, ctx.Err()
		case <-updateCh:
		case <-timeout:
			if err := nm.RefreshGlobalBlockhash(ctx); err != nil {
				release()
				return solana.Hash{}, nil, err
			}

			nm.mutex.Lock()
			nonce, ok := nm.reserveTradeNonceLocked()
			nm.mutex.Unlock()
			if !ok {
				release()
				return solana.Hash{}, nil, fmt.Errorf("nonce 刷新后仍未变化，无法安全预留")
			}
			return nonce, release, nil
		}
	}
}

func (nm *NonceManager) acquireTradeNonceGate(ctx context.Context, priority bool) (func(), error) {
	waiter := &tradeNonceWaiter{
		ready:    make(chan struct{}),
		priority: priority,
	}

	nm.mutex.Lock()
	if nm.tradeGate == nil {
		nm.tradeGate = make(chan struct{}, 1)
		nm.tradeGate <- struct{}{}
	}
	nm.tradeNonceWaiters = append(nm.tradeNonceWaiters, waiter)
	nm.grantNextTradeNonceWaiterLocked()
	nm.mutex.Unlock()

	select {
	case <-ctx.Done():
		nm.cancelTradeNonceWaiter(waiter)
		return nil, ctx.Err()
	case <-waiter.ready:
	}

	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			nm.releaseTradeNonceGate()
		})
	}
	return release, nil
}

func (nm *NonceManager) cancelTradeNonceWaiter(waiter *tradeNonceWaiter) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	if waiter.granted {
		nm.releaseTradeNonceGateLocked()
		return
	}
	for i, candidate := range nm.tradeNonceWaiters {
		if candidate == waiter {
			nm.removeTradeNonceWaiterLocked(i)
			return
		}
	}
}

func (nm *NonceManager) releaseTradeNonceGate() {
	nm.mutex.Lock()
	nm.releaseTradeNonceGateLocked()
	nm.mutex.Unlock()
}

func (nm *NonceManager) releaseTradeNonceGateLocked() {
	nm.tradeGate <- struct{}{}
	nm.grantNextTradeNonceWaiterLocked()
}

func (nm *NonceManager) grantNextTradeNonceWaiterLocked() {
	if len(nm.tradeNonceWaiters) == 0 {
		return
	}
	select {
	case <-nm.tradeGate:
	default:
		return
	}

	index := 0
	for i, waiter := range nm.tradeNonceWaiters {
		if waiter.priority {
			index = i
			break
		}
	}
	waiter := nm.removeTradeNonceWaiterLocked(index)
	waiter.granted = true
	close(waiter.ready)
}

func (nm *NonceManager) removeTradeNonceWaiterLocked(index int) *tradeNonceWaiter {
	waiter := nm.tradeNonceWaiters[index]
	copy(nm.tradeNonceWaiters[index:], nm.tradeNonceWaiters[index+1:])
	last := len(nm.tradeNonceWaiters) - 1
	nm.tradeNonceWaiters[last] = nil
	nm.tradeNonceWaiters = nm.tradeNonceWaiters[:last]
	if last == 0 {
		nm.tradeNonceWaiters = nil
	}
	return waiter
}

func (nm *NonceManager) reserveTradeNonceLocked() (solana.Hash, bool) {
	if nm.globalBlockhash == nil {
		return solana.Hash{}, false
	}
	if nm.hasTradeNonce && *nm.globalBlockhash == nm.lastTradeNonce {
		return solana.Hash{}, false
	}

	nonce := *nm.globalBlockhash
	nm.lastTradeNonce = nonce
	nm.hasTradeNonce = true
	return nonce, true
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
	changed := nm.setGlobalBlockhashLocked(&newNonce)
	nm.mutex.Unlock()

	if changed {
		logger.Debugf("通过流更新全局区块哈希: %s", newNonce.String())
	}
}

func (nm *NonceManager) setGlobalBlockhashLocked(blockhash *solana.Hash) bool {
	if blockhash == nil || (nm.globalBlockhash != nil && *nm.globalBlockhash == *blockhash) {
		return false
	}

	nonce := *blockhash
	nm.globalBlockhash = &nonce
	nm.nonceVersion++
	if nm.nonceUpdateCh != nil {
		close(nm.nonceUpdateCh)
		nm.nonceUpdateCh = nil
	}
	return true
}
