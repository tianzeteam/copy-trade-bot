package bot

import (
	"context"
	"encoding/base64"
	"slices"
	"strings"
	"time"

	"copytrade/asset"
	"copytrade/constants"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/pump"
	pumpV2 "copytrade/pumpV2"
	pumpAmmV2 "copytrade/pumpammV2"
	"copytrade/queue"
	"copytrade/types"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	ag_solanago "github.com/gagliardetto/solana-go"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	_ "github.com/joho/godotenv/autoload"
	ag_base58 "github.com/mr-tron/base58"
)

// 全局AMM交易信息映射表已移至 global_maps.go

// parseAmmTransaction 解析AMM交易事件
func (b *Bot) parseUniversalRouterAmmTransaction(logMessages []string, mintAddress string) *global.AmmTradeEventInfo {
	// 使用pumpammV2包中的事件解析功能
	events, err := DecodePumpAmmV2EventsInLogs(logMessages)
	if err != nil {
		return nil
	}

	// 遍历解析出的事件
	for _, event := range events {
		// 检查是否是买入或卖出事件
		switch e := event.(type) {
		case *pumpAmmV2.BuyEvent:
			// 创建交易事件信息对象
			_baseReserve := e.PoolBaseTokenReserves
			_quoteReserve := e.PoolQuoteTokenReserves

			tradeInfo := &global.AmmTradeEventInfo{
				UserAddress:  e.User,
				SolAmount:    e.QuoteAmountIn,
				TokenAmount:  e.BaseAmountOut,
				IsBuy:        true,
				MintAddress:  solana.MustPublicKeyFromBase58(mintAddress), // 需要从其他地方获取
				Creator:      e.CoinCreator,
				AmmPool:      e.Pool,
				TimeStamp:    e.Timestamp,
				SolReserve:   _quoteReserve,
				BaseReserve:  _baseReserve,
				QuoteReserve: _quoteReserve,
				ReceivedAt:   time.Now(),
				CoinCreator:  e.CoinCreator,
			}

			// 计算当前价格 (SOL/TOKEN)
			// 使用大数计算以避免精度问题
			solDecimal := 1e9   // SOL有9个小数位
			tokenDecimal := 1e6 // Token有6个小数位

			// 转换为浮点数进行价格计算
			solReserveFloat := float64(_quoteReserve) / solDecimal
			tokenReserveFloat := float64(_baseReserve) / tokenDecimal

			// 计算当前价格 (SOL/TOKEN)
			tradeInfo.CurrentPrice = solReserveFloat / tokenReserveFloat

			// 格式化交易时间
			tradeInfo.TradeTime = time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")

			return tradeInfo

		case *pumpAmmV2.SellEvent:
			// 从卖出事件中提取信息
			tradeInfo := &global.AmmTradeEventInfo{
				UserAddress:  e.User,
				SolAmount:    e.QuoteAmountOut,
				TokenAmount:  e.BaseAmountIn,
				IsBuy:        false,
				MintAddress:  solana.MustPublicKeyFromBase58(mintAddress), // 需要从其他地方获取
				AmmPool:      e.Pool,
				TimeStamp:    e.Timestamp,
				SolReserve:   e.PoolQuoteTokenReserves,
				BaseReserve:  e.PoolBaseTokenReserves,  //token储备
				QuoteReserve: e.PoolQuoteTokenReserves, //sol储备
				ReceivedAt:   time.Now(),
				CoinCreator:  e.CoinCreator,
			}

			// 计算当前价格 (SOL/TOKEN)
			solDecimal := 1e9   // SOL有9个小数位
			tokenDecimal := 1e6 // Token有6个小数位

			// 转换为浮点数进行价格计算
			tokenReserveFloat := float64(e.PoolBaseTokenReserves) / tokenDecimal
			solReserveFloat := float64(e.PoolQuoteTokenReserves) / solDecimal

			// 计算当前价格 (SOL/TOKEN)
			tradeInfo.CurrentPrice = solReserveFloat / tokenReserveFloat

			// 格式化交易时间
			tradeInfo.TradeTime = time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")

			return tradeInfo
		}
	}

	return nil
}

const eventLogPrefix = "Program data: "

