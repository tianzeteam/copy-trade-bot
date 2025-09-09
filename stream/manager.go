package stream

import (
	"context"
	"fmt"
	"sync"
	"time"

	"copytrade/bot"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/raydium/layouts"
	"copytrade/wallet"

	goyser "copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"slices"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

// StreamManager 管理gRPC流连接和订阅
type StreamManager struct {
	grpcUrl             string
	client              *goyser.Client
	mintStreamClient    *goyser.StreamClient
	accountStreamClient *goyser.StreamClient
	marketStreamClient  *goyser.StreamClient // OpenBook市场流客户端
	poolStreamClient    *goyser.StreamClient // Raydium池流客户端
	tradeBot            *bot.Bot
	transactionMonitor  *wallet.TransactionMonitor
	walletMonitor       *wallet.WalletMonitor

	balanceManager interface{}

	// 控制流
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.RWMutex
	isRunning bool

	// 重连配置
	maxRetries    int
	retryInterval time.Duration

	// 订阅配置
	subscriptionConfig *SubscriptionConfig

	// 市场订阅配置
	quoteTokenMint solana.PublicKey // 用于市场和池订阅的报价代币
}

// SubscriptionConfig 订阅配置
type SubscriptionConfig struct {
	AccountInclude  []string
	Vote            bool
	Failed          bool
	CommitmentLevel yellowstone_geyser_pb.CommitmentLevel
}

// NewStreamManager 创建新的流管理器
func NewStreamManager(grpcUrl string, tradeBot *bot.Bot, transactionMonitor *wallet.TransactionMonitor, walletMonitor *wallet.WalletMonitor, balanceManager any) *StreamManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &StreamManager{
		grpcUrl:            grpcUrl,
		tradeBot:           tradeBot,
		transactionMonitor: transactionMonitor,
		walletMonitor:      walletMonitor,
		balanceManager:     balanceManager,
		ctx:                ctx,
		cancel:             cancel,
		maxRetries:         5,
		retryInterval:      time.Second * 10,
		quoteTokenMint:     solana.SolMint, // 默认使用SOL作为报价代币
		subscriptionConfig: &SubscriptionConfig{
			AccountInclude: []string{
				core.PUMP_PROGRAM_ID.String(),
				core.AMM_PROGRAM_ID.String(),
				core.RAYDIUM_V4_PROGRAM_ID.String(),
				core.RAYDIUM_LAUNCHPAD_PROGRAM_ID.String(),
				core.RAYDIUM_CPMM_PROGRAM_ID.String(),
				core.PHOTON_ROUTER_PROGRAM_ID.String(),
				core.OKX_ROUTER_V2_PROGRAM_ID.String(),
			},
			Vote:            false,
			Failed:          false,
			CommitmentLevel: yellowstone_geyser_pb.CommitmentLevel_PROCESSED,
		},
	}
}

// Start 启动流管理器
func (sm *StreamManager) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.isRunning {
		return fmt.Errorf("stream manager is already running")
	}

	if err := sm.initializeClient(); err != nil {
		return fmt.Errorf("failed to initialize client: %v", err)
	}

	if err := sm.setupStreams(); err != nil {
		return fmt.Errorf("failed to setup streams: %v", err)
	}

	sm.isRunning = true

	// 启动流处理goroutines
	go sm.handleTransactionStream()
	go sm.handleAccountStream()
	go sm.monitorConnection() // 启动连接监控

	// 启动市场和池订阅
	if err := sm.setupMarketSubscriptions(); err != nil {
		logger.Warnf("Failed to setup market subscriptions: %v", err)
		// 不返回错误，因为这不是致命错误
	}

	logger.Infof("Stream manager started successfully")
	return nil
}

// Stop 停止流管理器
func (sm *StreamManager) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.isRunning {
		return
	}

	sm.cancel()
	sm.isRunning = false

	if sm.client != nil {
		// 关闭客户端连接
		// 注意：根据goyser库的实际API调整关闭方法
		sm.client = nil
	}

	logger.Infof("Stream manager stopped")
}

