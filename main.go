package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"copytrade/asset"
	"copytrade/bot"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/utils"
	"copytrade/wallet"
	"copytrade/web"
	goyser "copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"slices"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mr-tron/base58"
)

var (
	tradeBot           *bot.Bot                   = nil
	walletMonitor      *wallet.WalletMonitor      = nil
	transactionMonitor *wallet.TransactionMonitor = nil
)

// processWaitingSellTokens 处理等待卖出的代币
func processWaitingSellTokens(tradeBot *bot.Bot) {
	logger.Infof("开始监听等待卖出的代币...")

	for tokenInfo := range queue.WaitingSellChan {
		logger.Infof("收到卖出请求: Mint=%s, 余额=%d, 用户=%s", tokenInfo.Mint, tokenInfo.Amount, tokenInfo.Owner)

		// 检查代币信息是否有效
		if tokenInfo.Amount <= 0 {
			logger.Warnf("代币余额为0，跳过卖出操作")
			continue
		}

		// 解析Mint地址
		mintAddress := tokenInfo.Mint.String()
		platform := tokenInfo.Platform
		if platform == "pumpamm" {
			//@TODO 使用多级卖出策略
			go tradeBot.ExecuteAmmSellOrder(tokenInfo)
			continue
		} else if platform == "pumpfun" {
			// 查找代币的交易信息
			tradeInfo := global.GetTradeInfo(mintAddress)

			if tradeInfo == nil {
				logger.Warnf("未找到代币%s的交易信息，无法执行卖出操作", tokenInfo.Mint)
				continue
			}

			vTokenReserves := tradeInfo.VirtualTokenReserves
			vSolReserves := tradeInfo.VirtualSolReserves

			go tradeBot.ExecuteSellOrder(tokenInfo, vTokenReserves, vSolReserves)

		} else if platform == "raydiumV4" {

			go tradeBot.ExecuteRaydiumV4SellOrder(tokenInfo)

		}

	}
}

func getRootDir() string {
	_, b, _, _ := runtime.Caller(0)
	// 直接返回当前文件所在目录，不再往上追加一级
	return filepath.Dir(b)
}

