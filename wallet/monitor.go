package wallet

import (
	"context"
	"copytrade/queue"
	"math"
	"sync"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
)

// WalletType 定义钱包类型
type WalletType string

const (
	HighFrequency WalletType = "high_frequency" // 高频交易钱包
	LowFrequency  WalletType = "low_frequency"  // 低频交易钱包
)

// WalletSettings 定义单个钱包的交易设置
type WalletSettings struct {
	Address       solana.PublicKey // 钱包地址
	Name          string           // 钱包名称或标识
	Type          WalletType       // 钱包类型
	TakeProfitPct int              // 止盈百分比
	StopLossPct   int              // 止损百分比
	ExitTimeSec   int              // 持仓时间(秒)
	Enabled       bool             // 是否启用
	IsFollowSell  bool             // 是否跟卖
	ParamSchemeID string           // 参数方案ID
	Risk          string           // 风险等级: high, medium, low
	Note          string           // 备注信息
}

// TradeRecord 记录交易信息
type TradeRecord struct {
	MintAddress   string          // 代币地址
	WalletAddress string          // 交易钱包地址
	EntryPrice    float64         // 入场价格
	RecoveryPct   float64         // 抽回本金百分比
	Amount        uint64          // 交易数量
	Timestamp     time.Time       // 交易时间
	ExitDeadline  time.Time       // 退出截止时间
	Settings      *WalletSettings // 交易设置
	StrategyName  string          // 策略名称
}

// WalletMonitor 钱包监控器
type WalletMonitor struct {
	Wallets             []*WalletSettings             // 监控的钱包列表
	TradeRecords        map[string]*TradeRecord       // 交易记录，key为代币地址
	TradedTokens        map[string]bool               // 已交易过的代币，防止重复交易
	BotTrades           map[string]*TradeRecord       // 机器人的交易记录，key为代币地址
	Mux                 sync.RWMutex                  // 保护并发访问
	RedisService        *RedisWalletService           // Redis服务
	ParamSchemeProvider ParamSchemeProvider           // 参数模板提供者
	Ctx                 context.Context               // 上下文
	ParamSchemes        map[string]*types.ParamScheme // 参数模板缓存，key为钱包地址
	SchemeTemplates     map[string]*types.ParamScheme // 模板缓存，key为模板ID
	DefaultSchemeID     string                        // 默认参数模板ID
	ParamSchemesMux     sync.RWMutex                  // 参数模板缓存的并发访问保护
	BlacklistService    *BlacklistService             // 黑名单服务
	BlacklistAddresses  map[string]bool               // 黑名单地址缓存
	BlacklistMux        sync.RWMutex                  // 黑名单缓存的并发访问保护
	BlacklistedTokens   map[string]bool               // 被黑名单用户购买的代币缓存
	BlacklistedTokenMux sync.RWMutex                  // 代币黑名单的并发访问保护

}

// NewWalletMonitor 创建新的钱包监控器
func NewWalletMonitor(redisService *RedisWalletService, paramSchemeProvider ParamSchemeProvider) *WalletMonitor {
	// 创建黑名单服务
	var blacklistService *BlacklistService
	if redisService != nil {
		blacklistService = NewBlacklistService(redisService.Client, redisService.KeyPrefix)
	}

	return &WalletMonitor{
		Wallets:      make([]*WalletSettings, 0),
		TradeRecords: make(map[string]*TradeRecord),
		TradedTokens: make(map[string]bool),
		BotTrades:    make(map[string]*TradeRecord),

		Mux:                 sync.RWMutex{},
		RedisService:        redisService,
		ParamSchemeProvider: paramSchemeProvider,
		Ctx:                 context.Background(),
		ParamSchemes:        make(map[string]*types.ParamScheme), // 钱包地址 -> 参数模板
		SchemeTemplates:     make(map[string]*types.ParamScheme), // 模板ID -> 参数模板
		DefaultSchemeID:     "",
		ParamSchemesMux:     sync.RWMutex{},
		BlacklistService:    blacklistService,
		BlacklistAddresses:  make(map[string]bool),
		BlacklistMux:        sync.RWMutex{},
		BlacklistedTokens:   make(map[string]bool),
		BlacklistedTokenMux: sync.RWMutex{},
	}
}

// AddWallet 添加监控钱包
func (m *WalletMonitor) AddWallet(wallet *WalletSettings) {
	m.Mux.Lock()
	defer m.Mux.Unlock()
	m.Wallets = append(m.Wallets, wallet)
	logger.Infof("添加监控钱包: %s (%s)", wallet.Address.String(), wallet.Name)
}