// Restart 重启流管理器
func (sm *StreamManager) Restart() error {
	logger.Infof("Restarting stream manager...")

	sm.Stop()

	// 等待一段时间确保资源清理完成
	time.Sleep(time.Second * 2)

	// 重新创建context
	sm.ctx, sm.cancel = context.WithCancel(context.Background())

	return sm.Start()
}

// UpdateSubscriptionConfig 更新订阅配置
func (sm *StreamManager) UpdateSubscriptionConfig(config *SubscriptionConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.subscriptionConfig = config

	// 如果正在运行，需要重启以应用新配置
	if sm.isRunning {
		go func() {
			if err := sm.Restart(); err != nil {
				logger.Errorf("Failed to restart stream manager with new config: %v", err)
			}
		}()
	}

	return nil
}

// initializeClient 初始化gRPC客户端
func (sm *StreamManager) initializeClient() error {
	client, err := goyser.New(sm.ctx, sm.grpcUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to create geyser client: %v", err)
	}

	sm.client = client
	return nil
}

// setupStreams 设置流客户端
func (sm *StreamManager) setupStreams() error {
	// 设置交易流
	if err := sm.client.AddStreamClient(sm.ctx, "mint_stream", sm.subscriptionConfig.CommitmentLevel); err != nil {
		return fmt.Errorf("failed to add mint stream client: %v", err)
	}

	sm.mintStreamClient = sm.client.GetStreamClient("mint_stream")
	if sm.mintStreamClient == nil {
		return fmt.Errorf("failed to get mint stream client")
	}

	// 订阅交易
	if err := sm.mintStreamClient.SubscribeTransaction("transaction_stream", &yellowstone_geyser_pb.SubscribeRequestFilterTransactions{
		Vote:           &sm.subscriptionConfig.Vote,
		Failed:         &sm.subscriptionConfig.Failed,
		AccountInclude: sm.subscriptionConfig.AccountInclude,
	}); err != nil {
		return fmt.Errorf("failed to subscribe to transactions: %v", err)
	}

	// 设置账户流
	if err := sm.client.AddStreamClient(sm.ctx, "accounts_stream", sm.subscriptionConfig.CommitmentLevel); err != nil {
		return fmt.Errorf("failed to add accounts stream client: %v", err)
	}

	sm.accountStreamClient = sm.client.GetStreamClient("accounts_stream")
	if sm.accountStreamClient == nil {
		return fmt.Errorf("failed to get accounts stream client")
	}

	// 订阅账户
	if err := sm.setupAccountSubscription(); err != nil {
		return fmt.Errorf("failed to setup account subscription: %v", err)
	}

	// 启动ping服务
	sm.startPingService(sm.mintStreamClient)
	sm.startPingService(sm.accountStreamClient)

	return nil
}

// setupMarketSubscriptions 设置市场和池订阅
func (sm *StreamManager) setupMarketSubscriptions() error {
	// 设置OpenBook市场订阅
	if err := sm.setupOpenBookMarketSubscription(); err != nil {
		logger.Errorf("Failed to setup OpenBook market subscription: %v", err)
		// 继续尝试设置Raydium池订阅
	}

	// 设置Raydium池订阅
	if err := sm.setupRaydiumPoolSubscription(); err != nil {
		logger.Errorf("Failed to setup Raydium pool subscription: %v", err)
	}

	return nil
}

