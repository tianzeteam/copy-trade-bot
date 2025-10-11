package bot

import (
	"context"
	"time"

	"copytrade/asset"
	"copytrade/dbc"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/types"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

// parseDbcSwapEvent 解析DBC Swap事件
func (b *Bot) parseDbcSwapEvent(logMessages []string, pool string, config string, user string, mintAddress string) *global.DbcTradeEventInfo {
	// 使用dbc包中的事件解析功能
	for _, logMessage := range logMessages {
		if len(logMessage) < 20 {
			continue
		}

		// 查找事件数据
		// Solana日志格式: "Program data: <base64_encoded_data>"
		if len(logMessage) > 14 && logMessage[:14] == "Program data: " {
			eventDataStr := logMessage[14:]
			eventData, err := base58.Decode(eventDataStr)
			if err != nil {
				logger.Debugf("解码事件数据失败: %v", err)
				continue
			}

			// 尝试解析为EvtSwap事件
			event, err := dbc.ParseAnyEvent(eventData)
			if err != nil {
				logger.Debugf("解析DBC事件失败: %v", err)
				continue
			}

			// 检查是否是Swap事件
			switch evt := event.(type) {
			case *dbc.EvtSwap:
				return b.processEvtSwap(evt, pool, config, user, mintAddress)
			case *dbc.EvtSwap2:
				return b.processEvtSwap2(evt, pool, config, user, mintAddress)
			}
		}
	}

	return nil
}

// processEvtSwap 处理EvtSwap事件
func (b *Bot) processEvtSwap(evt *dbc.EvtSwap, poolStr string, configStr string, userStr string, mintAddress string) *global.DbcTradeEventInfo {
	// TradeDirection: 0 = Buy (SOL -> Token), 1 = Sell (Token -> SOL)
	isBuy := evt.TradeDirection == 0

	var solAmount, tokenAmount uint64
	if isBuy {
		solAmount = evt.AmountIn
		tokenAmount = evt.SwapResult.OutputAmount
	} else {
		tokenAmount = evt.AmountIn
		solAmount = evt.SwapResult.OutputAmount
	}

	// 创建交易事件信息对象
	tradeInfo := &global.DbcTradeEventInfo{
		UserAddress: solana.MustPublicKeyFromBase58(userStr),
		SolAmount:   solAmount,
		TokenAmount: tokenAmount,
		IsBuy:       isBuy,
		MintAddress: solana.MustPublicKeyFromBase58(mintAddress),
		Pool:        evt.Pool,
		Config:      evt.Config,
		TimeStamp:   evt.CurrentTimestamp,
		ReceivedAt:  time.Now(),
		Router:      "meteora-dbc",
	}

	// 计算当前价格 (SOL/TOKEN)
	// 需要从池状态获取储备量信息
	// 这里暂时使用交易数据估算
	if tokenAmount > 0 {
		solDecimal := 1e9   // SOL有9个小数位
		tokenDecimal := 1e6 // Token通常有6个小数位

		solAmountFloat := float64(solAmount) / solDecimal
		tokenAmountFloat := float64(tokenAmount) / tokenDecimal

		if tokenAmountFloat > 0 {
			tradeInfo.CurrentPrice = solAmountFloat / tokenAmountFloat
		}
	}

	// 格式化交易时间
	tradeInfo.TradeTime = time.Unix(int64(evt.CurrentTimestamp), 0).Format("2006-01-02 15:04:05")

	logger.Infof("解析到DBC Swap事件: 用户=%s, 代币=%s, SOL=%d, Token=%d, 是否买入=%v, 价格=%.8f",
		tradeInfo.UserAddress, tradeInfo.MintAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice)

	return tradeInfo
}

// processEvtSwap2 处理EvtSwap2事件
func (b *Bot) processEvtSwap2(evt *dbc.EvtSwap2, poolStr string, configStr string, userStr string, mintAddress string) *global.DbcTradeEventInfo {
	// TradeDirection: 0 = Buy (SOL -> Token), 1 = Sell (Token -> SOL)
	isBuy := evt.TradeDirection == 0

	var solAmount, tokenAmount uint64
	if isBuy {
		solAmount = evt.SwapParameters.Amount0
		tokenAmount = evt.SwapResult.OutputAmount
	} else {
		tokenAmount = evt.SwapParameters.Amount0
		solAmount = evt.SwapResult.OutputAmount
	}

	// 创建交易事件信息对象
	tradeInfo := &global.DbcTradeEventInfo{
		UserAddress:  solana.MustPublicKeyFromBase58(userStr),
		SolAmount:    solAmount,
		TokenAmount:  tokenAmount,
		IsBuy:        isBuy,
		MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
		Pool:         evt.Pool,
		Config:       evt.Config,
		TimeStamp:    evt.CurrentTimestamp,
		BaseReserve:  0, // 从池状态获取
		QuoteReserve: evt.QuoteReserveAmount,
		ReceivedAt:   time.Now(),
		Router:       "meteora-dbc",
	}

	// 计算当前价格 (SOL/TOKEN)
	if tokenAmount > 0 {
		solDecimal := 1e9   // SOL有9个小数位
		tokenDecimal := 1e6 // Token通常有6个小数位

		solAmountFloat := float64(solAmount) / solDecimal
		tokenAmountFloat := float64(tokenAmount) / tokenDecimal

		if tokenAmountFloat > 0 {
			tradeInfo.CurrentPrice = solAmountFloat / tokenAmountFloat
		}
	}

	// 格式化交易时间
	tradeInfo.TradeTime = time.Unix(int64(evt.CurrentTimestamp), 0).Format("2006-01-02 15:04:05")

	logger.Debugf("解析到DBC Swap2事件: 用户=%s, 代币=%s, SOL=%d, Token=%d, 是否买入=%v, 价格=%.8f, Quote储备=%d",
		tradeInfo.UserAddress, tradeInfo.MintAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice, tradeInfo.QuoteReserve)

	return tradeInfo
}

// MeteoraDbcParse 解析Meteora DBC交易日志
func (b *Bot) MeteoraDbcParse(out *yellowstone_geyser_pb.SubscribeUpdate, ctx context.Context) *global.DbcTradeEventInfo {
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

	messages := txn.Transaction.Transaction.Message
	instructions := messages.Instructions
	_signature := base58.Encode(txn.Transaction.Transaction.Signatures[0])

	var pool, config, user, mintAddress string
	var foundSwap bool

	// DBC Program ID
	dbcProgramID := solana.MustPublicKeyFromBase58("dbcij3LWUppWqq96dh6gJWwBifmcGfLSB5D4DuSMaqN")

	for _, instruction := range instructions {
		// 获取程序ID
		if int(instruction.ProgramIdIndex) >= len(messages.AccountKeys) {
			continue
		}
		programID := base58.Encode(messages.AccountKeys[instruction.ProgramIdIndex])

		// 检查是否是DBC程序
		if programID != dbcProgramID.String() {
			continue
		}

		// 安全检查：确保数据长度足够
		data := instruction.GetData()
		if len(data) < 8 {
			logger.Debug("指令数据不足8字节，跳过")
			continue
		}

		// 安全地获取判别器
		var discriminator [8]byte
		copy(discriminator[:], data[:8])

		// 比较判别器 - Swap指令
		if discriminator == dbc.Instruction_Swap || discriminator == dbc.Instruction_Swap2 {
			accountList := b.GetAccountList(out)

			// Swap指令账户布局:
			// 0: pool_authority
			// 1: config
			// 2: pool
			// 3: input_token_account (user's token account)
			// 4: output_token_account (user's token account)
			// 5: base_vault
			// 6: quote_vault
			// 7: base_mint
			// 8: quote_mint
			// 9: payer (user)
			// 10: token_base_program
			// 11: token_quote_program
			// 12: referral_token_account
			// 13: event_authority
			// 14: program

			if len(instruction.Accounts) < 10 {
				logger.Debug("DBC Swap指令账户数量不足")
				continue
			}

			config = accountList[instruction.Accounts[1]]
			pool = accountList[instruction.Accounts[2]]
			user = accountList[instruction.Accounts[9]]
			baseMint := accountList[instruction.Accounts[7]]
			quoteMint := accountList[instruction.Accounts[8]]

			// 判断交易方向：如果base_mint不是SOL，则它是代币mint
			solMint := solana.SolMint.String()
			if baseMint != solMint {
				mintAddress = baseMint
			} else if quoteMint != solMint {
				mintAddress = quoteMint
			} else {
				logger.Debug("无法确定代币mint地址")
				continue
			}

			foundSwap = true
			logger.Debugf("匹配DBC Swap指令: pool=%s, user=%s, mint=%s", pool, user, mintAddress)
			logger.Debugf("交易签名: %s", _signature)
			break
		}
	}

	if !foundSwap || mintAddress == "" {
		return nil
	}

	// 处理DBC交易信息
	tradeInfo := b.parseDbcSwapEvent(logMessages, pool, config, user, mintAddress)
	if tradeInfo == nil {
		return nil
	}

	botWallet := global.GetConfig().TradeWalletKey.PublicKey()

	// 存储到全局映射表
	// 注意：这里需要根据实际需求决定是否存储到现有的映射表或创建新的映射表
	// 暂时使用类似AMM的方式存储

	global.SetDBCTradeInfo(mintAddress, tradeInfo)

	logger.Debugf("DBC交易信息: 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f",
		tradeInfo.UserAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice)

	// 处理机器人自己的交易
	if tradeInfo.UserAddress.String() == botWallet.String() {
		logger.Debugf("bot 钱包DBC交易记录处理: %s, %v", mintAddress, tradeInfo.IsBuy)
		if tradeInfo.IsBuy {
			// 设置入场价格
			global.SetTradeAsset(
				mintAddress,
				solana.PublicKey{}, // DBC没有creator概念，使用空地址
				botWallet.String(),
				tradeInfo.CurrentPrice,
				tradeInfo.TokenAmount,
				6, // 假设代币精度为6
			)

			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				assetManager := asset.GetHoldingsManager()
				wallets := assetManager.GetWalletsByMint(mintAddress)
				if len(wallets) == 0 {
					logger.Errorf("未找到代币 %s 的持仓", mintAddress)
					return tradeInfo
				}
				walletAddress := wallets[0]
				specialScheme := global.GetConfigByWallet(walletAddress)
				if specialScheme == nil {
					logger.Errorf("未找跟单钱包的配置参数模版 %s", walletAddress)
					return tradeInfo
				}

				message := types.ClosePositionMessage{
					MintAddress:  mintAddress,
					CurrentPrice: tradeInfo.CurrentPrice,
					Platform:     "meteora-dbc",
					MessageType:  "delay_sell",
					Reason:       "延迟卖出",
					CopyAddress:  walletAddress,
				}
				go b.sdkClient.PublishDelayedCPMessage(b.Config.TradeChannel, message, specialScheme.SellDelayTime)
			}

		} else {
			// 卖出记录处理
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				logger.Debugf("bot 钱包DBC卖出记录处理: %s, %d", mintAddress, tradeInfo.TokenAmount)
				global.DelPendingHoldAssetWithAmount(mintAddress, tradeInfo.TokenAmount)
			}
		}
	}

	return tradeInfo
}

// MeteoraDbcSell 处理 Meteora DBC 平台的代币卖出操作
func (b *Bot) MeteoraDbcSell(tokenInfo queue.TokenInfo) {
	// 使用策略模式进行卖出
	logger.Infof("使用策略模式卖出Meteora DBC代币: %s", tokenInfo.Mint.String())

	// 确保卖出策略已初始化
	if b.sellStrategy == nil {
		b.sellStrategy = NewMultiLevelSellStrategy(b.Config.MultiSellLevels)
		logger.Infof("初始化多级卖出策略: %s", b.sellStrategy.Name())
	}

	// 执行卖出策略 - 使用AMM卖出策略
	b.ExecuteAmmSellStrategy(tokenInfo)
}
