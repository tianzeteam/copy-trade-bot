package bot

import (
	"context"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/raydium"
	goyser "copytrade/yellowstone_geyser"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mr-tron/base58"
	"github.com/near/borsh-go"
)

type SwapData struct {
	Discriminator    uint8
	AmountIn         uint64
	MinimumAmountOut uint64
}

// RaydiumV4Parse 解析Raydium pool V4交易日志
func (b *Bot) RaydiumV4Parse(out *yellowstone_geyser_pb.SubscribeUpdate, client *goyser.Client, ctx context.Context) *global.AmmRaydiumV4TradeInfo {
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
	var tradeInfo *global.AmmRaydiumV4TradeInfo
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
		var discriminator = data[0]
		// 比较判别器
		switch discriminator {
		case raydium.SwapDiscriminator:
			var swapData SwapData
			err := borsh.Deserialize(&swapData, data)
			if err != nil {
				return nil
			}
			logger.Debugf("SwapBaseIn instruction found, signature: %s", _signature)
			accounts := instruction.Accounts
			if len(accounts) < 18 {
				logger.Debug("Not enough accounts in the instruction")
				return nil
			}

			accountList := b.GetAccountList(out)

			// Map accounts based on the structure you provided for SwapDiscriminator (type 9)
			_ = accountList[accounts[0]]                     // #1 - Token Program
			ammId := accountList[accounts[1]]                // #2 - Amm Id (Raydium Market)
			_ = accountList[accounts[2]]                     // #3 - Amm Authority
			openOrders := accountList[accounts[3]]           // #4 - Amm Open Orders
			targetOrders := accountList[accounts[4]]         // #5 - Amm Target Orders
			poolCoinTokenAccount := accountList[accounts[5]] // #6 - Pool Coin Token Account
			poolPcTokenAccount := accountList[accounts[6]]   // #7 - Pool Pc Token Account
			_ = accountList[accounts[7]]                     // #8 - Serum Program
			serumMarket := accountList[accounts[8]]          // #9 - Serum Market
			_ = accountList[accounts[9]]                     // #10 - Serum Bids
			_ = accountList[accounts[10]]                    // #11 - Serum Asks
			_ = accountList[accounts[11]]                    // #12 - Serum Event Queue
			_ = accountList[accounts[12]]                    // #13 - Serum Coin Vault Account
			_ = accountList[accounts[13]]                    // #14 - Serum Pc Vault Account
			_ = accountList[accounts[14]]                    // #15 - Serum Vault Signer
			_ = accountList[accounts[15]]                    // #16 - User Source Token Account
			_ = accountList[accounts[16]]                    // #17 - User Destination Token Account
			userOwner := accountList[accounts[17]]           // #18 - User Source Owner

			logger.Debugf("Raydium SwapBaseIn: User=%s, Market=%s, AmmPool=%s ,poolCoinTokenAccount=%s,poolPcTokenAccount=%s", userOwner, serumMarket, ammId, poolCoinTokenAccount, poolPcTokenAccount)

			// This is likely a buy operation (SwapBaseIn)
			isBuy := true
			logger.Debugf("Raydium V4 SwapBaseIn detected: IsBuy=%v, openOrders=%s,targetOrders=%s userOwner=%s",
				isBuy, openOrders, targetOrders, userOwner)

			// Extract mint address for tracking
			marketState := b.marketStates[serumMarket]
			if marketState == nil {
				return nil
			}
			logger.Debugf("serumMarket: ", serumMarket)
			mintAddress = marketState.BaseMint.String()
			//preTokenBalances := txn.Transaction.Meta.PreTokenBalances
			//postTokenBalances := txn.Transaction.Meta.PostTokenBalances

			// Check for token balance changes
			//tokenChange := CheckTokenBalances(preTokenBalances, postTokenBalances)

			solChange := GetSwapBalances(txn, poolCoinTokenAccount, poolPcTokenAccount, accountList)

			if solChange != nil {

				_currentPrice := solChange.SolChangeAmount / solChange.TokenChangeAmount
				logger.Debugf("Raydium V4  solChange %.8f, tokenChange %.8f ,currentPrice %.8f ,(decimals=%d)", solChange.SolChangeAmount, solChange.TokenChangeAmount, _currentPrice, solChange.Decimals)
				// Format the token amount with proper decimals
				isBuy := true
				if solChange.SolChangeAmount < 0 {
					isBuy = false
				}

				// Create AmmTradeEventInfo for the detected trade
				tradeInfo = &global.AmmRaydiumV4TradeInfo{
					MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
					AmmPool:      solana.MustPublicKeyFromBase58(ammId),
					OpenOrders:   solana.MustPublicKeyFromBase58(openOrders),
					TargetOrders: solana.MustPublicKeyFromBase58(targetOrders),
					BaseVault:    solana.MustPublicKeyFromBase58(poolCoinTokenAccount),
					QuoteVault:   solana.MustPublicKeyFromBase58(poolPcTokenAccount),

					Version:      4,
					UserAddress:  solana.MustPublicKeyFromBase58(userOwner),
					SolAmount:    uint64(solChange.SolChangeAmount * 1e9),
					TokenAmount:  uint64(solChange.TokenChangeAmount),
					IsBuy:        isBuy,         // Use the IsSell field (inverted since IsBuy is the opposite of IsSell)
					CurrentPrice: _currentPrice, // You may need to calculate this based on other data
					TimeStamp:    time.Now().Unix(),
					ReceivedAt:   time.Now(),
					TxHash:       _signature,
				}
			}

		case raydium.SwapDiscriminator_V2:
			logger.Infof("SwapBaseOut instruction found, signature: %s", _signature)
			accounts := instruction.Accounts
			if len(accounts) < 17 {
				logger.Warn("Not enough accounts in the instruction")
				return nil
			}
			if len(accounts) == 17 {
				accountList := b.GetAccountList(out)

				// Map accounts based on the structure you provided
				_ = accountList[accounts[0]]                     // #1 - Token Program
				ammId := accountList[accounts[1]]                // #2 - Amm Id (Raydium Market)
				_ = accountList[accounts[2]]                     // #3 - Amm Authority
				openOrders := accountList[accounts[3]]           // #4 - Amm Open Orders
				poolCoinTokenAccount := accountList[accounts[4]] // #5 - Pool Coin Token Account
				poolPcTokenAccount := accountList[accounts[5]]   // #6 - Pool Pc Token Account
				_ = accountList[accounts[6]]                     // #7 - Serum Program Id
				serumMarket := accountList[accounts[7]]          // #8 - Serum Market
				_ = accountList[accounts[8]]                     // #9 - Serum Bids
				_ = accountList[accounts[9]]                     // #10 - Serum Asks
				_ = accountList[accounts[10]]                    // #11 - Serum Event Queue
				_ = accountList[accounts[11]]                    // #12 - Serum Coin Vault Account
				_ = accountList[accounts[12]]                    // #13 - Serum Pc Vault Account
				_ = accountList[accounts[13]]                    // #14 - Serum Vault Signer

				userOwner := accountList[accounts[16]] // #17 - User Owner
				// Extract mint address for tracking
				marketState := b.marketStates[serumMarket]
				if marketState == nil {
					return nil
				}
				logger.Infof("serumMarket: ", serumMarket)
				mintAddress = marketState.BaseMint.String()
				// For debugging
				solChange := GetSwapBalances(txn, poolCoinTokenAccount, poolPcTokenAccount, accountList)

				if solChange != nil {

					_currentPrice := solChange.SolChangeAmount / solChange.TokenChangeAmount
					logger.Infof("Raydium V4  solChange %.8f, tokenChange %.8f ,currentPrice %.8f ,(decimals=%d)", solChange.SolChangeAmount, solChange.TokenChangeAmount, _currentPrice, solChange.Decimals)
					// Format the token amount with proper decimals
					isBuy := true
					if solChange.SolChangeAmount < 0 {
						isBuy = false
					}

					// Create AmmTradeEventInfo for the detected trade
					tradeInfo = &global.AmmRaydiumV4TradeInfo{
						MintAddress: solana.MustPublicKeyFromBase58(mintAddress),
						AmmPool:     solana.MustPublicKeyFromBase58(ammId),
						OpenOrders:  solana.MustPublicKeyFromBase58(openOrders),

						BaseVault:  solana.MustPublicKeyFromBase58(poolCoinTokenAccount),
						QuoteVault: solana.MustPublicKeyFromBase58(poolPcTokenAccount),

						Version:     3,
						UserAddress: solana.MustPublicKeyFromBase58(userOwner),

						SolAmount:    uint64(solChange.SolChangeAmount * 1e9),
						TokenAmount:  uint64(solChange.TokenChangeAmount),
						IsBuy:        isBuy,         // Use the IsSell field (inverted since IsBuy is the opposite of IsSell)
						CurrentPrice: _currentPrice, // You may need to calculate this based on other data
						TimeStamp:    time.Now().Unix(),
						ReceivedAt:   time.Now(),
						TxHash:       _signature,
					}
				}
			} else {
				accountList := b.GetAccountList(out)

				// Map accounts based on the structure you provided
				_ = accountList[accounts[0]]                     // #1 - Token Program
				ammId := accountList[accounts[1]]                // #2 - Amm Id (Raydium Market)
				_ = accountList[accounts[2]]                     // #3 - Amm Authority
				openOrders := accountList[accounts[3]]           // #4 - Amm Open Orders
				targetOrders := accountList[accounts[4]]         //#5 - Amm Target Orders
				poolCoinTokenAccount := accountList[accounts[5]] // #5 - Pool Coin Token Account
				poolPcTokenAccount := accountList[accounts[6]]   // #6 - Pool Pc Token Account
				_ = accountList[accounts[7]]                     // #7 - Serum Program Id
				serumMarket := accountList[accounts[8]]          // #8 - Serum Market
				_ = accountList[accounts[9]]                     // #9 - Serum Bids
				_ = accountList[accounts[10]]                    // #10 - Serum Asks
				_ = accountList[accounts[11]]                    // #11 - Serum Event Queue
				_ = accountList[accounts[12]]                    // #12 - Serum Coin Vault Account
				_ = accountList[accounts[13]]                    // #13 - Serum Pc Vault Account
				_ = accountList[accounts[14]]                    // #14 - Serum Vault Signer

				userOwner := accountList[accounts[17]] // #18 - User Owner
				// Extract mint address for tracking
				marketState := b.marketStates[serumMarket]
				if marketState == nil {
					return nil
				}
				logger.Infof("serumMarket: %s", serumMarket)
				mintAddress = marketState.BaseMint.String()
				// For debugging
				solChange := GetSwapBalances(txn, poolCoinTokenAccount, poolPcTokenAccount, accountList)

				if solChange != nil {
					_currentPrice := solChange.SolChangeAmount / solChange.TokenChangeAmount
					logger.Infof("Raydium V4  solChange %.8f, tokenChange %.8f ,currentPrice %.8f ,(decimals=%d)", solChange.SolChangeAmount, solChange.TokenChangeAmount, _currentPrice, solChange.Decimals)
					// Format the token amount with proper decimals
					isBuy := true
					if solChange.SolChangeAmount < 0 {
						isBuy = false
					}

					// Create AmmTradeEventInfo for the detected trade
					tradeInfo = &global.AmmRaydiumV4TradeInfo{
						MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
						AmmPool:      solana.MustPublicKeyFromBase58(ammId),
						OpenOrders:   solana.MustPublicKeyFromBase58(openOrders),
						TargetOrders: solana.MustPublicKeyFromBase58(targetOrders),
						BaseVault:    solana.MustPublicKeyFromBase58(poolCoinTokenAccount),
						QuoteVault:   solana.MustPublicKeyFromBase58(poolPcTokenAccount),

						Version:      4,
						UserAddress:  solana.MustPublicKeyFromBase58(userOwner),
						SolAmount:    uint64(solChange.SolChangeAmount * 1e9),
						TokenAmount:  uint64(solChange.TokenChangeAmount),
						IsBuy:        isBuy,         // Use the IsSell field (inverted since IsBuy is the opposite of IsSell)
						CurrentPrice: _currentPrice, // You may need to calculate this based on other data
						TimeStamp:    time.Now().Unix(),
						ReceivedAt:   time.Now(),
						TxHash:       _signature,
					}
				}
			}

		default:
			logger.Debug("未匹配判别器")
		}
	}

	if mintAddress == "" {
		return nil
	}

	// 处理AMM交易信息
	botWallet := b.privateKey.PublicKey()
	if tradeInfo != nil {
		global.SetRaydiumV4TradeInfo(mintAddress, tradeInfo)
		logger.Debugf("AMM交易信息: 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f",
			tradeInfo.UserAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice)

		userPubkey := tradeInfo.UserAddress
		if tradeInfo.IsBuy {
			// 检查是否是自己的交易
			if userPubkey.String() == botWallet.String() {
				entryPrice := float64(tradeInfo.SolAmount/1e9) / float64(tradeInfo.TokenAmount/1e6)
				// 设置入场价格
				global.SetTradeAssetOnce(
					mintAddress,
					botWallet.String(),
					entryPrice,
					tradeInfo.TokenAmount,
				)
			}
		}

	}
	return tradeInfo
}

// RaydiumV4Sell 处理 Raydium POOL V4 平台的代币卖出操作
func (b *Bot) RaydiumV4Sell(tokenInfo queue.TokenInfo) {
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