// setupOpenBookMarketSubscription 设置OpenBook市场订阅
func (sm *StreamManager) setupOpenBookMarketSubscription() error {
	// 创建OpenBook市场流客户端
	streamName := fmt.Sprintf("openbook_markets_%s", sm.quoteTokenMint.String())
	if err := sm.client.AddStreamClient(sm.ctx, streamName, sm.subscriptionConfig.CommitmentLevel); err != nil {
		return fmt.Errorf("failed to add stream client for OpenBook markets: %w", err)
	}

	// 获取流客户端
	sm.marketStreamClient = sm.client.GetStreamClient(streamName)
	if sm.marketStreamClient == nil {
		return fmt.Errorf("failed to get stream client for OpenBook markets")
	}

	datasizeFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Datasize{
			Datasize: 388,
		},
	}

	memcmpFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 85,
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: sm.quoteTokenMint.String(),
				},
			},
		},
	}

	// 订阅OpenBook市场账户
	if err := sm.marketStreamClient.SubscribeAccounts("openbook_market_accounts", &yellowstone_geyser_pb.SubscribeRequestFilterAccounts{
		Owner: []string{core.OPENBOOK_MARKET_PROGRAM_ID.String()},
		Filters: []*yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
			datasizeFilter,
			memcmpFilter,
		},
	}); err != nil {
		return fmt.Errorf("failed to subscribe to OpenBook market accounts: %w", err)
	}

	// 启动ping服务和处理goroutine
	go sm.startPingService(sm.marketStreamClient)
	go sm.handleOpenBookMarketStream()

	logger.Infof("Successfully subscribed to OpenBook markets for quote token: %s", sm.quoteTokenMint.String())
	return nil
}

// setupRaydiumPoolSubscription 设置Raydium池订阅
func (sm *StreamManager) setupRaydiumPoolSubscription() error {
	// 创建Raydium池流客户端
	streamName := fmt.Sprintf("raydium_pools_%s", sm.quoteTokenMint.String())
	if err := sm.client.AddStreamClient(sm.ctx, streamName, sm.subscriptionConfig.CommitmentLevel); err != nil {
		return fmt.Errorf("failed to add stream client for Raydium pools: %w", err)
	}

	// 获取流客户端
	sm.poolStreamClient = sm.client.GetStreamClient(streamName)
	if sm.poolStreamClient == nil {
		return fmt.Errorf("failed to get stream client for Raydium pools")
	}

	// 数据大小过滤器
	datasizeFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Datasize{
			Datasize: 752, // Raydium V4流动性状态布局大小
		},
	}

	// 报价代币过滤器
	quoteMintFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 432, // 布局中quoteMint的偏移量
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: sm.quoteTokenMint.String(),
				},
			},
		},
	}

	// 市场程序ID过滤器
	marketProgramFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 560, // 布局中marketProgramId的偏移量
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: core.OPENBOOK_MARKET_PROGRAM_ID.String(),
				},
			},
		},
	}

	// 状态过滤器
	statusFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 0, // 布局中status的偏移量
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Bytes{
					Bytes: []byte{6, 0, 0, 0, 0, 0, 0, 0}, // 状态6（活跃池）
				},
			},
		},
	}

	// 订阅Raydium V4程序拥有的账户
	if err := sm.poolStreamClient.SubscribeAccounts("raydium_pool_accounts", &yellowstone_geyser_pb.SubscribeRequestFilterAccounts{
		Owner: []string{core.RAYDIUM_V4_PROGRAM_ID.String()},
		Filters: []*yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
			datasizeFilter,
			quoteMintFilter,
			marketProgramFilter,
			statusFilter,
		},
	}); err != nil {
		return fmt.Errorf("failed to subscribe to Raydium pool accounts: %w", err)
	}

	// 启动ping服务和处理goroutine
	go sm.startPingService(sm.poolStreamClient)
	go sm.handleRaydiumPoolStream()

	logger.Infof("Successfully subscribed to Raydium V4 pools for quote token: %s", sm.quoteTokenMint.String())
	return nil
}

// setupAccountSubscription 设置账户订阅
func (sm *StreamManager) setupAccountSubscription() error {

	tradeWalletKey := global.GetConfig().TradeWalletKey
	datasizeFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Datasize{
			Datasize: 165,
		},
	}

	memcmpFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 32,
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: tradeWalletKey.PublicKey().String(),
				},
			},
		},
	}

	return sm.accountStreamClient.SubscribeAccounts("token_accounts_stream", &yellowstone_geyser_pb.SubscribeRequestFilterAccounts{
		Owner: []string{solana.TokenProgramID.String()},
		Filters: []*yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
			datasizeFilter,
			memcmpFilter,
		},
	})
}

