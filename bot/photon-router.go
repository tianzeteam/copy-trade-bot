package bot

import (
	"context"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/pump"
	"copytrade/queue"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"

	"copytrade/pumpamm"

	"github.com/mr-tron/base58"
)

// 全局AMM交易信息映射表已移至 global_maps.go

// parseAmmTransaction 解析AMM交易事件
func (b *Bot) parsePhotonRouterAmmTransaction(logMessages []string, mintAddress string) *global.AmmTradeEventInfo {
	// 使用pumpamm包中的事件解析功能
	events, err := pumpamm.DecodeEventsInLogs(logMessages)
	if err != nil {
		logger.Errorf("解析AMM事件失败: %v", err)
		return nil
	}

	// 遍历解析出的事件
	for _, event := range events {
		// 检查是否是买入或卖出事件
		switch event.Name {
		case "BuyEvent":
			if buyEvent, ok := event.Data.(*pumpamm.BuyEventEventData); ok {

				// 创建交易事件信息对象
				_baseReserve := buyEvent.PoolBaseTokenReserves
				_quoteReserve := buyEvent.PoolQuoteTokenReserves

				tradeInfo := &global.AmmTradeEventInfo{
					UserAddress:  buyEvent.User,
					SolAmount:    buyEvent.QuoteAmountIn,
					TokenAmount:  buyEvent.BaseAmountOut,
					IsBuy:        true,
					MintAddress:  solana.MustPublicKeyFromBase58(mintAddress), // 需要从其他地方获取
					Creator:      buyEvent.CoinCreator,
					AmmPool:      buyEvent.Pool,
					TimeStamp:    buyEvent.Timestamp,
					SolReserve:   _quoteReserve,
					BaseReserve:  _baseReserve,
					QuoteReserve: _quoteReserve,
					ReceivedAt:   time.Now(),
					CoinCreator:  buyEvent.CoinCreator,
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
				tradeInfo.TradeTime = time.Unix(buyEvent.Timestamp, 0).Format("2006-01-02 15:04:05")

				// 查找对应的池信息以获取Mint地址

				logger.Debugf("解析到AMM买入事件: 用户=%s, 代币=%s, 数量=%d, 价格=%.8f,sol储备=%f,token储备=%f",
					tradeInfo.UserAddress, tradeInfo.MintAddress, tradeInfo.TokenAmount, tradeInfo.CurrentPrice, solReserveFloat, tokenReserveFloat)

				return tradeInfo
			}

		case "SellEvent":
			if sellEvent, ok := event.Data.(*pumpamm.SellEventEventData); ok {
				// 从卖出事件中提取信息
				tradeInfo := &global.AmmTradeEventInfo{
					UserAddress:  sellEvent.User,
					SolAmount:    sellEvent.QuoteAmountOut,
					TokenAmount:  sellEvent.BaseAmountIn,
					IsBuy:        false,
					MintAddress:  solana.MustPublicKeyFromBase58(mintAddress), // 需要从其他地方获取
					AmmPool:      sellEvent.Pool,
					TimeStamp:    sellEvent.Timestamp,
					SolReserve:   sellEvent.PoolQuoteTokenReserves,
					BaseReserve:  sellEvent.PoolBaseTokenReserves,  //token储备
					QuoteReserve: sellEvent.PoolQuoteTokenReserves, //sol储备
					ReceivedAt:   time.Now(),
					CoinCreator:  sellEvent.CoinCreator,
				}

				// 计算当前价格 (SOL/TOKEN)
				solDecimal := 1e9   // SOL有9个小数位
				tokenDecimal := 1e6 // Token有6个小数位

				// 转换为浮点数进行价格计算
				tokenReserveFloat := float64(sellEvent.PoolBaseTokenReserves) / tokenDecimal
				solReserveFloat := float64(sellEvent.PoolQuoteTokenReserves) / solDecimal

				// 计算当前价格 (SOL/TOKEN)
				tradeInfo.CurrentPrice = solReserveFloat / tokenReserveFloat

				// 格式化交易时间
				tradeInfo.TradeTime = time.Unix(sellEvent.Timestamp, 0).Format("2006-01-02 15:04:05")

				logger.Debugf("解析到AMM卖出事件: 用户=%s, 代币=%s, 数量=%d, 价格=%.8f,sol储备=%f,token储备=%f",
					tradeInfo.UserAddress, tradeInfo.MintAddress, tradeInfo.TokenAmount, tradeInfo.CurrentPrice, solReserveFloat, tokenReserveFloat)

				return tradeInfo
			}
		}
	}

	return nil
}

// PumpParse 使用pipeline模式处理交易事件
func (b *Bot) PhotonRouterPumpParse(out *yellowstone_geyser_pb.SubscribeUpdate, currentInstruction *yellowstone_geyser_pb.InnerInstruction, ctx context.Context) *global.TradeEventInfo {
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

	_signature := base58.Encode(txn.Transaction.Transaction.Signatures[0])

	var mintAddress string

	// 安全检查：确保数据长度足够

	data := currentInstruction.GetData()
	if len(data) < 8 {
		logger.Debugf("指令数据不足8字节，跳过 %s", _signature)
		return nil
	}

	// 安全地获取判别器
	var discriminator [8]byte
	copy(discriminator[:], data[:8])

	// 比较判别器
	switch discriminator {
	case pumpamm.Instruction_Buy:

		accountList := b.GetAccountList(out)
		//user := accountList[currentInstruction.Accounts[1]]
		mintAddress = accountList[currentInstruction.Accounts[3]]

		if mintAddress == solana.SolMint.String() {
			logger.Debugf("跳过 SOL 交易 %s", _signature)
			return nil
		}

		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------买入------------判别器")

		logger.Debugf("_signature: %s", _signature)
		logger.Debugf("-------------------------------------------")
	case pumpamm.Instruction_Sell:
		accountList := b.GetAccountList(out)
		//user := accountList[currentInstruction.Accounts[1]]
		mintAddress = accountList[currentInstruction.Accounts[3]]
		if mintAddress == solana.SolMint.String() {
			logger.Debugf("跳过 SOL 交易 %s", _signature)
			return nil
		}
		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------卖出------------判别器")

		logger.Debugf("_signature: %s", _signature)
		logger.Debugf("-------------------------------------------")
	case pumpamm.CreateDiscriminator:
		logger.Debug("匹配 Mint 判别器")
	default:
		logger.Debug("未匹配判别器")
	}

	// 检查是否为代币创建事件
	events, err := pump.DecodeEventsInLogs(logMessages)
	if err != nil {
		logger.Errorf("解析事件失败: %v", err)
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
			botWallet := b.privateKey.PublicKey()
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

			// 管道步骤3: 处理买入事件
			if tradeEvent.User.String() == botWallet.String() {
				if tradeEvent.IsBuy {

					entryPrice := (float64(tradeEvent.VirtualSolReserves) / SolDecimals) / (float64(tradeEvent.VirtualTokenReserves) / TokenDecimals)

					// 设置入场价格
					global.SetTradeAssetOnce(
						mintAddress,
						tradeEvent.Creator,
						botWallet.String(),
						entryPrice,
						tradeEvent.TokenAmount,
						6,
					)
					assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
					if ok {
						go queue.SendTokenToSellChannel(global.GetConfig().SellDelayTime, assetInfo, entryPrice, "pumpfun", "delay_sell", "延迟卖出")
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
		}
	}

	return nil
}

// PumpAmmParse 解析PumpAMM交易日志
func (b *Bot) PhotonRouterAmmParse(out *yellowstone_geyser_pb.SubscribeUpdate, currentInstruction *yellowstone_geyser_pb.InnerInstruction, ctx context.Context) *global.AmmTradeEventInfo {
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

	_signature := base58.Encode(txn.Transaction.Transaction.Signatures[0])

	var mintAddress string

	// 安全检查：确保数据长度足够

	data := currentInstruction.GetData()
	if len(data) < 8 {
		logger.Debugf("指令数据不足8字节，跳过 %s", _signature)
		return nil
	}

	// 安全地获取判别器
	var discriminator [8]byte
	copy(discriminator[:], data[:8])

	// 比较判别器
	switch discriminator {
	case pumpamm.Instruction_Buy:

		accountList := b.GetAccountList(out)
		//user := accountList[currentInstruction.Accounts[1]]
		mintAddress = accountList[currentInstruction.Accounts[3]]

		if mintAddress == solana.SolMint.String() {
			logger.Debugf("跳过 SOL 交易 %s", _signature)
			return nil
		}

		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------买入------------判别器")

		logger.Debugf("_signature: %s", _signature)
		logger.Debugf("-------------------------------------------")
	case pumpamm.Instruction_Sell:
		accountList := b.GetAccountList(out)
		//user := accountList[currentInstruction.Accounts[1]]
		mintAddress = accountList[currentInstruction.Accounts[3]]
		if mintAddress == solana.SolMint.String() {
			logger.Debugf("跳过 SOL 交易 %s", _signature)
			return nil
		}
		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------卖出------------判别器")

		logger.Debugf("_signature: %s", _signature)
		logger.Debugf("-------------------------------------------")
	case pumpamm.CreateDiscriminator:
		logger.Debug("匹配 Mint 判别器")
	default:
		logger.Debug("未匹配判别器")
	}

	if mintAddress == "" {
		logger.Infof("未找到代币地址，跳过 %s", _signature)
		return nil
	}

	// 处理AMM交易信息
	tradeInfo := b.parsePhotonRouterAmmTransaction(logMessages, mintAddress)
	botWallet := b.privateKey.PublicKey()

	if tradeInfo == nil {
		logger.Infof("未找到交易信息，跳过 %s", _signature)
		return nil
	}
	tradeInfo.Router = "externalRouter"
	global.SetAmmTradeInfo(mintAddress, tradeInfo)
	logger.Debugf("AMM交易信息: 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f",
		tradeInfo.UserAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice)

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
			global.SetTradeAssetOnce(
				mintAddress,
				tradeInfo.Creator,
				botWallet.String(),
				entryPrice,
				tradeInfo.TokenAmount,
				6,
			)
			assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				go queue.SendTokenToSellChannel(global.GetConfig().SellDelayTime, assetInfo, entryPrice, "pumpamm", "delay_sell", "延迟卖出")
			}
		} else {
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				global.DelPendingHoldAssetWithAmount(mintAddress, tradeInfo.TokenAmount)
			}
		}
	}

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