// RemoveWallet 移除监控钱包
func (m *WalletMonitor) RemoveWallet(address solana.PublicKey) bool {
	m.Mux.Lock()
	defer m.Mux.Unlock()

	for i, wallet := range m.Wallets {
		if wallet.Address.Equals(address) {
			// 移除钱包
			m.Wallets = append(m.Wallets[:i], m.Wallets[i+1:]...)
			logger.Infof("移除监控钱包: %s (%s)", wallet.Address.String(), wallet.Name)
			return true
		}
	}
	return false
}

// GetWallet 获取钱包设置
func (m *WalletMonitor) GetWallet(address solana.PublicKey) *WalletSettings {
	m.Mux.RLock()
	defer m.Mux.RUnlock()

	for _, wallet := range m.Wallets {
		if wallet.Address.Equals(address) {
			return wallet
		}
	}
	return nil
}

// RecordTrade 记录原始交易者的交易
func (m *WalletMonitor) RecordTrade(mintAddress string, copyAddress solana.PublicKey, entryPrice float64, originAmount uint64) {

	// 获取聪明钱包设置
	var walletSettings *WalletSettings
	for _, wallet := range m.Wallets {
		if wallet.Address.Equals(copyAddress) {
			walletSettings = wallet
			break
		}
	}

	if walletSettings == nil {
		logger.Warnf("未找到聪明钱包设置: %s", copyAddress.String())
		return
	}

	// 计算退出截止时间
	exitDeadline := time.Now().Add(time.Duration(walletSettings.ExitTimeSec) * time.Second)

	// 记录交易
	tradeRecord := &TradeRecord{
		MintAddress:   mintAddress,
		WalletAddress: copyAddress.String(),
		EntryPrice:    entryPrice,
		Amount:        originAmount,
		Timestamp:     time.Now(),
		ExitDeadline:  exitDeadline,
		Settings:      walletSettings,
	}

	m.TradeRecords[mintAddress] = tradeRecord
	m.TradedTokens[mintAddress] = true

	logger.Infof("记录原始交易: 代币=%s, 聪明钱包地址=%s, 入场价格=%.8f, 数量=%d, 止盈=%d%%, 止损=%d%%, 持仓时间=%d秒",
		mintAddress, copyAddress.String(), entryPrice, originAmount,
		walletSettings.TakeProfitPct, walletSettings.StopLossPct, walletSettings.ExitTimeSec)
}

// RecordBotTrade 记录机器人的跟单交易
func (m *WalletMonitor) RecordBotTrade(mintAddress string, copyAddress, botAddress solana.PublicKey, entryPrice float64) {
	m.Mux.Lock()
	defer m.Mux.Unlock()

	// 获取钱包设置
	var walletSettings *WalletSettings
	for _, wallet := range m.Wallets {
		if wallet.Address.Equals(copyAddress) {
			walletSettings = wallet
			break
		}
	}

	if walletSettings == nil {
		// 如果机器人钱包没有特定设置，退出执行
		return
	}

	// 计算退出截止时间
	exitDeadline := time.Now().Add(time.Duration(walletSettings.ExitTimeSec) * time.Second)

	// 记录交易
	tradeRecord := &TradeRecord{
		MintAddress:   mintAddress,
		WalletAddress: botAddress.String(),
		EntryPrice:    entryPrice,
		Amount:        0,
		Timestamp:     time.Now(),
		ExitDeadline:  exitDeadline,
		Settings:      walletSettings,
	}

	m.BotTrades[mintAddress] = tradeRecord

	logger.Infof("记录机器人跟单交易: 代币=%s, 机器人钱包=%s, 入场价格=%.8f, 止盈=%d%%, 止损=%d%%, 持仓时间=%d秒",
		mintAddress, botAddress.String(), entryPrice,
		walletSettings.TakeProfitPct, walletSettings.StopLossPct, walletSettings.ExitTimeSec)
}