// decodePumpAmmV2EventsInLogs 从日志消息中解析 PumpAMM V2 事件
// 支持两种事件来源：
// 1. 日志消息 (Program data: 前缀)
// 2. Anchor Self CPI Log (通过 InnerInstructions)
func DecodePumpAmmV2EventsInLogs(logMessages []string) ([]any, error) {
	var events []any

	// 从日志消息中解析事件
	for _, log := range logMessages {
		if strings.HasPrefix(log, eventLogPrefix) {
			eventBase64 := log[len(eventLogPrefix):]
			eventBinary, err := base64.StdEncoding.DecodeString(eventBase64)
			if err != nil {
				logger.Debugf("failed to decode logMessage event: %s", eventBase64)
				continue
			}

			event, err := pumpAmmV2.ParseAnyEvent(eventBinary)
			if err != nil {
				// 忽略非事件数据或未知事件
				continue
			}
			events = append(events, event)
		}
	}

	return events, nil
}

// decodePumpAmmV2EventsWithCPI 从日志消息和 CPI 指令中解析 PumpAMM V2 事件
// 这是完整版本，同时支持日志和 CPI 两种事件来源
func decodePumpAmmV2EventsWithCPI(logMessages []string, innerIxs []ag_rpc.InnerInstruction, accountKeys ag_solanago.PublicKeySlice, targetProgramId ag_solanago.PublicKey) ([]any, error) {
	var eventBinaries [][]byte

	// 1. 从日志消息中提取事件
	for _, log := range logMessages {
		if strings.HasPrefix(log, eventLogPrefix) {
			eventBase64 := log[len(eventLogPrefix):]
			eventBinary, err := base64.StdEncoding.DecodeString(eventBase64)
			if err != nil {
				logger.Debugf("failed to decode logMessage event: %s", eventBase64)
				continue
			}
			eventBinaries = append(eventBinaries, eventBinary)
		}
	}

	// 2. 从 CPI 指令中提取事件 (Anchor Self CPI Log)
	for _, parsedIx := range innerIxs {
		for _, ix := range parsedIx.Instructions {
			// 检查是否是目标程序的指令
			if int(ix.ProgramIDIndex) >= len(accountKeys) {
				continue
			}
			if accountKeys[ix.ProgramIDIndex] != targetProgramId {
				continue
			}

			// 解码指令数据
			ixData, err := ag_base58.Decode(ix.Data.String())
			if err != nil {
				logger.Warnf("failed to decode CPI instruction data: %v", err)
				continue
			}

			// 跳过指令判别器（前8字节），剩余部分是事件数据
			if len(ixData) < 8 {
				continue
			}

			eventData := ixData[8:]
			if len(eventData) > 0 {
				eventBinaries = append(eventBinaries, eventData)
			}
		}
	}

	// 3. 解析所有事件
	var events []any
	for _, eventBinary := range eventBinaries {
		event, err := pumpAmmV2.ParseAnyEvent(eventBinary)
		if err != nil {
			// 忽略非事件数据或未知事件
			logger.Debugf("failed to parse event: %v", err)
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// PumpParse 使用pipeline模式处理交易事件
func (b *Bot) UniversalRouterPumpParse(out *yellowstone_geyser_pb.SubscribeUpdate, ctx context.Context, signature string) *global.TradeEventInfo {
	if out.GetTransaction() == nil {
		return nil
	}

	// 获取交易信息
	txn := out.GetTransaction()
	logMessages := txn.Transaction.Meta.LogMessages

	// 检查是否有日志信息
	if len(logMessages) == 0 {
		return nil
	}

	// 获取账户列表
	accountList := b.GetAccountList(out)

	// 优先使用 pumpV2 解析（支持 Token2022）
	// 准备 InnerInstructions
	innerInstructions := txn.Transaction.Meta.InnerInstructions
	var accountKeysPubkeys ag_solanago.PublicKeySlice
	for _, addrStr := range accountList {
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
			if int(ix.ProgramIdIndex) >= len(accountKeysPubkeys) {
				continue
			}
			accounts := make([]uint16, 0, len(ix.Accounts))
			for _, acc := range ix.Accounts {
				if int(acc) >= len(accountKeysPubkeys) {
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
		if len(instructions) > 0 {
			innerIxs = append(innerIxs, ag_rpc.InnerInstruction{
				Index:        uint16(innerIx.Index),
				Instructions: instructions,
			})
		}
	}

	if slices.Contains(accountList, solana.Token2022ProgramID.String()) {
		//logger.Infof("tx signature switch: %s", signature)
		// 尝试使用 pumpV2 解析
		eventsV2, errV2 := pumpV2.DecodeEventsInTransaction(logMessages, innerIxs, accountKeysPubkeys, core.PUMP_PROGRAM_ID, signature)
		if errV2 != nil {
			logger.Debugf("pumpV2 解析失败: %v", errV2)
			return nil
		} else if len(eventsV2) == 0 {
			logger.Debugf("pumpV2 解析成功但未找到事件")
			return nil
		}
		// 先处理 CreateEvent
		processedMints := make(map[string]bool) // 用于去重
		//var tradeEventV2 *pumpV2.TradeEvent

		for i, evt := range eventsV2 {
			logger.Debugf("[V2 Event #%d] 类型=%T", i, evt)

			if createEvent, ok := evt.(*pumpV2.CreateEvent); ok {
				mintAddress := createEvent.Mint.String()
				// 检查是否已经处理过该代币
				if processedMints[mintAddress] {
					logger.Debugf("跳过重复的 CreateEvent: 代币=%s", mintAddress)
					continue
				}
				processedMints[mintAddress] = true

				createInfo := &global.CreateEventInfo{
					MintAddress: mintAddress,
					Name:        createEvent.Name,
					CreatedAt:   time.Now(),
				}
				global.SetCreateInfo(mintAddress, createInfo)
				logger.Debugf("代币创建事件处理 (Pump V2 - Token2022): 代币=%s, 名称=%s, Symbol=%s, URI=%s, Creator=%s, 创建时间=%v",
					mintAddress,
					createEvent.Name,
					createEvent.Symbol,
					createEvent.Uri,
					createEvent.User.String(),
					createInfo.CreatedAt.Format("2006-01-02 15:04:05.000000"))
			}

			// 查找 TradeEvent
			if tradeEventV2, ok := evt.(*pumpV2.TradeEvent); ok {
				logger.Debugf("[V2 TradeEvent] User=%s, Mint=%s, IsBuy=%v, SolAmount=%d",
					tradeEventV2.User.String(), tradeEventV2.Mint.String(),
					tradeEventV2.IsBuy, tradeEventV2.SolAmount)
				return b.handlePumpV2TradeEvent(tradeEventV2)
			}
		}

		// pumpV2 解析成功但没有 TradeEvent，返回 nil
		return nil
	} else {
		// 降级到 pump V1 模块，使用包含 CPI 数据的解析方法
		events, err := b.decodePumpV1EventsWithCPI(logMessages, innerIxs, accountKeysPubkeys)
		if err != nil {
			logger.Errorf("pump V1 解析事件失败: %v", err)
			return nil
		}

		// 处理 Pump V1 的 CreateEvent 和 TradeEvent
		for _, event := range events {
			if event.Name == "CreateEvent" {
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
				logger.Debugf("代币创建事件处理 (Pump V1): 代币=%s, 名称=%s, Symbol=%s, URI=%s, 创建时间=%v",
					createEvent.Mint.String(),
					createEvent.Name,
					createEvent.Symbol,
					createEvent.Uri,
					createInfo.CreatedAt.Format("2006-01-02 15:04:05.000000"))
			}
		}

		// 处理 Pump V1 TradeEvent
		for _, event := range events {
			if event.Name == "TradeEvent" {
				tradeEvent, ok := event.Data.(*pump.TradeEventEventData)
				if !ok {
					logger.Errorf("无法解析TradeEvent事件")
					continue
				}
				return b.handlePumpTradeEvent(tradeEvent)
			}
		}
		return nil
	}
}

// decodePumpV1EventsWithCPI 解析 Pump V1 事件（包含 CPI 数据）
func (b *Bot) decodePumpV1EventsWithCPI(logMessages []string, innerIxs []ag_rpc.InnerInstruction, accountKeys ag_solanago.PublicKeySlice) ([]*pump.Event, error) {
	// 1. 从日志消息中解析事件
	eventsFromLogs, err := pump.DecodeEventsInLogs(logMessages)
	if err != nil {
		return nil, err
	}

	// 2. 从 CPI 指令中解析事件
	eventsFromCPI, err := b.decodePumpV1EventsFromCPI(innerIxs, accountKeys)
	if err != nil {
		// CPI 解析失败不影响日志事件，只记录警告
		logger.Warnf("pump V1 CPI 事件解析失败: %v", err)
		return eventsFromLogs, nil
	}

	// 3. 合并两种来源的事件
	allEvents := append(eventsFromLogs, eventsFromCPI...)
	logger.Debugf("pump V1 解析完成: 日志事件=%d, CPI事件=%d, 总计=%d",
		len(eventsFromLogs), len(eventsFromCPI), len(allEvents))

	return allEvents, nil
}

// decodePumpV1EventsFromCPI 从 CPI 指令中解析 Pump V1 事件
func (b *Bot) decodePumpV1EventsFromCPI(innerIxs []ag_rpc.InnerInstruction, accountKeys ag_solanago.PublicKeySlice) ([]*pump.Event, error) {
	// 收集所有 CPI 事件的二进制数据
	var eventBinaries [][]byte

	for _, parsedIx := range innerIxs {
		for _, ix := range parsedIx.Instructions {
			// 检查是否是 PUMP_PROGRAM_ID
			if int(ix.ProgramIDIndex) >= len(accountKeys) {
				continue
			}
			if accountKeys[ix.ProgramIDIndex] != core.PUMP_PROGRAM_ID {
				continue
			}

			// 解码 Base58 数据
			ixData, err := ag_base58.Decode(ix.Data.String())
			if err != nil {
				logger.Warnf("解析 CPI 指令数据失败: %v", err)
				continue
			}

			// 跳过指令判别器（前8字节）
			if len(ixData) < 8 {
				continue
			}

			eventData := ixData[8:]
			if len(eventData) == 0 {
				continue
			}

			// 添加到事件二进制数据列表
			eventBinaries = append(eventBinaries, eventData)
		}
	}

	// 使用 pump 模块的 parseEvents 方法解析事件
	// 注意：parseEvents 是私有方法，我们需要手动解析
	var events []*pump.Event
	decoder := ag_binary.NewDecoderWithEncoding(nil, ag_binary.EncodingBorsh)

	for _, eventBinary := range eventBinaries {
		if len(eventBinary) < 8 {
			continue
		}

		// 尝试解析为 TradeEvent
		tradeEvent := &pump.TradeEventEventData{}
		decoder.Reset(eventBinary)
		if err := tradeEvent.UnmarshalWithDecoder(decoder); err == nil {
			events = append(events, &pump.Event{
				Name: "TradeEvent",
				Data: tradeEvent,
			})
			continue
		}

		// 尝试解析为 CreateEvent
		createEvent := &pump.CreateEventEventData{}
		decoder.Reset(eventBinary)
		if err := createEvent.UnmarshalWithDecoder(decoder); err == nil {
			events = append(events, &pump.Event{
				Name: "CreateEvent",
				Data: createEvent,
			})
		}
	}

	return events, nil
}

// UniversalRouterAmmParse 解析通过 Universal Router 调用的 PumpAMM 交易日志
// 参考 UniversalRouterPumpParse 的泛化处理逻辑，直接从事件中提取所有信息
func (b *Bot) UniversalRouterAmmParse(out *yellowstone_geyser_pb.SubscribeUpdate, currentInstruction *yellowstone_geyser_pb.InnerInstruction, ctx context.Context) *global.AmmTradeEventInfo {
	if out.GetTransaction() == nil {
		return nil
	}

	// 获取交易信息
	txn := out.GetTransaction()
	logMessages := txn.Transaction.Meta.LogMessages

	// 检查是否有日志信息
	if len(logMessages) == 0 {
		return nil
	}

	// 步骤1: 从指令账户中提取 mintAddress（因为事件中没有 Mint 字段）
	var mintAddress string
	data := currentInstruction.GetData()
	if len(data) >= 8 {
		var discriminator [8]byte
		copy(discriminator[:], data[:8])

		// 根据指令类型提取 mintAddress
		switch discriminator {
		case pumpAmmV2.Instruction_Buy, pumpAmmV2.Instruction_BuyExactQuoteIn, pumpAmmV2.Instruction_Sell:
			accountList := b.GetAccountList(out)
			if len(currentInstruction.Accounts) > 3 {
				mintAddress = accountList[currentInstruction.Accounts[3]]
				// 过滤 SOL mint
				if mintAddress == solana.SolMint.String() {
					return nil
				}
			}
		default:
			// 不是交易指令，忽略
			return nil
		}
	}

	if mintAddress == "" {
		return nil
	}

	// 步骤2: 准备 InnerInstructions 和 AccountKeys（用于 CPI 事件解析）
	accountList := b.GetAccountList(out)
	var accountKeysPubkeys ag_solanago.PublicKeySlice
	for _, addrStr := range accountList {
		pubkey, err := ag_solanago.PublicKeyFromBase58(addrStr)
		if err != nil {
			logger.Warnf("无效的账户地址: %s, 错误: %v", addrStr, err)
			continue
		}
		accountKeysPubkeys = append(accountKeysPubkeys, pubkey)
	}

	// 转换 InnerInstructions 为 solana-go 类型
	innerInstructions := txn.Transaction.Meta.InnerInstructions
	var innerIxs []ag_rpc.InnerInstruction
	for _, innerIx := range innerInstructions {
		var instructions []ag_solanago.CompiledInstruction
		for _, ix := range innerIx.Instructions {
			if int(ix.ProgramIdIndex) >= len(accountKeysPubkeys) {
				continue
			}
			accounts := make([]uint16, 0, len(ix.Accounts))
			for _, acc := range ix.Accounts {
				if int(acc) >= len(accountKeysPubkeys) {
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
		if len(instructions) > 0 {
			innerIxs = append(innerIxs, ag_rpc.InnerInstruction{
				Index:        uint16(innerIx.Index),
				Instructions: instructions,
			})
		}
	}

	// 步骤3: 从日志和 CPI 中解析 AMM 事件（支持 Anchor Self CPI Log）
	events, err := decodePumpAmmV2EventsWithCPI(logMessages, innerIxs, accountKeysPubkeys, pumpAmmV2.ProgramID)
	if err != nil {
		logger.Debugf("解析 PumpAMM V2 事件失败: %v", err)
		return nil
	}

	if len(events) == 0 {
		return nil
	}

	// 步骤3: 遍历事件，构建 tradeInfo（使用从指令中提取的 mintAddress）
	var tradeInfo *global.AmmTradeEventInfo
	for _, event := range events {
		switch e := event.(type) {
		case *pumpAmmV2.BuyEvent:
			_baseReserve := e.PoolBaseTokenReserves
			_quoteReserve := e.PoolQuoteTokenReserves

			tradeInfo = &global.AmmTradeEventInfo{
				UserAddress:  e.User,
				SolAmount:    e.QuoteAmountIn,
				TokenAmount:  e.BaseAmountOut,
				IsBuy:        true,
				MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
				Creator:      e.CoinCreator,
				AmmPool:      e.Pool,
				TimeStamp:    e.Timestamp,
				SolReserve:   _quoteReserve,
				BaseReserve:  _baseReserve,
				QuoteReserve: _quoteReserve,
				ReceivedAt:   time.Now(),
				CoinCreator:  e.CoinCreator,
				Router:       "externalRouter",
			}

			// 计算当前价格 (SOL/TOKEN)
			solDecimal := 1e9   // SOL有9个小数位
			tokenDecimal := 1e6 // Token有6个小数位

			solReserveFloat := float64(_quoteReserve) / solDecimal
			tokenReserveFloat := float64(_baseReserve) / tokenDecimal
			tradeInfo.CurrentPrice = solReserveFloat / tokenReserveFloat
			tradeInfo.TradeTime = time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")

		case *pumpAmmV2.SellEvent:
			tradeInfo = &global.AmmTradeEventInfo{
				UserAddress:  e.User,
				SolAmount:    e.QuoteAmountOut,
				TokenAmount:  e.BaseAmountIn,
				IsBuy:        false,
				MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
				AmmPool:      e.Pool,
				TimeStamp:    e.Timestamp,
				SolReserve:   e.PoolQuoteTokenReserves,
				BaseReserve:  e.PoolBaseTokenReserves,
				QuoteReserve: e.PoolQuoteTokenReserves,
				ReceivedAt:   time.Now(),
				CoinCreator:  e.CoinCreator,
				Router:       "externalRouter",
			}

			// 计算当前价格 (SOL/TOKEN)
			solDecimal := 1e9
			tokenDecimal := 1e6
			tokenReserveFloat := float64(e.PoolBaseTokenReserves) / tokenDecimal
			solReserveFloat := float64(e.PoolQuoteTokenReserves) / solDecimal
			tradeInfo.CurrentPrice = solReserveFloat / tokenReserveFloat
			tradeInfo.TradeTime = time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05")

		}
	}

	// 如果没有找到交易事件，返回 nil
	if tradeInfo == nil {
		return nil
	}

	// 存储到全局映射表
	global.SetAmmTradeInfo(mintAddress, tradeInfo)

	// 处理机器人自己的交易
	botWallet := global.GetConfig().TradeWalletKey.PublicKey()
	userPubkey := tradeInfo.UserAddress
	if userPubkey.String() == botWallet.String() {
		if tradeInfo.IsBuy {
			// 检查是否是自己的交易

			// 安全地计算入场价格
			var entryPrice float64
			if tradeInfo.TokenAmount > 0 {
				solAmountFloat := float64(tradeInfo.SolAmount) / SolDecimals
				tokenAmountFloat := float64(tradeInfo.TokenAmount) / TokenDecimals
				entryPrice = solAmountFloat / tokenAmountFloat
			} else {
				logger.Warnf("计算入场价格时发现代币数量为零: %d", tradeInfo.TokenAmount)
				entryPrice = 0
			}

			logger.Infof("买入交易: SOL=%d (%.4f), 代币=%d (%.2f), 价格=%.8f",
				tradeInfo.SolAmount, float64(tradeInfo.SolAmount)/SolDecimals,
				tradeInfo.TokenAmount, float64(tradeInfo.TokenAmount)/TokenDecimals,
				entryPrice)
			// 设置入场价格
			global.SetTradeAsset(
				mintAddress,
				tradeInfo.Creator,
				botWallet.String(),
				entryPrice,
				tradeInfo.TokenAmount,
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
					CurrentPrice: tradeInfo.CurrentPrice,
					Platform:     "pumpamm",
					MessageType:  "delay_sell",
					Reason:       "延迟卖出",
					CopyAddress:  walletAddress,
				}
				go b.sdkClient.PublishDelayedCPMessage(b.Config.TradeChannel, message, specialScheme.SellDelayTime)
			}
		} else {
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				global.DelPendingHoldAssetWithAmount(mintAddress, tradeInfo.TokenAmount)
			}
		}
	}
	tradeInfo.Router = "universalRouter"
	return tradeInfo
}

// PumpAmmSell 处理 PumpAMM 平台的代币卖出操作
func (b *Bot) PhotonRouterSell(tokenInfo queue.TokenInfo) {
	// 使用策略模式进行卖出
	logger.Infof("使用策略模式卖出代币: %s", tokenInfo.Mint.String())

	// 确保卖出策略已初始化
	if b.sellStrategy == nil {
		b.sellStrategy = NewMultiLevelSellStrategy(b.Config.MultiSellLevels)
		logger.Infof("初始化多级卖出策略: %s", b.sellStrategy.Name())
	}

	// 执行卖出策略
	b.ExecuteAmmSellStrategy(tokenInfo)
}

// handleUniversalRouterPumpV2TradeEvent 处理通过 Universal Router 调用的 pumpV2 TradeEvent
func (b *Bot) handleUniversalRouterPumpV2TradeEvent(tradeEvent *pumpV2.TradeEvent, mintAddress string) *global.TradeEventInfo {
	// 获取基本信息
	botWallet := global.GetConfig().TradeWalletKey.PublicKey()
	currentPrice := float64(tradeEvent.VirtualSolReserves/1e9) / float64(tradeEvent.VirtualTokenReserves/1e6)

	logger.Infof("handleUniversalRouterPumpV2TradeEvent: Mint=%s, User=%s, IsBuy=%v, Price=%.9f, SolAmount=%d, TokenAmount=%d",
		mintAddress, tradeEvent.User.String(), tradeEvent.IsBuy, currentPrice,
		tradeEvent.SolAmount, tradeEvent.TokenAmount)

	// 创建交易信息对象
	tradeInfo := &global.TradeEventInfo{
		PumpVersion:          constants.PumpVersionV2,
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
