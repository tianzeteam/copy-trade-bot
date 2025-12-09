package bot

import (
	"context"
	"strings"
	"time"

	"copytrade/asset"
	"copytrade/constants"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/pump"
	pumpV2 "copytrade/pumpV2"
	"copytrade/queue"
	"copytrade/types"

	goyser "copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	ag_solanago "github.com/gagliardetto/solana-go"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	_ "github.com/joho/godotenv/autoload"
)

// 检查是否是自己的交易
// 定义常量以提高可读性
const (
	SolDecimals   = 1e9 // SOL有9位小数
	TokenDecimals = 1e6 // 代币有6位小数

	// Token2022 程序地址
	Token2022Program = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
)

// TradeEventInfo 存储交易事件信息
type TradeEventInfo struct {
	UserAddress          string    // 用户地址
	SolAmount            uint64    // Sol数量
	TokenAmount          uint64    // Token数量
	IsBuy                bool      // 是否买入
	MintAddress          string    // Mint地址
	TimeStamp            int64     // 时间戳
	VirtualSolReserves   uint64    // 虚拟Sol储备
	VirtualTokenReserves uint64    // 虚拟Token储备
	RealSolReserves      uint64    // 实际Sol储备
	RealTokenReserves    uint64    // 实际Token储备
	CurrentPrice         float64   // 当前价格
	TradeTime            string    // 交易时间
	Milliseconds         int64     // 毫秒时间戳
	ReceivedAt           time.Time // 接收时间
}
type MyBuyMap struct {
	UserAddress string
	MintAddress string
	EntryPrice  float64
	Amount      uint64
	SoldLevels  map[int]bool // 记录已经卖出的价格档位
	TimeStamp   int64
}

// 全局交易信息映射表已移至 global_maps.go

// PumpSellMint 处理代币卖出逻辑
func (b *Bot) PumpSell(tokenInfo queue.TokenInfo, feeRecipient solana.PublicKey, virtualTokenReserves, virtualSolReserves uint64) {

	// 使用策略模式进行卖出
	logger.Infof("使用策略模式卖出代币: %s", tokenInfo.Mint.String())

	// 确保卖出策略已初始化
	if b.sellStrategy == nil {
		b.sellStrategy = NewMultiLevelSellStrategy(b.Config.MultiSellLevels)
		logger.Errorf("初始化多级卖出策略: %s", b.sellStrategy.Name())
	}

	// 执行卖出策略
	b.ExecuteSellStrategy(tokenInfo, feeRecipient, virtualTokenReserves, virtualSolReserves)

}

