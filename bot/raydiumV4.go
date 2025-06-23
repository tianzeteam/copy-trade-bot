package bot

import (
	"context"
	"math"
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

// Constants for account indices in Raydium V4 instructions
const (
	TokenProgramIdx      = 0  // #1 - Token Program
	AmmIdIdx             = 1  // #2 - Amm Id (Raydium Market)
	AmmAuthorityIdx      = 2  // #3 - Amm Authority
	OpenOrdersIdx        = 3  // #4 - Amm Open Orders
	TargetOrdersIdx      = 4  // #5 - Amm Target Orders
	PoolCoinTokenAcctIdx = 5  // #6 - Pool Coin Token Account
	PoolPcTokenAcctIdx   = 6  // #7 - Pool Pc Token Account
	SerumProgramIdx      = 7  // #8 - Serum Program
	SerumMarketIdx       = 8  // #9 - Serum Market
	SerumBidsIdx         = 9  // #10 - Serum Bids
	SerumAsksIdx         = 10 // #11 - Serum Asks
	SerumEventQueueIdx   = 11 // #12 - Serum Event Queue
	SerumCoinVaultIdx    = 12 // #13 - Serum Coin Vault Account
	SerumPcVaultIdx      = 13 // #14 - Serum Pc Vault Account
	SerumVaultSignerIdx  = 14 // #15 - Serum Vault Signer
	UserSourceTokenIdx   = 15 // #16 - User Source Token Account
	UserDestTokenIdx     = 16 // #17 - User Destination Token Account
	UserOwnerIdx         = 17 // #18 - User Source Owner

	// Minimum account counts
	MinSwapAccounts   = 18
	MinSwapV2Accounts = 17
)

// SwapData represents the deserialized swap instruction data
type SwapData struct {
	Discriminator    uint8
	AmountIn         uint64
	MinimumAmountOut uint64
}

// extractAccountInfo extracts and returns the account information from instruction accounts
func (b *Bot) extractAccountInfo(accounts []byte, accountList []string) (ammId, openOrders, targetOrders, poolCoinTokenAccount, poolPcTokenAccount, serumMarket, userOwner string, hasTargetOrders bool, version uint32) {
	// First determine the version based on account count
	if len(accounts) >= MinSwapAccounts {
		// V4 structure with target orders
		version = 4
		hasTargetOrders = true

		// Extract accounts for V4
		ammId = accountList[accounts[AmmIdIdx]]               // #2 - Amm Id (Raydium Market)
		openOrders = accountList[accounts[OpenOrdersIdx]]     // #4 - Amm Open Orders
		targetOrders = accountList[accounts[TargetOrdersIdx]] // #5 - Amm Target Orders
		poolCoinTokenAccount = accountList[accounts[5]]       // #6 - Pool Coin Token Account (shifted due to target orders)
		poolPcTokenAccount = accountList[accounts[6]]         // #7 - Pool Pc Token Account (shifted due to target orders)
		serumMarket = accountList[accounts[8]]                // #9 - Serum Market (shifted due to target orders)
		userOwner = accountList[accounts[UserOwnerIdx]]       // #18 - User Source Owner
	} else if len(accounts) >= MinSwapV2Accounts {
		// V3 structure without target orders
		version = 3
		hasTargetOrders = false

		// Extract accounts for V3
		ammId = accountList[accounts[AmmIdIdx]]             // #2 - Amm Id (Raydium Market)
		openOrders = accountList[accounts[OpenOrdersIdx]]   // #4 - Amm Open Orders
		poolCoinTokenAccount = accountList[accounts[4]]     // #5 - Pool Coin Token Account (no target orders)
		poolPcTokenAccount = accountList[accounts[5]]       // #6 - Pool Pc Token Account (no target orders)
		serumMarket = accountList[accounts[7]]              // #8 - Serum Market (no target orders)
		userOwner = accountList[accounts[UserDestTokenIdx]] // #17 - User Owner in V3 structure
	}

	return
}

// createTradeInfo creates an AmmRaydiumV4TradeInfo struct from the extracted data
func (b *Bot) createTradeInfo(
	mintAddress, ammId, openOrders, targetOrders, poolCoinTokenAccount,
	poolPcTokenAccount, serumMarket, userOwner, signature string,
	solChange *SolBalanceChange, version uint32, hasTargetOrders bool, solReserve float64) *global.AmmRaydiumV4TradeInfo {

	if solChange == nil {
		return nil
	}

	// Calculate current price
	currentPrice := solChange.SolChangeAmount / solChange.TokenChangeAmount

	// Determine if this is a buy or sell
	isBuy := solChange.SolChangeAmount >= 0

	// Log the trade details
	logger.Debugf("Raydium V%d mintAddress: %s, solChange %.8f, tokenChange %.8f, currentPrice %.8f, (decimals=%d)",
		version, mintAddress, solChange.SolChangeAmount, solChange.TokenChangeAmount, currentPrice, solChange.Decimals)

	// Create the trade info struct
	tradeInfo := &global.AmmRaydiumV4TradeInfo{
		MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
		AmmPool:      solana.MustPublicKeyFromBase58(ammId),
		OpenOrders:   solana.MustPublicKeyFromBase58(openOrders),
		BaseVault:    solana.MustPublicKeyFromBase58(poolCoinTokenAccount),
		QuoteVault:   solana.MustPublicKeyFromBase58(poolPcTokenAccount),
		MarketId:     solana.MustPublicKeyFromBase58(serumMarket),
		UserAddress:  solana.MustPublicKeyFromBase58(userOwner),
		SolAmount:    uint64(solChange.SolChangeAmount * 1e9),
		TokenAmount:  uint64(solChange.TokenChangeAmount * math.Pow10(int(solChange.Decimals))),
		IsBuy:        isBuy,
		CurrentPrice: currentPrice,
		TimeStamp:    time.Now().Unix(),
		ReceivedAt:   time.Now(),
		TxHash:       signature,
		Decimal:      solChange.Decimals,
		Version:      version,
		SolReserve:   uint64(solReserve * 1e9),
	}

	// Add target orders if available
	if hasTargetOrders {
		tradeInfo.TargetOrders = solana.MustPublicKeyFromBase58(targetOrders)
	}

	return tradeInfo
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
			if len(accounts) < 17 {
				logger.Debug("Not enough accounts in the instruction")
				return nil
			}

			accountList := b.GetAccountList(out)

			// Extract account information using the optimized function
			ammId, openOrders, targetOrders, poolCoinTokenAccount, poolPcTokenAccount, serumMarket, userOwner, hasTargetOrders, version := b.extractAccountInfo(accounts, accountList)

			logger.Debugf("Raydium SwapBaseIn: User=%s, Market=%s, AmmPool=%s, poolCoinTokenAccount=%s, poolPcTokenAccount=%s",
				userOwner, serumMarket, ammId, poolCoinTokenAccount, poolPcTokenAccount)

			// Extract mint address for tracking
			marketState := global.GetOpenBookMarketState(serumMarket)
			if marketState == nil {
				return nil
			}
			logger.Debugf("serumMarket: %s", serumMarket)
			mintAddress = marketState.BaseMint.String()

			// Get balance changes
			solChange := GetSwapBalances(txn, poolCoinTokenAccount, poolPcTokenAccount, accountList)
			logger.Debugf("mintAddress: %s, solAmount: %.8f, solReserve: %.8f", mintAddress, solChange.SolChangeAmount, solChange.SolReserve)

			// Create trade info using the helper function
			tradeInfo = b.createTradeInfo(
				mintAddress, ammId, openOrders, targetOrders,
				poolCoinTokenAccount, poolPcTokenAccount, serumMarket,
				userOwner, _signature, solChange, version, hasTargetOrders, solChange.SolReserve)

		case raydium.SwapDiscriminator_V2:
			logger.Debugf("SwapBaseOut instruction found, signature: %s", _signature)
			accounts := instruction.Accounts
			if len(accounts) < 17 {
				logger.Warn("Not enough accounts in the instruction")
				return nil
			}

			// Get account list once
			accountList := b.GetAccountList(out)

			// Extract account information using the optimized function
			ammId, openOrders, targetOrders, poolCoinTokenAccount, poolPcTokenAccount, serumMarket, userOwner, hasTargetOrders, version := b.extractAccountInfo(accounts, accountList)

			// Extract mint address for tracking
			marketState := global.GetOpenBookMarketState(serumMarket)
			if marketState == nil {
				return nil
			}
			logger.Debugf("serumMarket: %s", serumMarket)
			mintAddress = marketState.BaseMint.String()

			// Get balance changes
			solChange := GetSwapBalances(txn, poolCoinTokenAccount, poolPcTokenAccount, accountList)
			logger.Debugf("mintAddress: %s, solReserve: %.8f", mintAddress, solChange.SolReserve)
			// Create trade info using the helper function
			tradeInfo = b.createTradeInfo(
				mintAddress, ammId, openOrders, targetOrders,
				poolCoinTokenAccount, poolPcTokenAccount, serumMarket,
				userOwner, _signature, solChange, version, hasTargetOrders, solChange.SolReserve)

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
				var entryPrice float64
				if tradeInfo.TokenAmount > 0 {
					solAmountFloat := float64(tradeInfo.SolAmount) / 1e9
					tokenAmountFloat := float64(tradeInfo.TokenAmount) / math.Pow10(int(tradeInfo.Decimal))
					entryPrice = solAmountFloat / tokenAmountFloat
				} else {
					logger.Warnf("计算入场价格时发现代币数量为零: %d", tradeInfo.TokenAmount)
					entryPrice = 0
				}

				logger.Infof("买入交易: SOL=%d , 代币=%d , 价格=%.8f",
					tradeInfo.SolAmount, tradeInfo.TokenAmount,
					entryPrice)
				// 设置入场价格
				global.SetTradeAssetOnce(
					mintAddress,
					solana.PublicKey{},
					botWallet.String(),
					entryPrice,
					tradeInfo.TokenAmount,
					tradeInfo.Decimal,
				)
				assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
				if ok {
					time.Sleep(time.Duration(global.GetConfig().SellDelayTime) * time.Millisecond)
					queue.SendTokenToSellChannel(assetInfo, entryPrice, "raydiumV4", "delay_sell", "延迟卖出")
				}
			}
		}

	}
	return tradeInfo
}

// RaydiumV4Sell 处理 Raydium POOL V4 平台的代币卖出操作
func (b *Bot) RaydiumV4Sell(tokenInfo queue.TokenInfo) {
	// 使用策略模式进行卖出
	logger.Infof("使用策略模式卖出代币: %s", tokenInfo.Mint.String())

	// 执行卖出策略
	b.ExecuteRaydiumV4SellOrder(tokenInfo)
}
