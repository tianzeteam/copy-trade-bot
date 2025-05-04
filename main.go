package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"copytrade/bot"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/types"
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

	for tokenInfo := range types.WaitingSellChan {
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
			tradeBot.PumpAmmSell(tokenInfo)
			continue
		}
		// 查找代币的交易信息
		tradeInfo := global.GetTradeInfo(mintAddress)

		if tradeInfo == nil {
			logger.Warnf("未找到代币%s的交易信息，无法执行卖出操作", tokenInfo.Mint)
			continue
		}
		createInfo := global.GetCreateInfo(mintAddress)
		if createInfo == nil {
			logger.Warnf("未找到代币%s的创建信息，无法执行卖出操作", tokenInfo.Mint)
			continue
		}
		bondingCurve := solana.MustPublicKeyFromBase58(createInfo.BondingCurve)
		vTokenReserves := tradeInfo.VirtualTokenReserves
		vSolReserves := tradeInfo.VirtualSolReserves

		go tradeBot.PumpSellMint(tokenInfo, bondingCurve, vTokenReserves, vSolReserves)
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
	transactionMonitor = wallet.NewTransactionMonitor(walletMonitor, tradeBot.GetWalletPublicKey(), config.BuySolAmount)

	// 启动钱包监控服务
	ctx := context.Background()
	walletMonitor.StartMonitoring(ctx)

	// 启动处理卖出请求的goroutine
	go processWaitingSellTokens(tradeBot)

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
	//includeTransaction := true
	// subscribe to the account you want to see txns from and set a custom filter name to filter them out later
	if err = mintStreamClient.SubscribeTransaction("transaction_stream", &yellowstone_geyser_pb.SubscribeRequestFilterTransactions{
		Vote:           &isVote,
		Failed:         &isFailed,
		AccountInclude: []string{core.PUMP_PROGRAM_ID.String(), core.AMM_PROGRAM_ID.String()},
	}, /* , &yellowstone_geyser_pb.SubscribeRequestFilterBlocks{
	AccountInclude:      []string{core.PUMP_PROGRAM_ID.String(), core.AMM_PROGRAM_ID.String()},
	IncludeTransactions: &includeTransaction} */); err != nil {
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

			/* 		if out.GetBlock() == nil {
				logger.Debug("Received update with nil block")
				continue

			}
			block := out.GetBlock()

			blockHash := block.Blockhash
			tradeBot.CacheLatestBlockhash(blockHash)

			logger.Debugf("block height %d ,blockHash %v", block.BlockHeight.GetBlockHeight(), blockHash) */
			if out.GetTransaction() == nil {
				logger.Info("Received update with nil transaction")
				continue //
			}

			accountKeys := out.GetTransaction().Transaction.Transaction.Message.AccountKeys
			accountsAddress := make([]string, len(accountKeys))
			for i, key := range accountKeys {
				accountsAddress[i] = base58.Encode(key)

			}

			if slices.Contains(accountsAddress, core.PUMP_PROGRAM_ID.String()) {
				tradeInfo := tradeBot.PumpMintParse(out, client, ctx)
				if tradeInfo != nil && transactionMonitor != nil {
					transactionMonitor.ProcessTransaction(tradeInfo)

				}
			}

			if slices.Contains(accountsAddress, core.AMM_PROGRAM_ID.String()) {
				// 处理AMM交易信息
				ammTradeInfo := tradeBot.PumpAmmParse(out, client, ctx)

				// 处理钱包监控AMM交易
				if ammTradeInfo != nil && transactionMonitor != nil {
					transactionMonitor.ProcessAmmTransaction(ammTradeInfo)
				}
			}

		}
	}()

	//-----------------------------  监听交易钱包账户变化 --------------------------------------------
	if err = client.AddStreamClient(ctx, "accounts_stream", yellowstone_geyser_pb.CommitmentLevel_CONFIRMED); err != nil {
		logger.Fatalf("添加账户流客户端失败: %v", err)
	}

	accountsStreamClient := client.GetStreamClient("accounts_stream")
	if accountsStreamClient == nil {
		logger.Fatal("客户端没有名为accounts_stream的流")
	}
	logger.Infof("开始监控交易钱包地址: %s", botPrivateKey.PublicKey().String())
	// 创建账户过滤器 - 监控两种类型的账户：
	// 1. 钱包主账户（SOL账户）
	// 2. 与钱包关联的所有SPL代币账户（通过过滤器过滤）
	tokenProgramID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	// 创建两个订阅请求，一个监控主账户，一个监控代币账户
	// 1. 监控钱包主账户
	if err = accountsStreamClient.SubscribeAccounts("main_account_stream", &yellowstone_geyser_pb.SubscribeRequestFilterAccounts{
		Account: []string{botPrivateKey.PublicKey().String()},
	}); err != nil {
		logger.Fatalf("订阅账户失败: %v", err)
	}

	// 2. 监控与钱包关联的代币账户
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
		Owner: []string{tokenProgramID}, // 只监控Token Program拥有的账户
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
			logger.Infof("收到账户更新: %s", userPubkey)
			// 获取账户所有者
			owner := base58.Encode(account.Account.Owner)
			logger.Infof("账户所有者: %s", owner)
			// 获取账户数据
			accountData := account.Account.GetData()
			if len(accountData) == 0 {
				logger.Warnf("账户数据为空: %s", base58.Encode(accountData[:]))
				continue
			}

			// 获取账户余额
			lamports := account.Account.Lamports
			solBalance := float64(lamports) / 1e9
			logger.Infof("owner:%s ,账号地址:%s,账户余额: %f SOL (%d lamports)", owner, userPubkey, solBalance, lamports)

			// 处理账户数据 - 这里可以根据需要解析特定的账户数据结构
			// 如果有特定的数据结构需要解析，可以在这里添加解析逻辑
			if owner == "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA" && len(accountData) >= 165 {
				// SPL代币账户数据结构
				_mint := solana.PublicKeyFromBytes(accountData[0:32])
				_owner := solana.PublicKeyFromBytes(accountData[32:64])
				_amount := binary.LittleEndian.Uint64(accountData[64:72])
				logger.Infof("代币账户信息 - Mint: %s, 所有者: %s, 余额: %d", _mint, _owner, _amount)
				if _amount > 100*1e6 {
					// 创建代币信息对象
					tokenInfo := types.TokenInfo{
						Mint:         _mint,
						Owner:        _owner,
						Amount:       _amount,
						User:         solana.PublicKeyFromBytes([]byte(userPubkey)), // token account ||main account
						UpdateTime:   time.Now(),
						EntryPrice:   0,
						CurrentPrice: 0,
						Platform:     "pumpfun",
					}

					// 发送代币信息到全局通道
					logger.Infof("发送代币信息到通道: Mint=%s, 余额=%d", tokenInfo.Mint, tokenInfo.Amount)

				} else {
					//删除代币信息
					logger.Infof("删除代币信息: Mint=%s", _mint.String())
					global.DelPendingHoldAsset(_mint.String())
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