// PumpParse 使用pipeline模式处理交易事件
// 根据 accountsAddress 是否包含 Token2022 程序地址来选择使用 pumpV2 或 pump 模块
func (b *Bot) PumpParse(out *yellowstone_geyser_pb.SubscribeUpdate, accountsAddress []string, ctx context.Context, signature string) *global.TradeEventInfo {
	if out.GetTransaction() == nil {
		return nil
	}

	txn := out.GetTransaction()
	logMessages := txn.Transaction.Meta.LogMessages

	// 获取 InnerInstructions 用于解析 CPI 事件
	innerInstructions := txn.Transaction.Meta.InnerInstructions

	// 使用传入的 accountsAddress 参数组装 accountKeysPubkeys
	var accountKeysPubkeys ag_solanago.PublicKeySlice
	for _, addrStr := range accountsAddress {
		pubkey, err := ag_solanago.PublicKeyFromBase58(addrStr)
		if err != nil {
			logger.Warnf("无效的账户地址: %s, 错误: %v", addrStr, err)
			continue
		}
		accountKeysPubkeys = append(accountKeysPubkeys, pubkey)
	}

	// 转换 InnerInstructions 为 solana-go 类型
	var innerIxs []ag_rpc.InnerInstruction
	for _, innerIx := range innerInstructions {
		var instructions []ag_solanago.CompiledInstruction
		for _, ix := range innerIx.Instructions {
			// 边界检查：确保 ProgramIdIndex 在有效范围内
			if int(ix.ProgramIdIndex) >= len(accountKeysPubkeys) {
				logger.Warnf("跳过无效的 InnerInstruction: ProgramIdIndex=%d 超出 accountKeys 范围(len=%d)",
					ix.ProgramIdIndex, len(accountKeysPubkeys))
				continue
			}

			// 转换 accounts 为 uint16 slice，并进行边界检查
			accounts := make([]uint16, 0, len(ix.Accounts))
			for _, acc := range ix.Accounts {
				if int(acc) >= len(accountKeysPubkeys) {
					logger.Warnf("跳过无效的 account index: %d 超出范围(len=%d)", acc, len(accountKeysPubkeys))
					continue
				}
				accounts = append(accounts, uint16(acc))
			}

			instructions = append(instructions, ag_solanago.CompiledInstruction{
				ProgramIDIndex: uint16(ix.ProgramIdIndex),
				Accounts:       accounts,
				Data:           ag_solanago.Base58(ix.Data),
			})
		}

		// 只添加有有效指令的 InnerInstruction
		if len(instructions) > 0 {
			innerIxs = append(innerIxs, ag_rpc.InnerInstruction{
				Index:        uint16(innerIx.Index),
				Instructions: instructions,
			})
		}
	}

	// 检查是否包含 Token2022 程序地址
	token2022ProgramID := solana.Token2022ProgramID.String()
	isToken2022 := false
	for _, addr := range accountsAddress {
		if addr == token2022ProgramID {
			isToken2022 = true
			break
		}
	}

	if isToken2022 {
		// 使用 pumpV2 模块解析 (Token-2022)
		logger.Debugf("检测到 Token2022 程序，使用 pumpV2 模块解析")
		eventsV2, errV2 := pumpV2.DecodeEventsInTransaction(logMessages, innerIxs, accountKeysPubkeys, core.PUMP_PROGRAM_ID, signature)
		if errV2 != nil {
			// 检查是否是未知事件类型错误
			if strings.Contains(errV2.Error(), "unknown discriminator") {
				logger.Debugf("pumpV2 遇到未知事件类型，跳过: %v", errV2)
				return nil
			}
			logger.Errorf("pumpV2 解析事件失败: %v", errV2)
			return nil
		}

		// 处理 pumpV2 事件
		var tradeInfo *global.TradeEventInfo
		for _, evt := range eventsV2 {
			result := b.processPumpV2Event(evt)
			if result != nil {
				tradeInfo = result
			}
		}
		return tradeInfo
	} else {
		// 使用 pump V1 模块解析 (标准 Token Program)
		logger.Debugf("未检测到 Token2022 程序，使用 pump V1 模块解析")
		eventsV1, errV1 := pump.DecodeEventsInLogs(logMessages)
		if errV1 != nil {
			// 检查是否是未知事件类型错误
			if strings.Contains(errV1.Error(), "unknown discriminator") {
				logger.Debugf("pump V1 遇到未知事件类型，跳过: %v", errV1)
				return nil
			}
			logger.Errorf("pump V1 解析事件失败: %v", errV1)
			return nil
		}

		// 处理 pump V1 事件
		var tradeInfo *global.TradeEventInfo
		for _, evt := range eventsV1 {
			result := b.processPumpEvent(evt)
			if result != nil {
				tradeInfo = result
			}
		}
		return tradeInfo
	}

}

// processPumpEvent 处理 pump 模块的事件
func (b *Bot) processPumpEvent(evt *pump.Event) *global.TradeEventInfo {
	// 检查事件类型
	if evt.Name == "TradeEvent" {
		if tradeEvent, ok := evt.Data.(*pump.TradeEventEventData); ok {
			return b.handlePumpTradeEvent(tradeEvent)
		} else {
			logger.Errorf("TradeEvent 类型断言失败，实际类型: %T", evt.Data)
		}
	}

	// 检查是否是创建事件
	if evt.Name == "CreateEvent" {
		if createEvent, ok := evt.Data.(*pump.CreateEventEventData); ok {
			b.handlePumpCreateEvent(createEvent)
		} else {
			logger.Errorf("CreateEvent 类型断言失败，实际类型: %T", evt.Data)
		}
	}

	return nil
}