func main() {
	// 获取配置文件的绝对路径
	rootDir := getRootDir()
	configPath := filepath.Join(rootDir, "config", "config.yaml")
	logger.Infof("加载配置文件: %s", configPath)

	// 加载配置
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		logger.Errorf("加载配置失败: %v", err)
		os.Exit(1)
	}

	// 初始化日志系统
	var logFile *os.File
	if config.Logging.File != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(config.Logging.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("创建日志目录失败: %v\n", err)
			os.Exit(1)
		}

		// 打开日志文件
		logFile, err = os.OpenFile(config.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Printf("打开日志文件失败: %v\n", err)
			os.Exit(1)
		}
		defer logFile.Close()
		logger.Init(config.Logging.Level, config.Logging.Format, logFile)
	} else {
		logger.Init(config.Logging.Level, config.Logging.Format, nil)
	}

	// Initialize global logger
	logger.GetLogger()

	// 创建钱包监控器 - 使用Redis服务
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // 默认Redis地址
	}

	redisDB := 0                  // 默认使用DB 0
	redisKeyPrefix := "copytrade" // 默认键前缀

	logger.Infof("使用Redis创建钱包监控器: %s", redisAddr)
	walletMonitor, err = wallet.CreateWalletMonitorFromRedis(redisAddr, redisDB, redisKeyPrefix)
	if err != nil {
		logger.Errorf("创建钱包监控器失败: %v", err)
		os.Exit(1)
	}

	// get the geyser rpc address
	grpcUrl := config.GRPCUrl

	// create geyser client
	client, err := goyser.New(context.Background(), grpcUrl, nil)
	if err != nil {
		logger.Fatalf("创建geyser客户端失败: %v", err)
	}

	tradeBot, err = bot.NewBot(config)
	if err != nil {
		logger.Fatalf("创建交易机器人失败: %v", err)
	}
	tradeBot.SkipATALookup = false

	// 创建交易监控器
	transactionMonitor = wallet.NewTransactionMonitor(tradeBot, walletMonitor, tradeBot.GetWalletPublicKey(), config.BuySolAmount)

	// 启动钱包监控服务
	ctx := context.Background()

	walletMonitor.StartMonitoring(ctx)

	// 启动处理卖出请求的goroutine
	go processWaitingSellTokens(tradeBot)

	// 初始化全局配置到内存中
	configMap := map[string]any{
		// 交易平台启用状态
		"pump_trade_enable":    true,  // PumpFun交易启用
		"pumpamm_trade_enable": false, // PumpAMM交易禁用
		"raydium_trade_enable": false, // Raydium交易禁用

		// 交易触发和池子大小参数
		"min_follow_trigger_amount": config.MinFollowTriggerAmount,
		"max_follow_trigger_amount": 10.0,
		"min_pool_size":             200.0,
		"max_pool_size":             config.MaxPoolSize,

		// 交易参数
		"max_buy_amount":         config.BuySolAmount,
		"min_buy_amount":         config.BuySolAmount * 0.5, // 默认为买入金额的一半
		"sell_percentage":        100.0,                     // 默认卖出100%
		"stop_loss_percentage":   10.0,                      // 默认止损10%
		"take_profit_percentage": 50.0,                      // 默认止盈50%

		// 滑点参数
		"buy_slippage":  0.2,
		"sell_slippage": 0.2,

		// 交易手续费参数
		"buy_priority_fee":  0.005,
		"buy_tip_fee":       0.005,
		"sell_priority_fee": 0.03,
		"sell_tip_fee":      0.03,
	}

	global.InitConfig(configMap)
	logger.Infof("成功将配置参数初始化到内存中")

	// 初始化资产管理器并启动自动更新服务
	balanceManager := asset.InitBalanceManager(
		tradeBot.GetWalletPublicKey().String(),
		config.RPCUrl,
		5*time.Minute, // 每5分钟更新一次余额
	)
	balanceManager.StartAutoUpdate(ctx)

	// 启动交易监控
	logger.Infof("启动钱包监控服务，监控 %d 个钱包", len(walletMonitor.Wallets))

	// 启动Web服务器（在单独的goroutine中运行）
	go web.StartServer(configPath)

	if err = client.AddStreamClient(ctx, "mint_stream", yellowstone_geyser_pb.CommitmentLevel_CONFIRMED); err != nil {
		logger.Fatalf("添加流客户端失败: %v", err)
		return
	}

	// get the stream client
	mintStreamClient := client.GetStreamClient("mint_stream")
	if mintStreamClient == nil {
		logger.Fatal("客户端没有名为mint_stream的流")
		return
	}

	isVote := false
	isFailed := false

	// subscribe to the account you want to see txns from and set a custom filter name to filter them out later
	if err = mintStreamClient.SubscribeTransaction("transaction_stream", &yellowstone_geyser_pb.SubscribeRequestFilterTransactions{
		Vote:           &isVote,
		Failed:         &isFailed,
		AccountInclude: []string{core.PUMP_PROGRAM_ID.String(), core.AMM_PROGRAM_ID.String(), core.RAYDIUM_V4_PROGRAM_ID.String()},
	}); err != nil {
		logger.Fatalf("订阅交易失败: %v", err)
		return
	}
	startPingService(mintStreamClient)
	botPrivateKey := solana.MustPrivateKeyFromBase58(config.PrivateKey)

	// 使用goroutine处理mintStreamClient.Ch，避免阻塞主进程
	go func() {
		for out := range mintStreamClient.Ch {
			// Check if out is nil before accessing its methods
			if out == nil {
				logger.Warn("Received nil update from stream")
				continue
			}

			if out.GetTransaction() == nil {
				logger.Info("Received update with nil transaction")
				continue
			}

			accountKeys := out.GetTransaction().Transaction.Transaction.Message.AccountKeys
			accountsAddress := make([]string, len(accountKeys))
			for i, key := range accountKeys {
				accountsAddress[i] = base58.Encode(key)

			}

			if slices.Contains(accountsAddress, core.PUMP_PROGRAM_ID.String()) {
				if !global.Config.PumpTradeEnable {
					logger.Warn("pump trade is disabled")
					continue
				}
				tradeInfo := tradeBot.PumpParse(out, client, ctx)
				if tradeInfo != nil && transactionMonitor != nil {
					transactionMonitor.ProcessTransaction(tradeInfo)
					walletMonitor.CheckExitConditions("pumpfun", tradeInfo.MintAddress, tradeInfo.CurrentPrice)
					continue

				}

			}

			if slices.Contains(accountsAddress, core.AMM_PROGRAM_ID.String()) {
				// 处理AMM交易信息
				if !global.Config.PumpAmmTradeEnable {
					logger.Warn("pumpamm trade is disabled")
					continue
				}
				ammTradeInfo := tradeBot.PumpAmmParse(out, client, ctx)

				// 处理钱包监控AMM交易
				if ammTradeInfo != nil && transactionMonitor != nil {
					transactionMonitor.ProcessAmmTransaction(ammTradeInfo)
					walletMonitor.CheckExitConditions("pumpamm", ammTradeInfo.MintAddress.String(), ammTradeInfo.CurrentPrice)
					continue
				}

			}

			if slices.Contains(accountsAddress, core.RAYDIUM_V4_PROGRAM_ID.String()) {
				if !global.Config.RaydiumTradeEnable {
					logger.Warn("raydium trade is disabled")
					continue
				}
				raydiumV4TradeInfo := tradeBot.RaydiumV4Parse(out, client, ctx)

				// 处理钱包监控AMM交易
				if raydiumV4TradeInfo != nil && transactionMonitor != nil {
					marketId := raydiumV4TradeInfo.MarketId
					marketLayout := global.GetOpenBookMarketState(marketId.String())
					if marketLayout == nil {
						logger.Warnf(" marketLayout dont exist %s", marketId.String())
						return
					}
					poolId := raydiumV4TradeInfo.AmmPool
					poolLayout := global.GetRaydiumPoolLayout(poolId.String())
					if poolLayout == nil {
						logger.Warnf(" poolLayout dont exist %s", poolId.String())
						return
					}
					transactionMonitor.ProcessRaydiumV4Transaction(raydiumV4TradeInfo, marketLayout, poolLayout)
					walletMonitor.CheckExitConditions("raydiumV4", raydiumV4TradeInfo.MintAddress.String(), raydiumV4TradeInfo.CurrentPrice)
					continue
				}

			}

		}
	}()
	// -----------------------------------------订阅open market data ----------------------------------
	tradeBot.SubscribeToOpenBookMarkets(ctx, client, solana.SolMint)

	// -------------------------------------------订阅raydium pool v4 data--------------------------
	tradeBot.SubscribeToRaydiumPools(ctx, client, solana.SolMint)

	//-----------------------------  监听交易钱包账户变化 --------------------------------------------
	if err = client.AddStreamClient(ctx, "accounts_stream", yellowstone_geyser_pb.CommitmentLevel_CONFIRMED); err != nil {
		logger.Fatalf("添加账户流客户端失败: %v", err)
	}

	accountsStreamClient := client.GetStreamClient("accounts_stream")
	if accountsStreamClient == nil {
		logger.Fatal("客户端没有名为accounts_stream的流")
	}
	logger.Infof("开始监控钱包账号资产变更: %s", botPrivateKey.PublicKey().String())

	// 创建数据大小过滤器 - SPL代币账户的标准大小为165字节
	datasizeFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Datasize{
			Datasize: 165,
		},
	}

	// 创建内存比较过滤器 - 检查偏移量32处的所有者是否为我们的钱包
	memcmpFilter := &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
		Filter: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter_Memcmp{
			Memcmp: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp{
				Offset: 32,
				Data: &yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilterMemcmp_Base58{
					Base58: botPrivateKey.PublicKey().String(),
				},
			},
		},
	}

	// 订阅代币账户
	if err = accountsStreamClient.SubscribeAccounts("token_accounts_stream", &yellowstone_geyser_pb.SubscribeRequestFilterAccounts{
		Owner: []string{solana.TokenProgramID.String()}, // 只监控Token Program拥有的账户
		Filters: []*yellowstone_geyser_pb.SubscribeRequestFilterAccountsFilter{
			datasizeFilter, // 数据大小过滤器
			memcmpFilter,   // 内存比较过滤器
		},
	}); err != nil {
		logger.Fatalf("订阅账户失败: %v", err)
	}
	startPingService(accountsStreamClient)
	// loop through the stream and print the output
	go func() {
		for out := range accountsStreamClient.Ch {
			account := out.GetAccount()
			if account == nil {
				continue
			}
			// 获取账户公钥
			userPubkey := base58.Encode(account.Account.Pubkey)
			logger.Debugf("收到账户更新: %s", userPubkey)
			// 获取账户所有者
			owner := base58.Encode(account.Account.Owner)
			//logger.Infof("账户所有者: %s", owner)
			// 获取账户数据
			accountData := account.Account.GetData()
			if len(accountData) == 0 {
				logger.Warnf("账户数据为空: %s", base58.Encode(accountData[:]))
				continue
			}

			// 获取账户余额
			preSolBalance := balanceManager.GetSolBalanceValue()
			lamports := account.Account.Lamports
			solBalance := float64(lamports) / 1e9
			changeSolBalance := preSolBalance - solBalance
			logger.Debugf("owner:%s ,账号地址:%s,变化前账号余额：%.8f,当前账户余额: %.8f ,sol变化: %.8f SOL (%d lamports)", owner, userPubkey, preSolBalance, solBalance, changeSolBalance, lamports)
			if changeSolBalance != 0 {
				balanceManager.UpdateSolBalance()
			} else {
				logger.Debugf("SOL余额未变化，跳过更新")
				continue
			}
			// 如果有特定的数据结构需要解析，可以在这里添加解析逻辑
			if owner == solana.TokenProgramID.String() && len(accountData) >= 165 {
				// SPL代币账户数据结构
				_mint := solana.PublicKeyFromBytes(accountData[0:32])
				_owner := solana.PublicKeyFromBytes(accountData[32:64])
				_amount := binary.LittleEndian.Uint64(accountData[64:72])
				logger.Infof("代币账户信息 - Mint: %s, 所有者: %s, token数量: %d", _mint, _owner, _amount)

				assetInfo, exist := global.GetPendingHoldAsset(_mint.String())
				lastestAmmTradeInfo := global.GetAmmTradeInfo(_mint.String())
				lastestPumpTradeInfo := global.GetTradeInfo(_mint.String())
				lastestRaydiumV4TradeInfo := global.GetAmmRaydiuV4TradeInfo(_mint.String())

				if assetInfo != nil {
					logger.Infof("Asset info for %s: Amount=%d, EntryPrice=%.8f", _mint.String(), assetInfo.Amount, assetInfo.EntryPrice)
				} else {
					logger.Infof("Asset info for %s is nil", _mint.String())
				}

				if exist {
					platform := "unknown"
					//@TODO
					currentPrice := changeSolBalance / (float64(assetInfo.Amount) / math.Pow(10, float64(assetInfo.Decimal)))
					if lastestPumpTradeInfo != nil {
						platform = "pumpfun"
						currentPrice = lastestPumpTradeInfo.CurrentPrice

					}
					if lastestAmmTradeInfo != nil {
						platform = "pumpamm"
						currentPrice = lastestAmmTradeInfo.CurrentPrice

					}

					if lastestRaydiumV4TradeInfo != nil {
						platform = "raydiumV4"
						currentPrice = lastestRaydiumV4TradeInfo.CurrentPrice

					}
					// Check if we have a trade record for this token
					tradeSetting, existCopyTrade := walletMonitor.TradeRecords[_mint.String()]
					logger.Infof("existCopyTrade %v", existCopyTrade)
					logger.Infof("tradeSetting %v", tradeSetting)

					// 检查代币是否已经被发送到卖出通道
					if existCopyTrade && changeSolBalance > 0 && platform != "unknown" {
						// 检查是否可以卖出该代币（防止多次卖出）
						if global.CanSellToken(_mint.String()) {
							// 更新代币卖出时间，防止短时间内重复卖出
							global.UpdateTokenSellTime(_mint.String())

							// tradeRecord := walletMonitor.BotTrades[_mint.String()]
							strategyName := tradeSetting.Settings.Name
							exitTimeSec := tradeSetting.Settings.ExitTimeSec
							nowTime := time.Now().Unix()
							waitSellTime := int64(exitTimeSec) + nowTime

							logger.Infof("将在 %d 秒后卖出代币 %s ,卖出时间: %s", exitTimeSec, _mint.String(), time.Unix(waitSellTime, 0).Format("2006-01-02 15:04:05"))
							time.Sleep(time.Duration(exitTimeSec) * time.Second)
							queue.SendTokenToSellChannel(assetInfo, currentPrice, platform, strategyName, "due_to_exit")

						} else {
							logger.Warnf("代币 %s 已经在卖出周期中，跳过重复卖出", _mint.String())
						}
					}
				}

			}
		}
	}()

	select {}
}

// startPingService 启动一个定时ping服务，保持与服务器的连接活跃
func startPingService(streamClient *goyser.StreamClient) {
	go func() {
		ticker := time.NewTicker(time.Second * 10)
		defer ticker.Stop()
		for range ticker.C {
			lastPingTime := time.Now()
			err := streamClient.SendCustomRequest(&yellowstone_geyser_pb.SubscribeRequest{
				Ping: &yellowstone_geyser_pb.SubscribeRequestPing{},
			})
			if err != nil {
				logger.Errorf("发送 ping 失败: %v", err)
				return
			}

			logger.Debugf("发送 ping - 时间:%v", lastPingTime.Format("15:04:05.000"))
		}
	}()
}
