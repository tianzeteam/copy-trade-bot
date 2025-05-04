package bot

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"log"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/pump"
	"copytrade/types"
	goyser "copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	_ "github.com/joho/godotenv/autoload"
	"github.com/mr-tron/base58"
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

func (b *Bot) parseTransaction(logMessages []string) *global.TradeEventInfo {
	for _, logMessage := range logMessages {
		// 只处理买入指令
		if strings.Contains(logMessage, "Program log: Instruction: Buy") || strings.Contains(logMessage, "Program log: Instruction: Sell") {
			// 查找并解析Program data
			for _, msg := range logMessages {
				if strings.Contains(msg, "Program data: ") {
					data := strings.Split(msg, "Program data: ")[1]
					decoded, err := base64.StdEncoding.DecodeString(data)
					if err != nil {
						log.Printf("解码失败: %v", err)
						continue
					}

					// 验证数据长度
					if len(decoded) < 8+32+8+8+1+32+8 {
						continue
					}

					// 跳过魔数和版本（8字节）
					offset := 8

					// 检查数据长度是否足够
					if len(decoded) < offset+32+8+8+1+32+8+8+8+8+8 {
						log.Printf("数据长度不足，无法完整解析TradeEvent: %d", len(decoded))
						continue
					}

					// 1. 解析Mint地址 (pubkey - 32字节)
					var mintBytes [32]byte
					copy(mintBytes[:], decoded[offset:offset+32])
					mintAddress := base58.Encode(mintBytes[:])
					offset += 32

					// 2. 解析solAmount (u64 - 8字节)
					solAmount := binary.LittleEndian.Uint64(decoded[offset : offset+8])
					offset += 8

					// 3. 解析tokenAmount (u64 - 8字节)
					tokenAmount := binary.LittleEndian.Uint64(decoded[offset : offset+8])
					offset += 8

					// 4. 解析isBuy (bool - 1字节)
					isBuy := decoded[offset] != 0
					offset += 1

					// 5. 解析用户地址 (pubkey - 32字节)
					var userBytes [32]byte
					copy(userBytes[:], decoded[offset:offset+32])
					userAddress := base58.Encode(userBytes[:])
					offset += 32

					// 6. 解析timestamp (i64 - 8字节)
					timeStamp := int64(binary.LittleEndian.Uint64(decoded[offset : offset+8]))
					offset += 8

					// 7. 解析virtualSolReserves (u64 - 8字节)
					virtualSolReserves := binary.LittleEndian.Uint64(decoded[offset : offset+8])
					offset += 8

					// 8. 解析virtualTokenReserves (u64 - 8字节)
					virtualTokenReserves := binary.LittleEndian.Uint64(decoded[offset : offset+8])
					offset += 8

					// 9. 解析realSolReserves (u64 - 8字节)
					realSolReserves := binary.LittleEndian.Uint64(decoded[offset : offset+8])
					offset += 8

					// 10. 解析realTokenReserves (u64 - 8字节)
					realTokenReserves := binary.LittleEndian.Uint64(decoded[offset : offset+8])
					offset += 8

					// 获取本地时间戳（毫秒）
					milliseconds := time.Now().UnixMilli()
					tradeTime := time.Now().Format("2006-01-02 15:04:05.000")
					_currentPrice := (float64(virtualSolReserves) / 1e9) / (float64(virtualTokenReserves) / 1e6)

					// 创建交易信息对象
					tradeInfo := &global.TradeEventInfo{
						UserAddress:          userAddress,
						SolAmount:            solAmount,
						TokenAmount:          tokenAmount,
						IsBuy:                isBuy,
						MintAddress:          mintAddress,
						TimeStamp:            timeStamp,
						VirtualSolReserves:   virtualSolReserves,
						VirtualTokenReserves: virtualTokenReserves,
						RealSolReserves:      realSolReserves,
						RealTokenReserves:    realTokenReserves,
						CurrentPrice:         _currentPrice,
						TradeTime:            tradeTime,
						Milliseconds:         milliseconds,
						ReceivedAt:           time.Now(),
					}

					// 输出交易信息
					tradeType := "买入"
					if !isBuy {
						tradeType = "卖出"
					}
					logger.Debugf("\n===================== %s交易 =====================\n"+
						"用户地址: %s\n"+
						"Sol数量: %d\n"+
						"Token数量: %d\n"+
						"是否买入: %t\n"+
						"Mint地址: %s\n"+
						"时间戳: %d\n"+
						"虚拟Sol储备: %d\n"+
						"虚拟Token储备: %d\n"+
						"实际Sol储备: %d\n"+
						"实际Token储备: %d\n"+
						"交易时间: %s\n"+
						"时间戳(毫秒): %d\n"+
						"================================================\n",
						tradeType,
						userAddress,
						solAmount,
						tokenAmount,
						isBuy,
						mintAddress,
						timeStamp,
						virtualSolReserves,
						virtualTokenReserves,
						realSolReserves,
						realTokenReserves,
						tradeTime,
						milliseconds,
					)
					return tradeInfo
				}
			}
		}
	}
	return nil
}