// TODO CheckExitConditions 检查是否满足止盈止损退出条件
func (m *WalletMonitor) CheckExitConditions(platform, mintAddress string, currentPrice float64, pumpVersion string) (bool, string, *TradeRecord) {
	m.Mux.RLock()
	defer m.Mux.RUnlock()

	// 获取交易记录
	tradeRecord, exists := m.TradeRecords[mintAddress]
	if !exists {
		// 如果在原始交易记录中没有找到，尝试从机器人交易中查找
		tradeRecord, exists = m.BotTrades[mintAddress]
		if !exists {
			logger.Debugf("未买入该资产信息 ：%s", mintAddress)
			return false, "", nil
		}
	}
	assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
	if !ok {
		logger.Debugf("读取买入资产信息发生异常 ：%s", mintAddress)
		return false, "读取买入资产信息发生异常", nil
	}
	// 计算价格变化百分比
	var priceChangePct float64
	if assetInfo.EntryPrice == 0 {

		logger.Debugf("代币 %s 的入场价格为零，无法计算价格变化百分比", mintAddress)
		return false, "入场价信息异常", nil
	} else {
		priceChangePct = (currentPrice - assetInfo.EntryPrice) / assetInfo.EntryPrice * 100
	}
	// 检查是否抽回本金
	m.checkCapitalRecovery(assetInfo, mintAddress, priceChangePct, currentPrice, pumpVersion, platform, tradeRecord)
	// 检查止盈条件
	if priceChangePct >= float64(tradeRecord.Settings.TakeProfitPct) {
		logger.Infof("止盈条件被触发 :%s ,价格波动：%.8f%% ,止盈设置：%d%%", mintAddress, priceChangePct, tradeRecord.Settings.TakeProfitPct)
		go queue.SendTokenToSellChannel(0, assetInfo, assetInfo.Amount, currentPrice, pumpVersion, platform, tradeRecord.Settings.Name, "stop_profit")
		return true, "止盈触发", tradeRecord
	}

	// 检查止损条件
	if priceChangePct <= -float64(tradeRecord.Settings.StopLossPct) {
		logger.Infof("止损条件被触发 :%s ,价格波动：%.8f%% ,止损设置：%d%%", mintAddress, priceChangePct, tradeRecord.Settings.StopLossPct)
		go queue.SendTokenToSellChannel(0, assetInfo, assetInfo.Amount, currentPrice, pumpVersion, platform, tradeRecord.Settings.Name, "stop_loss")
		return true, "止损触发", tradeRecord
	}

	return false, "", nil
}

// checkCapitalRecovery 检查并执行抽回本金逻辑
func (m *WalletMonitor) checkCapitalRecovery(assetInfo *global.MyBuyMap, mintAddress string, priceChangePct float64, currentPrice float64, pumpVersion string, platform string, tradeRecord *TradeRecord) {
	capitalRecoveryPercent := global.GetConfig().CapitalRecoveryPercent
	// 检查是否抽回本金，如果没有抽回本金，则判断是否达到抽回本金条件
	if !assetInfo.IsRecovery && capitalRecoveryPercent > 0 {
		if priceChangePct >= capitalRecoveryPercent {
			assetInfo.IsRecovery = true
			logger.Infof("抽回本金条件被触发: %s, 当前涨幅: %.2f%%, 抽回本金盈利百分比: %.1f%%",
				mintAddress, priceChangePct, global.GetConfig().CapitalRecoveryPercent)

			// 使用存储的Decimal字段进行精确计算
			tokenDecimals := math.Pow(10, float64(assetInfo.Decimal))

			// 计算初始投资的SOL数量（标准单位）
			recoverySOLAmount := assetInfo.EntryPrice * (float64(assetInfo.InitBuyAmount) / tokenDecimals)

			// 计算需要卖出的代币数量（最小单位）
			recoveryTokenAmount := uint64(recoverySOLAmount / currentPrice * tokenDecimals)

			// 确保不会卖出超过当前持有量
			if recoveryTokenAmount > assetInfo.Amount {
				logger.Warnf("抽回本金数量超过持有量，调整为不抽回本金: %s, 持有量: %d", mintAddress, assetInfo.Amount)
				return
			}

			// 确保至少卖出一些代币
			if recoveryTokenAmount > 0 {
				logger.Infof("执行抽回本金: %s, 卖出代币数量: %d, 预期回收SOL: %.8f, 剩余持有量: %d",
					mintAddress, recoveryTokenAmount, recoverySOLAmount, assetInfo.Amount-recoveryTokenAmount)
				go queue.SendTokenToSellChannel(0, assetInfo, recoveryTokenAmount, currentPrice, pumpVersion, platform, tradeRecord.Settings.Name, "recovery")
			} else {
				logger.Warnf("抽回本金计算异常，跳过执行: %s", mintAddress)
			}
		}
	}
}