// processPumpV2Event 处理 pumpV2 模块的事件
func (b *Bot) processPumpV2Event(evt any) *global.TradeEventInfo {

	// 尝试解析为 TradeEvent
	if tradeEvent, ok := evt.(*pumpV2.TradeEvent); ok {
		return b.handlePumpV2TradeEvent(tradeEvent)
	}

	// 尝试解析为 CreateEvent
	if createEvent, ok := evt.(*pumpV2.CreateEvent); ok {
		b.handlePumpV2CreateEvent(createEvent)
		return nil
	}

	// 尝试解析为 ExtendAccountEvent
	if _, ok := evt.(*pumpV2.ExtendAccountEvent); ok {
		return nil
	}

	// 其他不需要处理的事件类型
	switch evt.(type) {
	case *pumpV2.CompleteEvent,
		*pumpV2.CompletePumpAmmMigrationEvent,
		*pumpV2.CollectCreatorFeeEvent,
		*pumpV2.SetCreatorEvent,
		*pumpV2.SetMetaplexCreatorEvent,
		*pumpV2.SetParamsEvent,
		*pumpV2.UpdateGlobalAuthorityEvent,
		*pumpV2.AdminSetCreatorEvent,
		*pumpV2.AdminSetIdlAuthorityEvent,
		*pumpV2.AdminUpdateTokenIncentivesEvent,
		*pumpV2.ClaimTokenIncentivesEvent,
		*pumpV2.CloseUserVolumeAccumulatorEvent,
		*pumpV2.InitUserVolumeAccumulatorEvent,
		*pumpV2.SyncUserVolumeAccumulatorEvent:
		return nil
	}

	logger.Warnf("pumpV2 未知事件类型: %T", evt)
	return nil
}

// handlePumpTradeEvent 处理 pump 模块的 TradeEvent
func (b *Bot) handlePumpTradeEvent(tradeEvent *pump.TradeEventEventData) *global.TradeEventInfo {
	// 获取基本信息
	botWallet := global.GetConfig().TradeWalletKey.PublicKey()
	mintAddress := tradeEvent.Mint.String()
	currentPrice := float64(tradeEvent.VirtualSolReserves/1e9) / float64(tradeEvent.VirtualTokenReserves/1e6)

	logger.Debugf("handlePumpTradeEvent: Mint=%s, User=%s, IsBuy=%v, Price=%.9f, SolAmount=%d, TokenAmount=%d",
		mintAddress, tradeEvent.User.String(), tradeEvent.IsBuy, currentPrice,
		tradeEvent.SolAmount, tradeEvent.TokenAmount)

	// 创建交易信息对象
	tradeInfo := &global.TradeEventInfo{
		PumpVersion:          constants.PumpVersionV1,
		MintAddress:          mintAddress,
		UserAddress:          tradeEvent.User.String(),
		FeeRecipient:         tradeEvent.FeeRecipient,
		SolAmount:            tradeEvent.SolAmount,
		TokenAmount:          tradeEvent.TokenAmount,
		IsBuy:                tradeEvent.IsBuy,
		VirtualSolReserves:   tradeEvent.VirtualSolReserves,
		VirtualTokenReserves: tradeEvent.VirtualTokenReserves,
		RealSolReserves:      tradeEvent.RealSolReserves,
		RealTokenReserves:    tradeEvent.RealTokenReserves,
		Creator:              tradeEvent.Creator,
		TimeStamp:            time.Now().Unix(),
		TradeTime:            time.Now().Format("2006-01-02 15:04:05.000"),
		Milliseconds:         time.Now().UnixMilli(),
		ReceivedAt:           time.Now(),
		CurrentPrice:         currentPrice,
	}

	// 存储到全局映射表中
	global.SetTradeInfo(mintAddress, tradeInfo)

	// ds清仓
	if global.GetConfig().DevSell {
		if tradeEvent.User.String() == tradeEvent.Creator.String() && !tradeEvent.IsBuy {
			assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				queue.SendTokenToSellChannel(0, assetInfo, assetInfo.Amount, currentPrice, constants.PumpVersionV1, "pumpfun", "DevSell", "ds清仓")
			}
		}
	}

	// 处理买入事件
	if tradeEvent.User.String() == botWallet.String() {
		if tradeEvent.IsBuy {
			// 设置入场价格
			global.SetTradeAsset(
				mintAddress,
				tradeEvent.Creator,
				botWallet.String(),
				currentPrice,
				tradeEvent.TokenAmount,
				6,
			)
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				assetManager := asset.GetHoldingsManager()
				wallets := assetManager.GetWalletsByMint(mintAddress)
				if len(wallets) == 0 {
					logger.Errorf("未找到代币 %s 的持仓", mintAddress)
					return nil
				}
				walletAddress := wallets[0]
				specialScheme := global.GetConfigByWallet(walletAddress)
				if specialScheme == nil {
					logger.Errorf("未找跟单钱包的配置参数模版 %s", walletAddress)
					return nil
				}
				message := types.ClosePositionMessage{
					MintAddress:  mintAddress,
					CurrentPrice: currentPrice,
					Platform:     "pumpfun",
					MessageType:  "delay_sell",
					Reason:       "延迟卖出",
					CopyAddress:  walletAddress,
				}
				go b.sdkClient.PublishDelayedCPMessage(b.Config.TradeChannel, message, specialScheme.SellDelayTime)

				// 启动盈利目标监控
				go b.StartProfitTargetMonitoring(mintAddress, "pumpfun", currentPrice,
					tradeEvent.TokenAmount, tradeEvent.Creator, botWallet)

				// 启动买单活跃度监控（仅pumpfun内盘）
				go b.StartBuyActivityMonitoring(mintAddress, "pumpfun", currentPrice,
					tradeEvent.TokenAmount, tradeEvent.Creator, botWallet)
			}
		} else {
			// 卖出记录处理
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				global.DelPendingHoldAssetWithAmount(mintAddress, tradeEvent.TokenAmount)
			}
		}
	}

	return tradeInfo
}

