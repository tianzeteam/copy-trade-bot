package bot

import (
	"context"
	"log"
	"math/big"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/types"
	goyser "copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"

	"copytrade/pumpamm"

	"github.com/mr-tron/base58"
)

// 全局AMM交易信息映射表已移至 global_maps.go

// parseAmmTransaction 解析AMM交易事件
func (b *Bot) parseAmmTransaction(logMessages []string, mintAddress string) *global.AmmTradeEventInfo {
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
				// 从买入事件中提取信息

				// 创建交易事件信息对象
				tradeInfo := &global.AmmTradeEventInfo{
					UserAddress:  buyEvent.User,
					SolAmount:    buyEvent.QuoteAmountIn,
					TokenAmount:  buyEvent.BaseAmountOut,
					IsBuy:        true,
					MintAddress:  solana.MustPublicKeyFromBase58(mintAddress), // 需要从其他地方获取
					AmmPool:      buyEvent.Pool,
					TimeStamp:    buyEvent.Timestamp,
					BaseReserve:  buyEvent.PoolBaseTokenReserves,
					QuoteReserve: buyEvent.PoolQuoteTokenReserves,
					ReceivedAt:   time.Now(),
				}

				// 计算当前价格 (SOL/TOKEN)
				// 使用大数计算以避免精度问题
				solDecimal := 1e9   // SOL有9个小数位
				tokenDecimal := 1e6 // Token有6个小数位

				// 转换为浮点数进行价格计算
				tokenReserveFloat := float64(buyEvent.PoolBaseTokenReserves) / tokenDecimal
				solReserveFloat := float64(buyEvent.PoolQuoteTokenReserves) / solDecimal

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
					BaseReserve:  sellEvent.PoolBaseTokenReserves,
					QuoteReserve: sellEvent.PoolQuoteTokenReserves,
					ReceivedAt:   time.Now(),
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

// PumpAmmParse 解析PumpAMM交易日志
func (b *Bot) PumpAmmParse(out *yellowstone_geyser_pb.SubscribeUpdate, client *goyser.Client, ctx context.Context) *global.AmmTradeEventInfo {
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
	cacheBlockhash := base58.Encode(messages.GetRecentBlockhash())
	logger.Debugf("recent blockhash :%s", cacheBlockhash)
	b.CacheLatestBlockhash(cacheBlockhash)
	//inners:=txn.Transaction.Meta.InnerInstructions[0].
	instructions := messages.Instructions
	_signature := base58.Encode(txn.Transaction.Transaction.Signatures[0])
	var mintAddress string
	for _, instruction := range instructions {
		// 安全检查：确保账户数据不为空
		if len(instruction.Accounts) == 0 {
			logger.Debug("跳过空账户指令")
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

		// 比较判别器
		switch discriminator {
		case pumpamm.BuyDiscriminator:

			accountList := b.GetAccountList(out)
			user := accountList[instruction.Accounts[1]]
			mintAddress = accountList[instruction.Accounts[3]]
			pool := accountList[instruction.Accounts[0]]

			logger.Debug("匹配-------买入------------判别器")

			logger.Debugf("user :%s ,mint :%s  pool %s", user, mintAddress, pool)
			logger.Debug("_signature: ", _signature)
		case pumpamm.SellDiscriminator:
			logger.Debug("匹配--------卖出-------------判别器")
		case pumpamm.CreateDiscriminator:
			logger.Debug("匹配 Mint 判别器")
		default:
			logger.Debug("未匹配判别器")
		}
	}

	if mintAddress == "" {
		return nil
	}

	// 处理AMM交易信息
	tradeInfo := b.parseAmmTransaction(logMessages, mintAddress)
	/* 	if tradeInfo != nil {
		global.SetAmmTradeInfo(mintAddress, tradeInfo)
		logger.Infof("AMM交易信息: 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f",
			tradeInfo.UserAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice)
		// 解析Mint地址
		mintPubkey, err := solana.PublicKeyFromBase58(tradeInfo.MintAddress)
		if err != nil {
			logger.Errorf("无法解析Mint地址: %v", err)
			return tradeInfo
		}

		// 解析用户地址
		userPubkey, err := solana.PublicKeyFromBase58(tradeInfo.UserAddress)
		if err != nil {
			logger.Errorf("无法解析用户地址: %v", err)
			return tradeInfo
		}

		tokenInfo := types.TokenInfo{
			Mint:         mintPubkey,
			Amount:       tradeInfo.TokenAmount,
			User:         userPubkey,
			Owner:        userPubkey,
			UpdateTime:   time.Now(),
			EntryPrice:   tradeInfo.CurrentPrice,
			CurrentPrice: tradeInfo.CurrentPrice,
			Platform:     "pumpamm",
		}

		// 发送代币信息到全局通道
		logger.Debugf("发送代币信息到通道: Mint=%s, 余额=%d", tokenInfo.Mint, tokenInfo.Amount)
		select {
		case types.WaitingSellChan <- tokenInfo:
			logger.Debugf("代币信息已发送到通道")
		default:
			logger.Debugf("通道已满，无法发送代币信息")
		}
	} */
	return tradeInfo
}

// PumpAmmSell 处理 PumpAMM 平台的代币卖出操作
func (b *Bot) PumpAmmSell(tokenInfo types.TokenInfo) {
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

func (b *Bot) GetAccountList(out *yellowstone_geyser_pb.SubscribeUpdate) []string {
	txn := out.GetTransaction()
	length := len(txn.Transaction.Transaction.Message.AccountKeys) +
		len(txn.Transaction.Meta.LoadedWritableAddresses) +
		len(txn.Transaction.Meta.LoadedReadonlyAddresses)
	AccountList := make([]string, length)

	var i = 0
	for _, v := range txn.GetTransaction().Transaction.Message.AccountKeys {
		AccountList[i] = base58.Encode(v)
		i++
	}
	for _, v := range txn.Transaction.Meta.LoadedWritableAddresses {
		AccountList[i] = base58.Encode(v)
		i++
	}
	for _, v := range txn.Transaction.Meta.LoadedReadonlyAddresses {
		AccountList[i] = base58.Encode(v)
		i++
	}
	return AccountList
}

func (b *Bot) PumpAmmPriceUpdate(priceStreamClient *goyser.StreamClient, outPrice *yellowstone_geyser_pb.SubscribeUpdate, mintAddress string) {
	// 使用全局 AmmPoolLayout 对象，避免每次创建新对象
	AmmPoolLayout := pumpamm.PUMP_AMM_POOL_LAYOUT{}
	data := outPrice.GetAccount().Account.GetData()
	if err := AmmPoolLayout.Decode(data); err != nil {
		logger.Errorf("Failed to decode liquidity state layout: %v\n", err)
		return
	}

	// 获取池中的代币储备信息
	swapBaseInAmount := AmmPoolLayout.LpSupply
	swapQuoteOutAmount := AmmPoolLayout.LpSupply
	poolBaseReserve := AmmPoolLayout.LpMint
	poolQuoteReserve := AmmPoolLayout.LpMint

	// 定义代币精度
	var (
		baseDecimals  = big.NewInt(1e9) // WSOL decimals (基础代币精度)
		quoteDecimals = big.NewInt(1e6) // 代币精度 (通常为6位小数)
	)

	// 转换为大整数以进行精确计算
	swapBaseInAmount_bigint := new(big.Int).SetUint64(swapBaseInAmount)
	swapQuoteOutAmount_bigint := new(big.Int).SetUint64(swapQuoteOutAmount)

	// 记录交易数量日志
	log.Printf("Pool update - Base in: %s, Quote out: %s",
		swapBaseInAmount_bigint.String(),
		swapQuoteOutAmount_bigint.String())

	// 获取当前池储备量
	baseReserve, _ := new(big.Int).SetString(poolBaseReserve.String(), 10)
	quoteReserve, _ := new(big.Int).SetString(poolQuoteReserve.String(), 10)

	// 转换为浮点数进行价格计算
	baseReserveFloat := new(big.Float).SetInt(baseReserve)
	quoteReserveFloat := new(big.Float).SetInt(quoteReserve)
	baseDecimalsFloat := new(big.Float).SetInt(baseDecimals)
	quoteDecimalsFloat := new(big.Float).SetInt(quoteDecimals)

	// 计算价格比率和当前价格
	priceRatio := new(big.Float).Quo(quoteDecimalsFloat, baseDecimalsFloat)
	quotePrice := new(big.Float).Quo(baseReserveFloat, quoteReserveFloat)
	currentPrice := quotePrice.Mul(quotePrice, priceRatio)

	log.Printf("Current price for %s: %s", mintAddress, currentPrice.String())

	// 检查是否需要触发卖出操作
	if _, ok := global.PendingHoldAsset[mintAddress]; ok {
		// 获取买入信息
		myBuyMap := global.GetPendingHoldAsset(mintAddress)
		if myBuyMap == nil {
			return
		}
		entryPrice := myBuyMap.EntryPrice
		amount := myBuyMap.Amount

		// 转换当前价格为浮点数进行比较
		currentPriceFloat, _ := currentPrice.Float64()

		// 如果当前价格高于入场价格且卖出通道可用，则触发卖出
		if currentPriceFloat > entryPrice && types.WaitingSellChan != nil {
			logger.Infof("触发卖出条件: 当前价格 %.6f > 入场价格 %.6f", currentPriceFloat, entryPrice)

			// 解析代币地址
			mintPubkey, err := solana.PublicKeyFromBase58(mintAddress)
			if err != nil {
				logger.Errorf("无法解析Mint地址: %v", err)
				return
			}

			// 解析所有者地址
			ownerPubkey := b.privateKey.PublicKey()

			// 创建代币信息对象
			tokenInfo := types.TokenInfo{
				Mint:         mintPubkey,
				Owner:        ownerPubkey,
				Amount:       amount,
				User:         ownerPubkey,
				UpdateTime:   time.Now(),
				EntryPrice:   entryPrice,
				CurrentPrice: currentPriceFloat,
				Platform:     "pumpamm",
			}

			// 发送代币信息到全局通道
			logger.Infof("发送代币信息到卖出通道: Mint=%s, 余额=%d, 入场价=%.6f, 当前价=%.8f",
				tokenInfo.Mint, tokenInfo.Amount, entryPrice, currentPriceFloat)

			// 尝试发送到通道，使用select避免阻塞
			select {
			case types.WaitingSellChan <- tokenInfo:
				logger.Debugf("代币信息已成功发送到卖出通道")
			default:
				logger.Debugf("卖出通道已满，无法发送代币信息")
			}
		}
	}
}