// handleTransactionStream 处理交易流
func (sm *StreamManager) handleTransactionStream() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Transaction stream handler panicked: %v", r)
			// 尝试重启
			go func() {
				time.Sleep(time.Second * 5)
				if err := sm.Restart(); err != nil {
					logger.Errorf("Failed to restart after panic: %v", err)
				}
			}()
		}
	}()

	for {
		select {
		case <-sm.ctx.Done():
			logger.Infof("Transaction stream handler stopped")
			return
		case out, ok := <-sm.mintStreamClient.Ch:
			if !ok {
				logger.Warnf("Transaction stream channel closed, attempting restart...")
				go sm.handleStreamError("transaction")
				return
			}

			sm.processTransactionUpdate(out)
		}
	}
}

// handleAccountStream 处理账户流
func (sm *StreamManager) handleAccountStream() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Account stream handler panicked: %v", r)
			go func() {
				time.Sleep(time.Second * 5)
				if err := sm.Restart(); err != nil {
					logger.Errorf("Failed to restart after panic: %v", err)
				}
			}()
		}
	}()

	for {
		select {
		case <-sm.ctx.Done():
			logger.Infof("Account stream handler stopped")
			return
		case out, ok := <-sm.accountStreamClient.Ch:
			if !ok {
				logger.Warnf("Account stream channel closed, attempting restart...")
				go sm.handleStreamError("account")
				return
			}

			sm.processAccountUpdate(out)
		}
	}
}

// handleStreamError 处理流错误
func (sm *StreamManager) handleStreamError(streamType string) {
	logger.Errorf("%s stream error detected, attempting to restart...", streamType)

	for i := 0; i < sm.maxRetries; i++ {
		time.Sleep(sm.retryInterval)

		if err := sm.Restart(); err != nil {
			logger.Errorf("Restart attempt %d failed: %v", i+1, err)
			continue
		}

		logger.Infof("Successfully restarted stream manager after %s stream error", streamType)
		return
	}

	logger.Errorf("Failed to restart stream manager after %d attempts", sm.maxRetries)
}

// monitorConnection 监控连接状态
func (sm *StreamManager) monitorConnection() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.mu.RLock()
			isRunning := sm.isRunning
			sm.mu.RUnlock()

			if !isRunning {
				continue
			}

			// 检查连接健康状态
			if !sm.isConnectionHealthy() {
				logger.Warnf("Connection health check failed, restarting...")
				go func() {
					if err := sm.Restart(); err != nil {
						logger.Errorf("Failed to restart during health check: %v", err)
					}
				}()
			}
		}
	}
}

// isConnectionHealthy 检查连接是否健康
func (sm *StreamManager) isConnectionHealthy() bool {
	// 实现连接健康检查逻辑
	// 可以通过发送ping或检查最近接收到的数据来判断
	return true // 简化实现
}

// startPingService 启动ping服务
func (sm *StreamManager) startPingService(streamClient *goyser.StreamClient) {
	go func() {
		ticker := time.NewTicker(time.Second * 15)
		defer ticker.Stop()

		for {
			select {
			case <-sm.ctx.Done():
				return
			case <-ticker.C:
				err := streamClient.SendCustomRequest(&yellowstone_geyser_pb.SubscribeRequest{
					Ping: &yellowstone_geyser_pb.SubscribeRequestPing{},
				})
				if err != nil {
					logger.Errorf("Failed to send ping: %v", err)
					go sm.Restart()
				}
			}
		}
	}()
}