// handlePumpV2TradeEvent 处理 pumpV2 模块的 TradeEvent
func (b *Bot) handlePumpV2TradeEvent(tradeEvent *pumpV2.TradeEvent) *global.TradeEventInfo {
	// 获取基本信息
	botWallet := global.GetConfig().TradeWalletKey.PublicKey()
	mintAddress := tradeEvent.Mint.String()
	currentPrice := float64(tradeEvent.VirtualSolReserves/1e9) / float64(tradeEvent.VirtualTokenReserves/1e6)

	logger.Debugf("handlePumpV2TradeEvent: Mint=%s, User=%s, IsBuy=%v, Price=%.9f, SolAmount=%d, TokenAmount=%d",
		mintAddress, tradeEvent.User.String(), tradeEvent.IsBuy, currentPrice,
		tradeEvent.SolAmount, tradeEvent.TokenAmount)

	// 创建交易信息对象
	tradeInfo := &global.TradeEventInfo{
		PumpVersion:          constants.PumpVersionV2,
		MintAddress:          mintAddress,
		UserAddress:          tradeEvent.User.String(),
		FeeRecipient:         tradeEvent.FeeRecipient,
		SolAmount:            tradeEvent.SolAmount,
		TokenAmount:          tradeEvent.TokenAmount,
		IsBuy:                tradeEvent.IsBuy,
		VirtualSolReserves:   tradeEvent.VirtualSolReserves,
		VirtualTokenReserves: tradeEvent.VirtualTokenReserves,
		RealSolReserves:      tradeEvent.RealSolReserves,
		RealTokenReserves:    tradeEvent.RealTokenReserves,
		Creator:              tradeEvent.Creator,
		TimeStamp:            time.Now().Unix(),
		TradeTime:            time.Now().Format("2006-01-02 15:04:05.000"),
		Milliseconds:         time.Now().UnixMilli(),
		ReceivedAt:           time.Now(),
		CurrentPrice:         currentPrice,
	}

	// 存储到全局映射表中
	global.SetTradeInfo(mintAddress, tradeInfo)

	// ds清仓
	if global.GetConfig().DevSell {
		if tradeEvent.User.String() == tradeEvent.Creator.String() && !tradeEvent.IsBuy {
			assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				queue.SendTokenToSellChannel(0, assetInfo, assetInfo.Amount, currentPrice, constants.PumpVersionV2, "pumpfun", "DevSell", "ds清仓")
			}
		}
	}

	// 处理买入事件
	if tradeEvent.User.String() == botWallet.String() {
		if tradeEvent.IsBuy {
			// 设置入场价格
			global.SetTradeAsset(
				mintAddress,
				tradeEvent.Creator,
				botWallet.String(),
				currentPrice,
				tradeEvent.TokenAmount,
				6,
			)
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				assetManager := asset.GetHoldingsManager()
				wallets := assetManager.GetWalletsByMint(mintAddress)
				if len(wallets) == 0 {
					logger.Errorf("未找到代币 %s 的持仓", mintAddress)
					return nil
				}
				walletAddress := wallets[0]
				specialScheme := global.GetConfigByWallet(walletAddress)
				if specialScheme == nil {
					logger.Errorf("未找跟单钱包的配置参数模版 %s", walletAddress)
					return nil
				}
				message := types.ClosePositionMessage{
					MintAddress:  mintAddress,
					CurrentPrice: currentPrice,
					Platform:     "pumpfun",
					MessageType:  "delay_sell",
					Reason:       "延迟卖出",
					CopyAddress:  walletAddress,
				}
				go b.sdkClient.PublishDelayedCPMessage(b.Config.TradeChannel, message, specialScheme.SellDelayTime)

				// 启动盈利目标监控
				go b.StartProfitTargetMonitoring(mintAddress, "pumpfun", currentPrice,
					tradeEvent.TokenAmount, tradeEvent.Creator, botWallet)

				// 启动买单活跃度监控（仅pumpfun内盘）
				go b.StartBuyActivityMonitoring(mintAddress, "pumpfun", currentPrice,
					tradeEvent.TokenAmount, tradeEvent.Creator, botWallet)
			}
		} else {
			// 卖出记录处理
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				global.DelPendingHoldAssetWithAmount(mintAddress, tradeEvent.TokenAmount)
			}
		}
	}

	return tradeInfo
}

