package bot

import (
	"context"
	"time"

	"copytrade/asset"
	"copytrade/constants"
	"copytrade/global"
	"copytrade/logger"
	pumpV3 "copytrade/pumpV3"
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

// PumpParse 解析泵内盘交易事件。
// 新旧六种买卖指令共用同一个 TradeEvent 判别码，统一走 pumpV3 解码；
// accountsAddress 中是否出现 Token2022 程序仅用于标记 base 代币程序归属。
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

	// 检查是否包含 Token2022 程序地址（决定 base 代币程序归属）
	token2022ProgramID := solana.Token2022ProgramID.String()
	isToken2022 := false
	for _, addr := range accountsAddress {
		if addr == token2022ProgramID {
			isToken2022 = true
			break
		}
	}

	// 统一事件解码：日志与 Self CPI 两个来源都覆盖，未知事件自动跳过。
	events, err := pumpV3.DecodeEventsInTransaction(logMessages, innerIxs, accountKeysPubkeys, pumpV3.ProgramID)
	if err != nil {
		logger.Errorf("解析泵事件失败: %v", err)
		return nil
	}

	var tradeInfo *global.TradeEventInfo
	for _, evt := range events {
		switch e := evt.(type) {
		case *pumpV3.TradeEvent:
			if info := b.handlePumpV3TradeEvent(e, isToken2022); info != nil {
				tradeInfo = info
			}
		case *pumpV3.CreateEvent:
			b.handlePumpV3CreateEvent(e)
		}
	}
	return tradeInfo
}

// handlePumpV3TradeEvent 处理统一 TradeEvent。
// buy / sell / buy_exact_sol_in / buy_v2 / sell_v2 / buy_exact_quote_in_v2
// 六种指令共用该事件，旧指令事件缺失的尾部字段按零值解码。
func (b *Bot) handlePumpV3TradeEvent(tradeEvent *pumpV3.TradeEvent, isToken2022 bool) *global.TradeEventInfo {
	b.cachePumpTradeEvent(tradeEvent, isToken2022)
	botWallet := global.GetConfig().TradeWalletKey.PublicKey()
	mintAddress := tradeEvent.Mint.String()
	currentPrice := tradeEvent.CurrentPrice()

	version := constants.PumpVersionV1
	if isToken2022 {
		version = constants.PumpVersionV2
	}

	logger.Debugf("handlePumpV3TradeEvent: Mint=%s, User=%s, IsBuy=%v, Price=%.9f, QuoteAmount=%d, TokenAmount=%d, IxName=%s",
		mintAddress, tradeEvent.User.String(), tradeEvent.IsBuy, currentPrice,
		tradeEvent.EffectiveQuoteAmount(), tradeEvent.TokenAmount, tradeEvent.IxName)

	tradeInfo := &global.TradeEventInfo{
		PumpVersion:          version,
		MintAddress:          mintAddress,
		UserAddress:          tradeEvent.User.String(),
		FeeRecipient:         tradeEvent.FeeRecipient,
		SolAmount:            tradeEvent.EffectiveQuoteAmount(),
		TokenAmount:          tradeEvent.TokenAmount,
		IsBuy:                tradeEvent.IsBuy,
		VirtualSolReserves:   tradeEvent.EffectiveVirtualQuoteReserves(),
		VirtualTokenReserves: tradeEvent.VirtualTokenReserves,
		RealSolReserves:      tradeEvent.EffectiveRealQuoteReserves(),
		RealTokenReserves:    tradeEvent.RealTokenReserves,
		Creator:              tradeEvent.Creator,
		QuoteMint:            tradeEvent.EffectiveQuoteMint(),
		MayhemMode:           tradeEvent.MayhemMode,
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
				queue.SendTokenToSellChannel(0, assetInfo, assetInfo.Amount, currentPrice, version, "pumpfun", "DevSell", "ds清仓")
			}
		}
	}

	// 处理自己的交易事件
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

// handlePumpV3CreateEvent 处理代币创建事件（create / create_v2 共用）。
func (b *Bot) handlePumpV3CreateEvent(createEvent *pumpV3.CreateEvent) {
	b.cachePumpCreateEvent(createEvent)
	createInfo := &global.CreateEventInfo{
		MintAddress: createEvent.Mint.String(),
		Name:        createEvent.Name,
		CreatedAt:   time.Now(),
	}
	global.SetCreateInfo(createEvent.Mint.String(), createInfo)
	logger.Debugf("代币创建事件处理: 代币=%s, 名称=%s, Symbol=%s, URI=%s, Creator=%s, 创建时间=%v",
		createEvent.Mint.String(),
		createEvent.Name,
		createEvent.Symbol,
		createEvent.URI,
		createEvent.User.String(),
		createInfo.CreatedAt.Format("2006-01-02 15:04:05.000000"))
}