// processTransactionUpdate 处理交易更新 (从原main.go中提取的逻辑)
func (sm *StreamManager) processTransactionUpdate(out *yellowstone_geyser_pb.SubscribeUpdate) {
	if out == nil || out.GetTransaction() == nil {
		logger.Debugf("Received nil update from stream")
		return
	}

	innerInstructions := out.GetTransaction().Transaction.Meta.InnerInstructions
	accountsAddress := sm.tradeBot.GetAccountList(out)
	var innerProgramID string = "nativeProgramID"
	var currentInstruction *yellowstone_geyser_pb.InnerInstruction

	for _, innerInstruction := range innerInstructions {
		instructions := innerInstruction.Instructions

		for _, instruction := range instructions {
			programIndex := instruction.ProgramIdIndex

			if programIndex >= uint32(len(accountsAddress)) {
				logger.Debugf("programIndex >= uint32(len(accountKeys))")
				continue
			}
			innerProgramID = accountsAddress[programIndex]
			if innerProgramID == core.PUMP_PROGRAM_ID.String() ||
				innerProgramID == core.AMM_PROGRAM_ID.String() ||
				innerProgramID == core.RAYDIUM_V4_PROGRAM_ID.String() ||
				innerProgramID == core.RAYDIUM_LAUNCHPAD_PROGRAM_ID.String() ||
				innerProgramID == core.RAYDIUM_CPMM_PROGRAM_ID.String() {
				currentInstruction = instruction
				break
			}
		}
	}

	ctx := context.Background()

	// 处理PumpFun交易
	if slices.Contains(accountsAddress, core.PUMP_PROGRAM_ID.String()) || innerProgramID == core.PUMP_PROGRAM_ID.String() {
		if !global.GetConfig().PumpTradeEnable {
			logger.Debugf("pump trade is disabled")
			return
		}

		var tradeInfo *global.TradeEventInfo
		if innerProgramID == core.PUMP_PROGRAM_ID.String() {
			tradeInfo = sm.tradeBot.PhotonRouterPumpParse(out, currentInstruction, ctx)
		} else {
			tradeInfo = sm.tradeBot.PumpParse(out, ctx)
		}

		if tradeInfo != nil && sm.transactionMonitor != nil {
			if tradeInfo.IsBuy {
				go sm.transactionMonitor.ProcessTransaction(tradeInfo)
			} else {
				go sm.transactionMonitor.ProcessSellTransaction(tradeInfo)
			}
			go sm.walletMonitor.CheckExitConditions("pumpfun", tradeInfo.MintAddress, tradeInfo.CurrentPrice)
		}
		return
	}

	// 处理PumpAMM交易
	if slices.Contains(accountsAddress, core.AMM_PROGRAM_ID.String()) || innerProgramID == core.AMM_PROGRAM_ID.String() {
		if !global.GetConfig().PumpAmmTradeEnable {
			logger.Debugf("pumpamm trade is disabled")
			return
		}

		var ammTradeInfo *global.AmmTradeEventInfo
		ammTradeInfo = sm.tradeBot.PumpAmmParse(out, ctx)

		if ammTradeInfo == nil && innerProgramID == core.AMM_PROGRAM_ID.String() {
			ammTradeInfo = sm.tradeBot.PhotonRouterAmmParse(out, currentInstruction, ctx)
		}

		if ammTradeInfo != nil && sm.transactionMonitor != nil {
			if ammTradeInfo.IsBuy {
				go sm.transactionMonitor.ProcessAmmTransaction(ammTradeInfo)
			} else {
				go sm.transactionMonitor.ProcessAmmSellTransaction(ammTradeInfo)
			}
			go sm.walletMonitor.CheckExitConditions("pumpamm", ammTradeInfo.MintAddress.String(), ammTradeInfo.CurrentPrice)
		}
		return
	}

	// 处理Raydium V4交易
	if slices.Contains(accountsAddress, core.RAYDIUM_V4_PROGRAM_ID.String()) {
		if !global.GetConfig().RaydiumTradeEnable {
			logger.Debugf("raydium trade is disabled")
			return
		}

		raydiumV4TradeInfo := sm.tradeBot.RaydiumV4Parse(out, ctx)

		if raydiumV4TradeInfo != nil && sm.transactionMonitor != nil {
			marketId := raydiumV4TradeInfo.MarketId
			marketLayout := global.GetOpenBookMarketState(marketId.String())
			if marketLayout == nil {
				logger.Warnf("marketLayout dont exist %s", marketId.String())
				return
			}
			poolId := raydiumV4TradeInfo.AmmPool
			poolLayout := global.GetRaydiumPoolLayout(poolId.String())
			if poolLayout == nil {
				logger.Warnf("poolLayout dont exist %s", poolId.String())
				return
			}
			if raydiumV4TradeInfo.IsBuy {
				go sm.transactionMonitor.ProcessRaydiumV4Transaction(raydiumV4TradeInfo, marketLayout, poolLayout)
			} else {
				go sm.transactionMonitor.ProcessRaydiumV4SellTransaction(raydiumV4TradeInfo, marketLayout, poolLayout)
			}
			go sm.walletMonitor.CheckExitConditions("raydiumV4", raydiumV4TradeInfo.MintAddress.String(), raydiumV4TradeInfo.CurrentPrice)
		}
		return
	}

	// 处理Raydium Launchpad交易
	if slices.Contains(accountsAddress, core.RAYDIUM_LAUNCHPAD_PROGRAM_ID.String()) || innerProgramID == core.RAYDIUM_LAUNCHPAD_PROGRAM_ID.String() {
		if !global.GetConfig().LaunchpadTradeEnable {
			logger.Debugf("launchpad trade is disabled")
			return
		}

		var launchpadTradeInfo *global.LaunchpadTradeEventInfo
		launchpadTradeInfo = sm.tradeBot.RaydiumLaunchpadParse(out, ctx)
		if launchpadTradeInfo == nil && innerProgramID == core.RAYDIUM_LAUNCHPAD_PROGRAM_ID.String() {
			launchpadTradeInfo = sm.tradeBot.WrapLaunchpadParse(out, currentInstruction, ctx)
		}

		if launchpadTradeInfo != nil && sm.transactionMonitor != nil {
			if launchpadTradeInfo.IsBuy {
				go sm.transactionMonitor.ProcessLaunchpadTransaction(launchpadTradeInfo)
			} else {
				go sm.transactionMonitor.ProcessLaunchpadSellTransaction(launchpadTradeInfo)
			}
			go sm.walletMonitor.CheckExitConditions("launchpad", launchpadTradeInfo.MintAddress.String(), launchpadTradeInfo.CurrentPrice)
		}
		return
	}

	// 处理Raydium CPMM交易
	if slices.Contains(accountsAddress, core.RAYDIUM_CPMM_PROGRAM_ID.String()) || innerProgramID == core.RAYDIUM_CPMM_PROGRAM_ID.String() {
		if !global.GetConfig().LaunchpadTradeEnable {
			logger.Debugf("cpmm trade is disabled")
			return
		}

		var cpmmTradeInfo *global.CPMMTradeEventInfo

		if innerProgramID == core.RAYDIUM_CPMM_PROGRAM_ID.String() {
			cpmmTradeInfo = sm.tradeBot.UniversalCpmmParse(out, currentInstruction, ctx)
		} else {
			cpmmTradeInfo = sm.tradeBot.RaydiumCpmmParse(out, ctx)
		}

		if cpmmTradeInfo != nil && sm.transactionMonitor != nil {
			if cpmmTradeInfo.IsBuy {
				go sm.transactionMonitor.ProcessCpmmTransaction(cpmmTradeInfo)
			} else {
				go sm.transactionMonitor.ProcessCpmmSellTransaction(cpmmTradeInfo)
			}
			go sm.walletMonitor.CheckExitConditions("cpmm", cpmmTradeInfo.MintAddress.String(), cpmmTradeInfo.CurrentPrice)
		}
	}
}