// parseMintTransaction 解析代币创建事件
func (b *Bot) parseMintTransaction(logMessages []string) *global.CreateEventInfo {
	for _, logEntry := range logMessages {
		// 检查是否为代币创建事件
		if !isPumpMintLog(logEntry) {
			continue
		}
		logger.Debugf("Found PumpFun mint instruction\n")
		for _, msg := range logMessages {
			if strings.Contains(msg, "Program data:") {
				data := strings.Split(msg, "Program data: ")[1]
				decoded, err := base64.StdEncoding.DecodeString(data)
				if err != nil {
					logger.Infof("解码失败: %v", err)
					continue
				}

				// 跳过魔数和版本（8字节）
				offset := 8

				// 根据CreateEvent结构解析数据
				// 1. 解析name (string)
				nameLen := int(binary.LittleEndian.Uint32(decoded[offset : offset+4]))
				offset += 4
				if offset+nameLen > len(decoded) {
					logger.Infof("数据长度不足，无法解析name字段")
					continue
				}
				name := string(decoded[offset : offset+nameLen])
				offset += nameLen

				// 2. 解析symbol (string)
				symbolLen := int(binary.LittleEndian.Uint32(decoded[offset : offset+4]))
				offset += 4
				if offset+symbolLen > len(decoded) {
					logger.Infof("数据长度不足，无法解析symbol字段")
					continue
				}
				symbol := string(decoded[offset : offset+symbolLen])
				offset += symbolLen

				// 3. 解析uri (string)
				uriLen := int(binary.LittleEndian.Uint32(decoded[offset : offset+4]))
				offset += 4
				if offset+uriLen > len(decoded) {
					logger.Infof("数据长度不足，无法解析uri字段")
					continue
				}
				uri := string(decoded[offset : offset+uriLen])
				offset += uriLen

				// 4. 解析mint (pubkey - 32字节)
				if offset+32 > len(decoded) {
					logger.Errorf("数据长度不足，无法解析mint地址")
					continue
				}
				var mintBytes [32]byte
				copy(mintBytes[:], decoded[offset:offset+32])
				mintAddress := base58.Encode(mintBytes[:])
				offset += 32

				// 5. 解析bondingCurve (pubkey - 32字节)
				if offset+32 > len(decoded) {
					logger.Errorf("数据长度不足，无法解析bondingCurve地址")
					continue
				}
				var bondingCurveBytes [32]byte
				copy(bondingCurveBytes[:], decoded[offset:offset+32])
				bondingCurve := base58.Encode(bondingCurveBytes[:])
				offset += 32

				// 6. 解析user (pubkey - 32字节)
				if offset+32 > len(decoded) {
					logger.Errorf("数据长度不足，无法解析user地址")
					continue
				}
				var userBytes [32]byte
				copy(userBytes[:], decoded[offset:offset+32])
				userAddress := base58.Encode(userBytes[:])

				// 创建代币信息对象
				createInfo := &global.CreateEventInfo{
					Name:         name,
					Symbol:       symbol,
					Uri:          uri,
					MintAddress:  mintAddress,
					BondingCurve: bondingCurve,
					Creator:      userAddress,
					CreatedAt:    time.Now(),
				}

				// 存储到全局映射表中
				global.SetCreateInfo(mintAddress, createInfo)

				// 输出代币创建信息
				logger.Debugf("\n===================== 代币创建事件 =====================\n"+
					"代币名称: %s\n"+
					"代币符号: %s\n"+
					"元数据URI: %s\n"+
					"Mint地址: %s\n"+
					"绑定曲线: %s\n"+
					"创建者: %s\n"+
					"创建时间: %s\n"+
					"================================================\n",
					name,
					symbol,
					uri,
					mintAddress,
					bondingCurve,
					userAddress,
					time.Now().Format("2006-01-02 15:04:05.000"),
				)

				// 异步获取代币元数据信息
				//go b.fetchTokenMetadata(mintAddress, uri)
				return createInfo
			}
		}
	}
	return nil

}

