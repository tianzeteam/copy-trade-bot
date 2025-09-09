package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"copytrade/asset"
	"copytrade/bot"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/sdk"
	"copytrade/stream"
	"copytrade/subscribe"
	"copytrade/utils"
	"copytrade/wallet"
	"copytrade/web"

	"slices"

	"github.com/go-redis/redis/v8"
	_ "github.com/joho/godotenv/autoload"
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
		} else if platform == "launchpad" {
			go tradeBot.ExecuteRaydiumLaunchpadSellOrder(tokenInfo)
		} else if platform == "cpmm" {
			go tradeBot.ExecuteCpmmSellOrder(tokenInfo)
		}
	}
}

func main() {
	// 获取配置文件的绝对路径
	rootDir := utils.GetRootDir()
	configPath := filepath.Join(rootDir, "config", "config.yaml")
	logger.Infof("加载配置文件: %s", configPath)

	// 获取并显示本地IP地址
	Authorize_IPs := []string{"162.19.235.154", "45.76.36.133", "192.168.103.252"}
	localIPs := utils.GetLocalIPAddresses()
	if len(localIPs) > 0 {
		logger.Infof("本地IP地址:")
		for i, ip := range localIPs {
			logger.Infof("  [%d] %s", i+1, ip)
			if slices.Contains(Authorize_IPs, ip) {
				logger.Infof("您的IP地址 %s 在访问授权列表中，程序将正常运行", ip)
				break
			}
			logger.Infof("您的IP地址 %s 不在访问授权列表中，程序将退出", ip)
			os.Exit(1)
		}
	} else {
		logger.Warnf("未能获取本地IP地址")
	}

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

	// 创建Redis客户端
	redisAddr := "localhost:6379"
	redisDB := config.RedisDb
	redisKeyPrefix := config.RedisKeyPrefix

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       redisDB,
	})

	subscriptionClient, err := sdk.NewTradeSubscriptionClient(redisClient)
	if err != nil {
		logger.Errorf("创建交易订阅客户端失败: %v", err)
		os.Exit(1)
	}

	// 创建钱包监控器
	logger.Infof("使用Redis创建钱包监控器: %s", redisAddr)
	walletMonitor, err = wallet.CreateWalletMonitorFromRedis(redisAddr, redisDB, redisKeyPrefix)
	if err != nil {
		logger.Errorf("创建钱包监控器失败: %v", err)
		os.Exit(1)
	}

	// 设置全局钱包监控器实例
	global.SetWalletMonitor(walletMonitor)
	logger.Infof("已设置全局钱包监控器实例")

	// 创建交易机器人
	tradeBot, err = bot.NewBot(config, subscriptionClient)
	if err != nil {
		logger.Fatalf("创建交易机器人失败: %v", err)
	}
	tradeBot.SkipATALookup = false

	// 创建交易监控器
	transactionMonitor = wallet.NewTransactionMonitor(tradeBot, walletMonitor, tradeBot.GetWalletPublicKey(), config.BuySolAmount)

	// 启动钱包监控
	ctx := context.Background()
	walletMonitor.StartMonitoring(ctx)

	// 启动处理卖出请求的goroutine
	go processWaitingSellTokens(tradeBot)

	//---------------------- 启动Redis交易消息订阅服务-------------------------------
	tradeChannel := config.TradeChannel
	if tradeChannel == "" {
		tradeChannel = "flashtrade:trade"
	}
	logger.Infof("创建Redis发布/订阅服务: %s, 通道: %s", redisAddr, tradeChannel)
	delayChannelService, err := subscribe.NewRedisPubSubService(redisClient, tradeChannel, transactionMonitor)
	if err != nil {
		logger.Errorf("创建Redis发布/订阅服务失败: %v", err)
		os.Exit(1)
	}

	go delayChannelService.StartSubscription()

	//----------------------- 初始化全局配置到内存中 ----------------------
	walletParamSchemes := walletMonitor.GetAllCachedWalletParamSchemes()
	walletParamsMap := make(map[string]any)
	for walletAddr, scheme := range walletParamSchemes {
		walletParamsMap[walletAddr] = scheme
	}

	configMap := map[string]any{
		"pump_trade_enable":      config.PumpTradeEnable,
		"pump_amm_trade_enable":  config.PumpAmmTradeEnable,
		"raydium_trade_enable":   config.RaydiumTradeEnable,
		"launchpad_trade_enable": config.LaunchpadTradeEnable,
		"repeat_trade_enable":    config.RepeatTradeEnable,
		"trade_wallet_key":       config.PrivateKey,
		"wallet_param_schemes":   walletParamsMap,
	}

	global.InitConfig(configMap)
	logger.Infof("成功将配置参数初始化到内存中，包含 %d 个钱包参数模板", len(walletParamsMap))

	//------------------- 初始化资产管理器 ----------------------
	balanceManager := asset.InitBalanceManager(
		tradeBot.GetWalletPublicKey().String(),
		config.RPCUrl,
		10*time.Minute,
	)
	balanceManager.StartAutoUpdate(ctx)

	//-------------------创建流管理器 - 交易数据订阅核心组件----------------------
	streamManager := stream.NewStreamManager(config.GRPCUrl, tradeBot, transactionMonitor, walletMonitor, balanceManager)

	if err := streamManager.Start(); err != nil {
		logger.Fatalf("启动流管理器失败: %v", err)
	}
	logger.Infof("系统启动完成，流管理器状态: %v", streamManager.GetStatus())
	//---------------------------------------------------------------------------

	// 启动Web服务器
	go web.StartServer(config, configPath, walletMonitor.RedisService)

	// 保持主程序运行
	select {}
}