// processAccountUpdate 处理账户更新 (从原main.go中提取的逻辑)
func (sm *StreamManager) processAccountUpdate(out *yellowstone_geyser_pb.SubscribeUpdate) {
	account := out.GetAccount()
	if account == nil {
		return
	}

	// 获取账户公钥
	userPubkey := base58.Encode(account.Account.Pubkey)
	logger.Debugf("收到账户更新: %s", userPubkey)

	// 获取账户所有者
	owner := base58.Encode(account.Account.Owner)

	// 获取账户数据
	accountData := account.Account.GetData()
	if len(accountData) == 0 {
		logger.Warnf("账户数据为空: %s", base58.Encode(accountData[:]))
		return
	}

	// 处理余额更新逻辑
	if sm.balanceManager != nil {
		// 这里需要根据实际的balanceManager接口来实现
		// 暂时保留原来的逻辑框架
		lamports := account.Account.Lamports
		solBalance := float64(lamports) / 1e9

		logger.Debugf("owner:%s ,账号地址:%s,当前账户余额: %.8f SOL (%d lamports)",
			owner, userPubkey, solBalance, lamports)
	}
}

// IsRunning 检查流管理器是否正在运行
func (sm *StreamManager) IsRunning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isRunning
}

// GetStatus 获取流管理器状态
func (sm *StreamManager) GetStatus() map[string]any {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]any{
		"running":       sm.isRunning,
		"grpc_url":      sm.grpcUrl,
		"max_retries":   sm.maxRetries,
		"config":        sm.subscriptionConfig,
		"quote_token":   sm.quoteTokenMint.String(),
		"market_stream": sm.marketStreamClient != nil,
		"pool_stream":   sm.poolStreamClient != nil,
	}
}