// handlePumpCreateEvent 处理 pump 模块的 CreateEvent
func (b *Bot) handlePumpCreateEvent(createEvent *pump.CreateEventEventData) {
	createInfo := &global.CreateEventInfo{
		MintAddress: createEvent.Mint.String(),
		Name:        createEvent.Name,
		CreatedAt:   time.Now(),
	}
	global.SetCreateInfo(createEvent.Mint.String(), createInfo)
	logger.Debugf("代币创建事件处理: 代币=%s, 创建时间=%v", createEvent.Mint.String(), createInfo.CreatedAt.Format("2006-01-02 15:04:05.000000"))
}

// handlePumpV2CreateEvent 处理 pumpV2 模块的 CreateEvent
func (b *Bot) handlePumpV2CreateEvent(createEvent *pumpV2.CreateEvent) {
	createInfo := &global.CreateEventInfo{
		MintAddress: createEvent.Mint.String(),
		Name:        createEvent.Name,
		CreatedAt:   time.Now(),
	}
	global.SetCreateInfo(createEvent.Mint.String(), createInfo)
	logger.Debugf("代币创建事件处理 (Pump V2 - Token2022): 代币=%s, 名称=%s, Symbol=%s, URI=%s, Creator=%s, 创建时间=%v",
		createEvent.Mint.String(),
		createEvent.Name,
		createEvent.Symbol,
		createEvent.Uri,
		createEvent.User.String(),
		createInfo.CreatedAt.Format("2006-01-02 15:04:05.000000"))
}