// PumpSellMint 处理代币卖出逻辑
func (b *Bot) PumpSellMint(tokenInfo types.TokenInfo, bondingCurve solana.PublicKey, virtualTokenReserves, virtualSolReserves uint64) {
	ctx := context.Background()
	if b.config.SellMode == "delay_sell" {
		logger.Infof("延迟卖出代币: %s", tokenInfo.Mint.String())
		time.Sleep(time.Duration(b.delaySecond) * time.Second)
		// 先计算卖出金额
		pumpAccounts, err := b.trader.AccountsCalc.CalculatePumpAccounts(ctx, tokenInfo.Mint, tokenInfo.Owner, bondingCurve)
		if err != nil {
			logger.Errorf("计算卖出账户失败: %v", err)
			return
		}
		amounts, err := b.trader.AmountsCalc.CalculateSellAmounts(
			context.Background(),
			pumpAccounts.BondingCurve,
			float64(tokenInfo.Amount),
			virtualTokenReserves,
			virtualSolReserves,
			b.config.SellSlippage,
		)
		if err != nil {
			logger.Errorf("计算卖出金额失败: %v", err)
			return
		}
		//构建交易基础参数
		tradeParams := types.TradeParams{
			Accounts: types.TradeAccounts{
				User:                   tokenInfo.User,
				Mint:                   tokenInfo.Mint,
				BondingCurve:           bondingCurve,
				AssociatedBondingCurve: pumpAccounts.AssociatedBondingCurve,
				AssociatedTokenAccount: pumpAccounts.AssociatedTokenAccount,
			},
			Amount:    amounts.TokenAmount,
			Cost:      amounts.MinOut, // 卖出时 Cost 表示最小获得数量
			Blockhash: *b.blockhash,
			FeeConfig: types.FeeConfig{
				Slippage:    b.config.SellSlippage,
				PriorityFee: uint64(b.config.SellPriorityFee * 1e9),
			},
		}

		// 构建完整的交易参数
		orderParams := types.OrderParams{
			TradeParams: tradeParams,
			BloXrouteParams: &types.BloXrouteParams{
				Enable:    b.config.BloXroute.Enable,
				RpcUrl:    b.config.BloXroute.RpcUrl,
				ApiKeys:   b.config.BloXroute.ApiKeys,
				TipAmount: uint64(b.config.BloXroute.SellTip * 1e9),
			},
			NextBlockParams: &types.NextBlockParams{
				Enable:    b.config.NextBlock.Enable,
				BundleUrl: b.config.NextBlock.RpcUrl,
				ApiKey:    b.config.NextBlock.ApiKeys[0],
				TipAmount: uint64(b.config.NextBlock.SellTip * 1e9),
			},
			JitoParams: &types.JitoParams{
				Enable:    b.config.Jito.Enable,
				BundleUrl: b.config.Jito.RpcUrl,
				TipAmount: uint64(b.config.Jito.SellTip * 1e9),
			},
			TemporalParams: &types.TemporalParams{
				Enable:    b.config.Temporal.Enable,
				BundleUrl: b.config.Temporal.RpcUrl,
				ApiKey:    b.config.Temporal.ApiKeys[0],
				TipAmount: uint64(b.config.Temporal.SellTip * 1e9),
			},
		}

		// 执行卖出交易
		txid, err := b.trader.Sell(ctx, orderParams)
		if err != nil {
			logger.Errorf("卖出失败: %v", err)
			return
		}

		logger.Infof("卖出发送成功! 交易ID: %s", txid)
		logger.Infof("卖出数量（token x 1e6 ）: %d token", amounts.TokenAmount)

	} else if b.config.SellMode == "multi_sell" {
		// 使用策略模式进行卖出
		logger.Infof("使用策略模式卖出代币: %s", tokenInfo.Mint.String())

		// 确保卖出策略已初始化
		if b.sellStrategy == nil {
			b.sellStrategy = NewMultiLevelSellStrategy(b.config.MultiSellLevels)
			logger.Errorf("初始化多级卖出策略: %s", b.sellStrategy.Name())
		}

		// 执行卖出策略
		b.ExecuteSellStrategy(tokenInfo, bondingCurve, virtualTokenReserves, virtualSolReserves)
		return
	} else {
		// 如果卖出模式既不是延迟卖出也不是分批卖出，则输出错误日志
		logger.Errorf("不支持的卖出模式: %s", b.config.SellMode)
	}

}