func (b *Bot) cachePumpTradeEvent(tradeEvent *pumpV3.TradeEvent, isToken2022 bool) {
	bondingCurve, err := pumpV3.BondingCurvePDA(tradeEvent.Mint)
	if err != nil {
		logger.Warnf("缓存 Pump 曲线失败: mint=%s, err=%v", tradeEvent.Mint, err)
		return
	}

	global.UpdatePumpCurveSnapshot(tradeEvent.Mint, func(snapshot *global.PumpCurveSnapshot) {
		snapshot.BondingCurve = bondingCurve
		if !tradeEvent.Creator.IsZero() {
			snapshot.Creator = tradeEvent.Creator
		}
		if quoteMint := tradeEvent.EffectiveQuoteMint(); !quoteMint.IsZero() {
			snapshot.QuoteMint = quoteMint
		}
		if isToken2022 {
			snapshot.BaseTokenProgram = solana.Token2022ProgramID
		} else if snapshot.BaseTokenProgram.IsZero() {
			snapshot.BaseTokenProgram = solana.TokenProgramID
		}
		snapshot.VirtualTokenReserves = tradeEvent.VirtualTokenReserves
		snapshot.VirtualQuoteReserves = tradeEvent.EffectiveVirtualQuoteReserves()
		snapshot.RealTokenReserves = tradeEvent.RealTokenReserves
		snapshot.RealQuoteReserves = tradeEvent.EffectiveRealQuoteReserves()
		snapshot.IsMayhemMode = tradeEvent.MayhemMode
		if !tradeEvent.MayhemMode && snapshot.TokenTotalSupply == 0 {
			snapshot.TokenTotalSupply = pumpV3.OneBillionSupply
		}
	})

	snapshot, _ := global.GetPumpCurveSnapshot(tradeEvent.Mint)
	if snapshot.Creator.IsZero() || (tradeEvent.MayhemMode && snapshot.TokenTotalSupply == 0) {
		b.prefetchPumpCurve(tradeEvent.Mint, bondingCurve)
	}
}

func (b *Bot) cachePumpCreateEvent(createEvent *pumpV3.CreateEvent) {
	baseTokenProgram := createEvent.TokenProgram
	if baseTokenProgram.IsZero() {
		baseTokenProgram = solana.TokenProgramID
	}
	tokenTotalSupply := createEvent.TokenTotalSupply
	if tokenTotalSupply == 0 && !createEvent.IsMayhemMode {
		tokenTotalSupply = pumpV3.OneBillionSupply
	}

	global.UpdatePumpCurveSnapshot(createEvent.Mint, func(snapshot *global.PumpCurveSnapshot) {
		snapshot.BondingCurve = createEvent.BondingCurve
		snapshot.Creator = createEvent.Creator
		snapshot.QuoteMint = createEvent.QuoteMint
		snapshot.BaseTokenProgram = baseTokenProgram
		snapshot.VirtualTokenReserves = createEvent.VirtualTokenReserves
		snapshot.VirtualQuoteReserves = createEvent.VirtualQuoteReserves
		snapshot.RealTokenReserves = createEvent.RealTokenReserves
		snapshot.RealQuoteReserves = 0
		snapshot.TokenTotalSupply = tokenTotalSupply
		snapshot.CreatorFeeBps = createEvent.CreatorFeeBps
		snapshot.IsMayhemMode = createEvent.IsMayhemMode
		snapshot.IsHolderReward = createEvent.IsHolderReward
		snapshot.IsCashbackCoin = createEvent.IsCashbackEnabled
	})
}

func (b *Bot) prefetchPumpCurve(mint, bondingCurve solana.PublicKey) {
	if b == nil || b.Trader == nil || b.Trader.AccountsCalc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	curve, err := b.Trader.AccountsCalc.FetchPumpBondingCurve(ctx, bondingCurve)
	if err != nil {
		logger.Warnf("预取 Pump 曲线失败: mint=%s, err=%v", mint, err)
		return
	}
	global.UpdatePumpCurveSnapshot(mint, func(snapshot *global.PumpCurveSnapshot) {
		if !curve.Creator.IsZero() {
			snapshot.Creator = curve.Creator
		}
		if !curve.QuoteMint.IsZero() {
			snapshot.QuoteMint = curve.QuoteMint
		}
		if curve.TokenTotalSupply != 0 {
			snapshot.TokenTotalSupply = curve.TokenTotalSupply
		}
		if curve.CreatorFeeBps != 0 {
			snapshot.CreatorFeeBps = curve.CreatorFeeBps
		}
		snapshot.IsMayhemMode = curve.IsMayhemMode
		snapshot.IsHolderReward = curve.IsHolderReward
		snapshot.IsCashbackCoin = curve.IsCashbackCoin
	})
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