// handleOpenBookMarketStream 处理OpenBook市场流
func (sm *StreamManager) handleOpenBookMarketStream() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("OpenBook market stream handler panicked: %v", r)
			go func() {
				time.Sleep(time.Second * 5)
				if err := sm.Restart(); err != nil {
					logger.Errorf("Failed to restart after panic: %v", err)
				}
			}()
		}
	}()

	for {
		select {
		case <-sm.ctx.Done():
			logger.Infof("OpenBook market stream handler stopped")
			return
		case update, ok := <-sm.marketStreamClient.Ch:
			if !ok {
				logger.Warnf("OpenBook market stream channel closed, attempting restart...")
				go sm.handleStreamError("openbook_market")
				return
			}

			if update == nil {
				logger.Debugf("Received nil update from OpenBook market stream")
				continue
			}

			// 检查更新是否包含账户数据
			accountUpdate := update.GetAccount()
			if accountUpdate == nil {
				logger.Debugf("Received nil account data from OpenBook market stream")
				continue
			}

			// 处理市场账户更新
			sm.handleOpenBookMarketUpdate(accountUpdate)
		}
	}
}

// handleRaydiumPoolStream 处理Raydium池流
func (sm *StreamManager) handleRaydiumPoolStream() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Raydium pool stream handler panicked: %v", r)
			go func() {
				time.Sleep(time.Second * 5)
				if err := sm.Restart(); err != nil {
					logger.Errorf("Failed to restart after panic: %v", err)
				}
			}()
		}
	}()

	for {
		select {
		case <-sm.ctx.Done():
			logger.Infof("Raydium pool stream handler stopped")
			return
		case update, ok := <-sm.poolStreamClient.Ch:
			if !ok {
				logger.Warnf("Raydium pool stream channel closed, attempting restart...")
				go sm.handleStreamError("raydium_pool")
				return
			}

			if update == nil {
				logger.Debugf("Received nil update from Raydium pool stream")
				continue
			}

			// 检查更新是否包含账户数据
			accountUpdate := update.GetAccount()
			if accountUpdate == nil {
				logger.Debugf("Received nil account data from Raydium pool stream")
				continue
			}

			// 处理池账户更新
			sm.handleRaydiumPoolUpdate(accountUpdate)
		}
	}
}