func (b *Bot) PumpMintParse(out *yellowstone_geyser_pb.SubscribeUpdate, client *goyser.Client, ctx context.Context) *global.TradeEventInfo {
	if out.GetTransaction() == nil {
		return nil
	}
	logMessages := out.GetTransaction().Transaction.Meta.LogMessages
	// 检查是否为代币创建事件
	events, err := pump.DecodeEventsInLogs(logMessages)
	if err != nil {
		logger.Errorf("解析事件失败: %v", err)
		return nil
	}

	for _, event := range events {
		// 检查是否是买入或卖出事件
		switch event.Name {
		case "TradeEvent":
			walletAddress := b.privateKey.PublicKey()
			tradeEvent, ok := event.Data.(*pump.TradeEventEventData)
			if !ok {
				logger.Errorf("无法将事件数据转换为TradeEventEventData类型")
				continue
			}
			if tradeEvent != nil {
				_currentPrice := (float64(tradeEvent.VirtualSolReserves) / 1e9) / (float64(tradeEvent.VirtualTokenReserves) / 1e6)
				mintAddress := tradeEvent.Mint.String()
				if tradeEvent.User.String() == walletAddress.String() && tradeEvent.IsBuy {
					global.SetPendingHoldAsset(mintAddress, &global.MyBuyMap{
						UserAddress: tradeEvent.User.String(),
						MintAddress: mintAddress,
						EntryPrice:  _currentPrice,
						Amount:      tradeEvent.TokenAmount,
						SoldLevels:  make(map[int]bool, 100), // PumpSellMint 使用策略模式处理代币卖出
						TimeStamp:   time.Now().Unix(),
					})
				}
				// Create a TradeEventInfo object from the event data
				tradeInfo := &global.TradeEventInfo{
					MintAddress:          tradeEvent.Mint.String(),
					UserAddress:          tradeEvent.User.String(),
					IsBuy:                tradeEvent.IsBuy,
					SolAmount:            tradeEvent.SolAmount,
					TokenAmount:          tradeEvent.TokenAmount,
					VirtualSolReserves:   tradeEvent.VirtualSolReserves,
					VirtualTokenReserves: tradeEvent.VirtualTokenReserves,
					RealSolReserves:      tradeEvent.RealSolReserves,
					RealTokenReserves:    tradeEvent.RealTokenReserves,
					TimeStamp:            tradeEvent.Timestamp,
				}
				global.SetTradeInfo(mintAddress, tradeInfo)

				return tradeInfo

			}
		case "CreateEvent":

		}
	}
	return nil

}

func (b *Bot) PumpStartPricePing(priceStreamClient *goyser.StreamClient) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for range ticker.C {
		lastPingTime := time.Now()
		if err := priceStreamClient.SendCustomRequest(&yellowstone_geyser_pb.SubscribeRequest{Ping: &yellowstone_geyser_pb.SubscribeRequestPing{}}); err != nil {
			logger.Errorf(" price ping : %v \n", err)
			return
		}
		logger.Infof(" price ping - :%v \n", lastPingTime.Format("15:04:05.000"))
	}
}

func isPumpMintLog(logEntry string) bool {
	const mintPrefix = "Program log: Instruction: InitializeMint2"
	return strings.Contains(logEntry, mintPrefix)
}
