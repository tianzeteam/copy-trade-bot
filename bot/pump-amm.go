package bot

import (
	"context"
	"time"

	"copytrade/asset"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/types"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"

	"copytrade/pumpamm"

	"github.com/mr-tron/base58"
)

// 全局AMM交易信息映射表已移至 global_maps.go

// parseAmmTransaction 解析AMM交易事件
func (b *Bot) parseAmmTransaction(logMessages []string, mintAddress string, tradeDirection string) *global.AmmTradeEventInfo {
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

// PumpAmmParse 解析PumpAMM交易日志
func (b *Bot) PumpAmmParse(out *yellowstone_geyser_pb.SubscribeUpdate, ctx context.Context) *global.AmmTradeEventInfo {
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

	var mintAddress string
	var tradeDirection string
	for _, instruction := range instructions {
		// 安全检查：确保账户数据不为空
		if len(instruction.Accounts) < 15 {
			logger.Debug("跳过空账户指令或者交易信息不完整")
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
		case pumpamm.Instruction_Buy:

			accountList := b.GetAccountList(out)
			user := accountList[instruction.Accounts[1]]
			mintAddress = accountList[instruction.Accounts[3]]

			if mintAddress == solana.SolMint.String() {
				return nil
			}

			pool := accountList[instruction.Accounts[0]]
			tradeDirection = "buy"
			logger.Debugf("匹配-------买入------------判别器")

			logger.Debugf("user :%s ,mint :%s  pool %s", user, mintAddress, pool)
			logger.Debugf("_signature: ", _signature)
		case pumpamm.Instruction_Sell:
			accountList := b.GetAccountList(out)
			user := accountList[instruction.Accounts[1]]
			mintAddress = accountList[instruction.Accounts[3]]
			if mintAddress == solana.SolMint.String() {
				return nil
			}
			pool := accountList[instruction.Accounts[0]]
			tradeDirection = "sell"
			logger.Debugf("匹配-------卖出------------判别器")

			logger.Debugf("user :%s ,mint :%s  pool %s", user, mintAddress, pool)
			logger.Debugf("_signature: ", _signature)
			logger.Debugf("-------------------------------------------")
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
	tradeInfo := b.parseAmmTransaction(logMessages, mintAddress, tradeDirection)

	botWallet := b.privateKey.PublicKey()

	if tradeInfo == nil {
		return nil
	}
	//@TODO
	tradeInfo.Router = "naiveRouter"
	global.SetAmmTradeInfo(mintAddress, tradeInfo)
	logger.Debugf("AMM交易信息: 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f",
		tradeInfo.UserAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice)

	userPubkey := tradeInfo.UserAddress
	if userPubkey.String() == botWallet.String() {
		logger.Debugf("bot 钱包交易记录处理: %s, %v", mintAddress, tradeInfo.IsBuy)
		if tradeInfo.IsBuy {
			// 检查是否是自己的交易

			// 设置入场价格
			global.SetTradeAsset(
				mintAddress,
				tradeInfo.Creator,
				botWallet.String(),
				tradeInfo.CurrentPrice,
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
				// 打印参数模板的所有key value
				logger.Infof("参数模板 ID: %s", specialScheme.ID)
				logger.Infof("  名称: %s", specialScheme.Name)
				logger.Infof("  描述: %s", specialScheme.Description)
				logger.Infof("  是否默认: %v", specialScheme.IsDefault)
				logger.Infof("  创建时间: %v", specialScheme.CreatedAt)
				logger.Infof("  更新时间: %v", specialScheme.UpdatedAt)
				logger.Infof("  卖出延迟时间: %d 毫秒", specialScheme.SellDelayTime)
				logger.Infof("  最小触发买入金额: %.4f SOL", specialScheme.MinFollowTriggerAmount)
				logger.Infof("  最大触发买入金额: %.4f SOL", specialScheme.MaxFollowTriggerAmount)
				logger.Infof("  最小池子大小: %.4f SOL", specialScheme.MinPoolSize)
				logger.Infof("  最大池子大小: %.4f SOL", specialScheme.MaxPoolSize)
				logger.Infof("  买入金额: %.4f SOL", specialScheme.BuySolAmount)
				logger.Infof("  买入滑点: %.2f%%", specialScheme.BuySlippage*100)
				logger.Infof("  卖出滑点: %.2f%%", specialScheme.SellSlippage*100)
				logger.Infof("  买入优先费: %.6f", specialScheme.BuyPriorityFee)
				logger.Infof("  买入小费: %.4f", specialScheme.BuyTipFee)
				logger.Infof("  卖出优先费: %.6f", specialScheme.SellPriorityFee)
				logger.Infof("  卖出小费: %.4f", specialScheme.SellTipFee)
				message := types.ClosePositionMessage{
					MintAddress:  mintAddress,
					CurrentPrice: tradeInfo.CurrentPrice,
					Platform:     "pumpamm",
					MessageType:  "delay_sell",
					Reason:       "延迟卖出",
				}
				go b.sdkClient.PublishDelayedCPMessage(b.Config.TradeChannel, message, specialScheme.SellDelayTime)
			}

		} else {
			_, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				logger.Debugf("bot 钱包卖出记录处理: %s, %v, %d", mintAddress, ok, tradeInfo.TokenAmount)
				global.DelPendingHoldAssetWithAmount(mintAddress, tradeInfo.TokenAmount)
			}
		}
	}

	return tradeInfo
}

// PumpAmmSell 处理 PumpAMM 平台的代币卖出操作
func (b *Bot) PumpAmmSell(tokenInfo queue.TokenInfo) {
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