// 以下是原来的代码，已被上面的函数替换
func (b *Bot) PumpParse_OLD_BACKUP(out *yellowstone_geyser_pb.SubscribeUpdate, accountsAddress []string, ctx context.Context) *global.TradeEventInfo {
	if out.GetTransaction() == nil {
		return nil
	}

	logMessages := out.GetTransaction().Transaction.Meta.LogMessages
	// 检查是否为代币创建事件

	events, err := pump.DecodeEventsInLogs(logMessages)
	if err != nil {
		return nil
	}

	for _, event := range events {
		// 检查是否是交易事件
		if event.Name == "TradeEvent" {
			// 管道步骤1: 解析交易事件数据
			tradeEvent, ok := event.Data.(*pump.TradeEventEventData)
			if !ok {
				logger.Errorf("无法解析TradeEvent事件")
				continue
			}

			// 获取基本信息
			botWallet := global.GetConfig().TradeWalletKey.PublicKey()
			mintAddress := tradeEvent.Mint.String()
			currentPrice := float64(tradeEvent.VirtualSolReserves/1e9) / float64(tradeEvent.VirtualTokenReserves/1e6)

			// 管道步骤2: 创建交易信息对象
			tradeInfo := &global.TradeEventInfo{
				MintAddress:          mintAddress,
				UserAddress:          tradeEvent.User.String(),
				SolAmount:            tradeEvent.SolAmount,
				TokenAmount:          tradeEvent.TokenAmount,
				IsBuy:                tradeEvent.IsBuy,
				VirtualSolReserves:   tradeEvent.VirtualSolReserves,
				VirtualTokenReserves: tradeEvent.VirtualTokenReserves,
				RealSolReserves:      tradeEvent.RealSolReserves,
				RealTokenReserves:    tradeEvent.RealTokenReserves,
				Creator:              tradeEvent.Creator,
				TimeStamp:            time.Now().Unix(),
				TradeTime:            time.Now().Format("2006-01-02 15:04:05.000"),
				Milliseconds:         time.Now().UnixMilli(),
				ReceivedAt:           time.Now(),
				CurrentPrice:         currentPrice,
			}

			// 存储到全局映射表中
			global.SetTradeInfo(mintAddress, tradeInfo)
			//ds清仓
			if global.GetConfig().DevSell {
				if tradeEvent.User.String() == tradeEvent.Creator.String() && !tradeEvent.IsBuy {
					assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
					if ok {
						queue.SendTokenToSellChannel(0, assetInfo, assetInfo.Amount, currentPrice, constants.PumpVersionV1, "pumpfun", "DevSell", "ds清仓")

					}
				}
			}

			// 管道步骤3: 处理买入事件
			if tradeEvent.User.String() == botWallet.String() {
				if tradeEvent.IsBuy {

					// 设置入场价格
					global.SetTradeAsset(
						mintAddress,
						tradeEvent.Creator,
						botWallet.String(),
						currentPrice,
						tradeEvent.TokenAmount,
						6,
					)
					_, ok := global.GetPendingHoldAsset(mintAddress)
					if ok {
						assetManager := asset.GetHoldingsManager()
						wallets := assetManager.GetWalletsByMint(mintAddress)
						if len(wallets) == 0 {
							logger.Errorf("未找到代币 %s 的持仓", mintAddress)
							return nil
						}
						walletAddress := wallets[0]
						specialScheme := global.GetConfigByWallet(walletAddress)
						if specialScheme == nil {
							logger.Errorf("未找跟单钱包的配置参数模版 %s", walletAddress)
							return nil
						}
						message := types.ClosePositionMessage{
							MintAddress:  mintAddress,
							CurrentPrice: currentPrice,
							Platform:     "pumpfun",
							MessageType:  "delay_sell",
							Reason:       "延迟卖出",
							CopyAddress:  walletAddress,
						}
						go b.sdkClient.PublishDelayedCPMessage(b.Config.TradeChannel, message, specialScheme.SellDelayTime)

						// 启动盈利目标监控
						go b.StartProfitTargetMonitoring(mintAddress, "pumpfun", currentPrice,
							tradeEvent.TokenAmount, tradeEvent.Creator, botWallet)

						// 启动买单活跃度监控（仅pumpfun内盘）
						go b.StartBuyActivityMonitoring(mintAddress, "pumpfun", currentPrice,
							tradeEvent.TokenAmount, tradeEvent.Creator, botWallet)
					}

					//买入记录处理

				} else {
					//卖出记录处理
					_, ok := global.GetPendingHoldAsset(mintAddress)
					if ok {
						global.DelPendingHoldAssetWithAmount(mintAddress, tradeEvent.TokenAmount)
					}

				}
			}

			return tradeInfo
		} else if event.Name == "CreateEvent" {
			createEvent, ok := event.Data.(*pump.CreateEventEventData)
			if !ok {
				logger.Errorf("无法解析CreateEvent事件")
				continue
			}
			createInfo := &global.CreateEventInfo{
				MintAddress: createEvent.Mint.String(),
				Name:        createEvent.Name,
				CreatedAt:   time.Now(),
			}
			global.SetCreateInfo(createEvent.Mint.String(), createInfo)
			logger.Debugf("代币创建事件处理: 代币=%s, 创建时间=%v", createEvent.Mint.String(), createInfo.CreatedAt.Format("2006-01-02 15:04:05.000000"))

		}
	}

	return nil
}

func (b *Bot) PumpStartPricePing(priceStreamClient *goyser.StreamClient) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for range ticker.C {
		lastPingTime := time.Now()
		if err := priceStreamClient.SendCustomRequest(&yellowstone_geyser_pb.SubscribeRequest{Ping: &yellowstone_geyser_pb.SubscribeRequestPing{Id: 1}}); err != nil {
			logger.Errorf(" price ping : %v \n", err)
			return
		}
		logger.Infof(" price ping - :%v \n", lastPingTime.Format("15:04:05.000"))
	}
}