// RemoveTrade 移除交易记录
func (m *WalletMonitor) RemoveTrade(mintAddress string) {
	m.Mux.Lock()
	defer m.Mux.Unlock()

	delete(m.TradeRecords, mintAddress)
	delete(m.BotTrades, mintAddress)
	// 注意：不删除 TradedTokens 中的记录，以防重复交易
}

// StartMonitoring 启动监控
func (m *WalletMonitor) StartMonitoring(ctx context.Context) {
	// 保存上下文
	m.Ctx = ctx

	// 从 Redis 加载钱包列表
	m.refreshWalletList()

	// 初始化时刷新参数模板列表
	m.refreshParamSchemes()

	logger.Infof("启动钱包监控服务，监控 %d 个钱包", len(m.Wallets))

	// 启动定期检查价格和退出条件的协程
	go func() {
		// 价格检查器 - 每秒检查一次
		priceTicker := time.NewTicker(1 * time.Second)
		defer priceTicker.Stop()

		// 钱包列表刷新器 - 每30秒从 Redis 刷新一次钱包列表
		walletRefreshTicker := time.NewTicker(30 * time.Second)
		defer walletRefreshTicker.Stop()

		// 参数模板刷新器 - 每30秒从 Redis 刷新一次参数模板列表
		paramSchemeRefreshTicker := time.NewTicker(30 * time.Second)
		defer paramSchemeRefreshTicker.Stop()

		// 黑名单刷新器 - 每30秒从 Redis 刷新一次黑名单列表
		blacklistRefreshTicker := time.NewTicker(30 * time.Second)
		defer blacklistRefreshTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Infof("钱包监控服务停止")
				return
			case <-walletRefreshTicker.C:
				// 刷新钱包列表
				m.refreshWalletList()
			case <-paramSchemeRefreshTicker.C:
				// 刷新参数模板列表
				m.refreshParamSchemes()
			case <-blacklistRefreshTicker.C:
				// 刷新黑名单列表
				m.refreshBlacklist()
			}
		}
	}()
}

// refreshWalletList 从 Redis 刷新钱包列表
func (m *WalletMonitor) refreshWalletList() {
	// 从 Redis 获取所有钱包
	wallets, err := m.RedisService.GetAllWallets(m.Ctx)
	if err != nil {
		logger.Errorf("从 Redis 获取钱包列表失败: %v", err)
		return
	}

	// 更新钱包列表
	m.Mux.Lock()
	m.Wallets = wallets
	m.Mux.Unlock()

	logger.Infof("从 Redis 刷新钱包列表，共 %d 个钱包", len(wallets))
}

// refreshParamSchemes 刷新参数模板列表
func (m *WalletMonitor) refreshParamSchemes() {
	// 如果没有参数模板提供者，直接返回
	if m.ParamSchemeProvider == nil {
		logger.Warnf("参数模板提供者未设置，无法刷新参数模板")
		return
	}

	// 使用接口获取所有参数模板 ID
	schemeIDs, err := m.ParamSchemeProvider.GetParamSchemeIDs(m.Ctx)
	if err != nil {
		logger.Errorf("获取参数模板列表失败: %v", err)
		return
	}

	// 使用接口获取默认参数模板 ID
	defaultSchemeID, err := m.ParamSchemeProvider.GetDefaultParamSchemeID(m.Ctx)
	if err != nil {
		logger.Warnf("获取默认参数模板ID失败: %v", err)
		// 如果获取默认模板ID失败，使用空字符串
		defaultSchemeID = ""
	}

	// 创建新的参数模板缓存 (模板ID -> 模板)
	newSchemeTemplates := make(map[string]*types.ParamScheme)

	// 遍历所有参数模板ID，获取并缓存模板详细信息
	for _, schemeID := range schemeIDs {
		scheme, err := m.ParamSchemeProvider.GetParamScheme(m.Ctx, schemeID)
		if err != nil {
			logger.Warnf("获取参数模板详细信息失败: %s, %v", schemeID, err)
			continue
		}

		// 将模板添加到模板ID缓存中
		newSchemeTemplates[schemeID] = scheme
	}

	// 创建新的钱包地址到参数模板的映射
	newParamSchemes := make(map[string]*types.ParamScheme)

	// 遍历钱包列表，记录参数模板 ID 并建立钱包地址到模板的映射
	m.Mux.RLock()
	for _, wallet := range m.Wallets {
		walletAddr := wallet.Address.String()
		schemeID := wallet.ParamSchemeID

		// 如果钱包有绑定的参数模板ID
		if schemeID != "" {
			// 如果模板存在，将其添加到钱包地址映射中
			if scheme, exists := newSchemeTemplates[schemeID]; exists {
				newParamSchemes[walletAddr] = scheme
			} else {
				logger.Warnf("钱包 %s 绑定的参数模板 %s 不存在", walletAddr, schemeID)
			}
		} else if defaultSchemeID != "" {
			// 如果钱包没有绑定模板但有默认模板，使用默认模板
			if scheme, exists := newSchemeTemplates[defaultSchemeID]; exists {
				newParamSchemes[walletAddr] = scheme
			}
		}
	}
	m.Mux.RUnlock()

	// 更新参数模板缓存
	m.ParamSchemesMux.Lock()
	m.SchemeTemplates = newSchemeTemplates // 更新（模板ID -> 模板）的映射
	m.ParamSchemes = newParamSchemes       // 更新（钱包地址 -> 模板）的映射
	m.DefaultSchemeID = defaultSchemeID
	m.ParamSchemesMux.Unlock()

	global.UpdateConfigField("wallet_param_schemes", newParamSchemes)

	logger.Infof("刷新参数模板列表，共 %d 个参数模板，默认参数模板 ID: %s", len(schemeIDs), defaultSchemeID)
	logger.Infof("成功缓存 %d 个参数模板到内存", len(newSchemeTemplates))
	logger.Infof("成功建立 %d 个钱包地址到参数模板的映射", len(newParamSchemes))

}