// handleOpenBookMarketUpdate 处理单个OpenBook市场账户更新
func (sm *StreamManager) handleOpenBookMarketUpdate(accountUpdate *yellowstone_geyser_pb.SubscribeUpdateAccount) {
	pubkey := base58.Encode(accountUpdate.Account.Pubkey)
	marketID, err := solana.PublicKeyFromBase58(pubkey)
	if err != nil {
		logger.Warnf("Invalid market pubkey: %s, error: %v", pubkey, err)
		return
	}

	var marketLayout layouts.MarketStateLayoutV3
	if err := marketLayout.Decode(accountUpdate.Account.Data); err != nil {
		logger.Warnf("Failed to decode OpenBook market data for account %s: %v", pubkey, err)
		return
	}

	baseMint := marketLayout.BaseMint.String()
	quoteMint := marketLayout.QuoteMint.String()
	baseVault := marketLayout.BaseVault.String()
	quoteVault := marketLayout.QuoteVault.String()
	ownAddress := marketLayout.OwnAddress.String()

	// 创建新的MarketState对象
	marketState := global.NewMarketStateFromLayout(marketID, marketLayout)

	// 存储到全局映射中以供其他组件访问
	global.SetOpenBookMarketState(marketID.String(), marketState)

	// 同时按baseMint存储以便通过代币查找
	global.SetOpenBookMarketState(baseMint, marketState)

	logger.Debugf("OpenBook market update: Market=%s, OwnAddress=%s, BaseMint=%s, QuoteMint=%s, baseVault=%s, quoteVault=%s",
		pubkey, ownAddress, baseMint, quoteMint, baseVault, quoteVault)
}

// handleRaydiumPoolUpdate 处理单个Raydium池账户更新
func (sm *StreamManager) handleRaydiumPoolUpdate(accountUpdate *yellowstone_geyser_pb.SubscribeUpdateAccount) {
	pubkey := base58.Encode(accountUpdate.Account.Pubkey)
	poolID, err := solana.PublicKeyFromBase58(pubkey)
	if err != nil {
		logger.Warnf("Invalid pool pubkey: %s, error: %v", pubkey, err)
		return
	}

	// 首先检查我们的全局映射中是否已有此池布局
	poolLayout := global.GetRaydiumPoolLayout(poolID.String())

	// 如果池布局在我们的映射中不存在或需要更新
	if poolLayout == nil {
		// 解码池状态
		poolLayout = new(layouts.LIQUIDITY_STATE_LAYOUT_V4)
		if err := poolLayout.Decode(accountUpdate.Account.Data); err != nil {
			logger.Warnf("Failed to decode Raydium V4 pool data for account %s: %v", pubkey, err)
			return
		}

		// 将解码的布局存储到全局映射中
		global.SetRaydiumPoolLayout(poolID.String(), poolLayout)
	} else {
		// 用新数据更新现有池布局
		if err := poolLayout.Decode(accountUpdate.Account.Data); err != nil {
			logger.Warnf("Failed to update Raydium V4 pool data for account %s: %v", pubkey, err)
			return
		}
		// 用更新的布局更新全局映射
		global.SetRaydiumPoolLayout(poolID.String(), poolLayout)
	}

	// 提取池信息
	baseMint := poolLayout.BaseMint.String()
	quoteMint := poolLayout.QuoteMint.String()
	marketId := poolLayout.MarketId.String()

	logger.Debugf("Raydium V4 pool update: Pool=%s, BaseMint=%s, QuoteMint=%s, MarketId=%s",
		poolID.String(), baseMint, quoteMint, marketId)
}

// SetQuoteTokenMint 设置报价代币
func (sm *StreamManager) SetQuoteTokenMint(mint solana.PublicKey) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.quoteTokenMint = mint
}

// GetQuoteTokenMint 获取报价代币
func (sm *StreamManager) GetQuoteTokenMint() solana.PublicKey {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.quoteTokenMint
}