// refreshBlacklist 刷新黑名单列表
func (m *WalletMonitor) refreshBlacklist() {
	// 如果没有黑名单服务，直接返回
	if m.BlacklistService == nil {
		return
	}

	// 获取所有启用的黑名单地址
	addresses, err := m.BlacklistService.GetEnabledAddresses(m.Ctx)
	if err != nil {
		logger.Errorf("刷新黑名单列表失败: %v", err)
		return
	}

	// 创建新的黑名单地址映射
	newBlacklist := make(map[string]bool)
	for _, addr := range addresses {
		newBlacklist[addr] = true
	}

	// 更新黑名单缓存
	m.BlacklistMux.Lock()
	m.BlacklistAddresses = newBlacklist
	m.BlacklistMux.Unlock()

	logger.Infof("刷新黑名单列表，共 %d 个启用的黑名单地址", len(addresses))
}

// IsBlacklisted 检查地址是否在黑名单中
func (m *WalletMonitor) IsBlacklisted(address string) bool {
	m.BlacklistMux.RLock()
	defer m.BlacklistMux.RUnlock()
	return m.BlacklistAddresses[address]
}

// GetBlacklistCount 获取黑名单地址数量
func (m *WalletMonitor) GetBlacklistCount() int {
	m.BlacklistMux.RLock()
	defer m.BlacklistMux.RUnlock()
	return len(m.BlacklistAddresses)
}

// AddBlacklistedToken 添加代币到黑名单（当黑名单用户购买时）
func (m *WalletMonitor) AddBlacklistedToken(tokenAddress, userAddress string) {
	m.BlacklistedTokenMux.Lock()
	defer m.BlacklistedTokenMux.Unlock()

	if !m.BlacklistedTokens[tokenAddress] {
		m.BlacklistedTokens[tokenAddress] = true
		logger.Warnf("代币已被标记为黑名单: token=%s, 原因=黑名单用户购买 user=%s", tokenAddress, userAddress)
	}
}

// IsTokenBlacklisted 检查代币是否在黑名单中
func (m *WalletMonitor) IsTokenBlacklisted(tokenAddress string) bool {
	m.BlacklistedTokenMux.RLock()
	defer m.BlacklistedTokenMux.RUnlock()
	return m.BlacklistedTokens[tokenAddress]
}

// RemoveBlacklistedToken 从代币黑名单中移除（可选功能）
func (m *WalletMonitor) RemoveBlacklistedToken(tokenAddress string) {
	m.BlacklistedTokenMux.Lock()
	defer m.BlacklistedTokenMux.Unlock()
	delete(m.BlacklistedTokens, tokenAddress)
	logger.Infof("代币已从黑名单中移除: token=%s", tokenAddress)
}

// GetBlacklistedTokenCount 获取黑名单代币数量
func (m *WalletMonitor) GetBlacklistedTokenCount() int {
	m.BlacklistedTokenMux.RLock()
	defer m.BlacklistedTokenMux.RUnlock()
	return len(m.BlacklistedTokens)
}
