package bot

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mr-tron/base58"

	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/raydium_launchpad"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"
)

// Constants for account indices in Raydium V4 instructions
const (

	// Constants for account indices in Raydium V4 instructions
	PayerIdx      = 0  // #1 - Payer
	PoolStateIdx  = 4  // #2 - Pool State
	BaseVaultIdx  = 7  // #3 - Base Vault
	QuoteVaultIdx = 8  // #4 - Quote Vault
	BaseMintIdx   = 9  // #5 - Base Mint
	QuoteMintIdx  = 10 // #6 - Quote Mint

)

type LaunchpadSwapData struct {
	AmountIn         uint64 `json:"amount_in"`
	MinimumAmountOut uint64 `json:"minimum_amount_out"`
	ShareFeeRate     uint64 `json:"share_fee_rate"`
}

// parseLaunchpadTransaction 解析AMM交易事件
func (b *Bot) parseLaunchpadTransaction(accountList []string, innerInstructions []*yellowstone_geyser_pb.InnerInstructions, mintAddress string, userAddress solana.PublicKey, isBuy bool, _signature string) *global.LaunchpadTradeEventInfo {
	var eventBinaries [][]byte
	var err error
	for _, parsedIx := range innerInstructions {
		for _, ix := range parsedIx.Instructions {
			if accountList[ix.ProgramIdIndex] != core.RAYDIUM_LAUNCHPAD_PROGRAM_ID.String() {
				continue
			}

			var ixData = ix.Data
			if len(ixData) < 8 {
				continue
			}

			eventBase64 := base64.StdEncoding.EncodeToString(ixData[8:])
			var eventBinary []byte
			if eventBinary, err = base64.StdEncoding.DecodeString(eventBase64); err != nil {
				return nil
			}
			eventBinaries = append(eventBinaries, eventBinary)
		}
	}

	events, err := raydium_launchpad.ParseEventsFromEmitCPI(eventBinaries)
	if err != nil {
		logger.Errorf("解析AMM事件失败: %v", err)
		return nil
	}

	// 遍历解析出的事件
	for _, event := range events {

		// 检查是否是买入或卖出事件
		switch event.Name {
		case "TradeEvent":

			if tradeEvent, ok := event.Data.(*raydium_launchpad.TradeEventEventData); ok {

				// 创建交易事件信息对象
				_baseReserve := tradeEvent.RealBaseAfter
				_realQuoteAfter := tradeEvent.RealQuoteAfter
				_realBaseAfter := tradeEvent.RealBaseAfter
				_virtualBase := tradeEvent.VirtualBase
				_virtualQuote := tradeEvent.VirtualQuote
				tradeDirection := tradeEvent.TradeDirection.String()

				_amountIn := tradeEvent.AmountIn
				_amountOut := tradeEvent.AmountOut
				if tradeDirection == "Sell" {
					_amountIn = tradeEvent.AmountOut
					_amountOut = tradeEvent.AmountIn
				}

				tradeInfo := &global.LaunchpadTradeEventInfo{
					UserAddress:  userAddress,
					SolAmount:    _amountIn,
					TokenAmount:  _amountOut,
					IsBuy:        tradeDirection == "Buy",
					MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
					PoolState:    tradeEvent.PoolState,
					BaseReserve:  _baseReserve,
					QuoteReserve: _realQuoteAfter,
					SolReserve:   _realQuoteAfter, // For launchpad, quote is usually SOL
					ReceivedAt:   time.Now(),
				}
				numerator := float64(_virtualQuote) + float64(_realQuoteAfter)
				denominator := (float64(_virtualBase) - float64(_realBaseAfter)) * 1e3

				tradeInfo.CurrentPrice = numerator / denominator

				logger.Debugf("解析到Launchpad[ %s] 交易事件: 用户=%s, 代币=%s,sol金额=%d,token数量=%d, 价格=%.8f,sol储备=%d, signature=%s",
					tradeDirection, tradeInfo.UserAddress, tradeInfo.MintAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.CurrentPrice, tradeInfo.SolReserve, _signature)

				return tradeInfo
			}

		case "PoolCreateEvent":
			if poolCreateEvent, ok := event.Data.(*raydium_launchpad.PoolCreateEventEventData); ok {
				logger.Debugf("解析到launchpad池创建事件: dev=%s, mint=%s, pool=%s",
					poolCreateEvent.Creator, mintAddress, poolCreateEvent.PoolState.String())
			}

		}
	}

	return nil
}

// WrapLaunchpadParse 解析raydium launchpad交易日志
func (b *Bot) WrapLaunchpadParse(out *yellowstone_geyser_pb.SubscribeUpdate, currentInstruction *yellowstone_geyser_pb.InnerInstruction, ctx context.Context) *global.LaunchpadTradeEventInfo {
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

	var mintAddress, userAddress string

	// 安全检查：确保数据长度足够

	data := currentInstruction.GetData()
	if len(data) < 8 {
		logger.Debugf("指令数据不足8字节，跳过 %s", _signature)
		return nil
	}

	// 安全地获取判别器
	var discriminator [8]byte
	copy(discriminator[:], data[:8])
	var isBuy bool = true
	accountList := b.GetAccountList(out)
	// 比较判别器
	switch discriminator {
	case raydium_launchpad.Instruction_BuyExactIn:
		userAddress = accountList[currentInstruction.Accounts[PayerIdx]] // #18 - User Source Owner
		mintAddress = accountList[currentInstruction.Accounts[BaseMintIdx]]

		if mintAddress == solana.SolMint.String() {
			logger.Debugf("跳过 SOL 交易 %s", _signature)
			return nil
		}

		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------买入------------判别器")
		logger.Debugf("-------------------------------------------")
	case raydium_launchpad.Instruction_SellExactIn:

		userAddress = accountList[currentInstruction.Accounts[0]]
		mintAddress = accountList[currentInstruction.Accounts[9]]
		if mintAddress == solana.SolMint.String() {
			logger.Debugf("跳过 SOL 交易 %s", _signature)
			return nil
		}
		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------卖出------------判别器")

		logger.Debugf("_signature: %s", _signature)
		logger.Debugf("-------------------------------------------")
		isBuy = false
	default:
		logger.Debug("未匹配判别器")
	}

	if mintAddress == "" {
		logger.Debugf("Launchpad未找到代币地址，跳过 %s", _signature)
		return nil
	}

	// 处理AMM交易信息
	userPubkey := solana.MustPublicKeyFromBase58(userAddress)
	innerInstructions := out.GetTransaction().Transaction.Meta.InnerInstructions

	tradeInfo := b.parseLaunchpadTransaction(accountList, innerInstructions, mintAddress, userPubkey, isBuy, _signature)
	botWallet := b.privateKey.PublicKey()

	if tradeInfo == nil {
		logger.Debugf("未找到交易信息，跳过 %s", _signature)
		return nil
	}
	tradeInfo.Router = "externalRouter"
	global.SetLaunchpadTradeInfo(mintAddress, tradeInfo)
	logger.Debugf("$$$$$$ externalRouter Launchpad $$$$$$$$$ 交易信息: mint=%s, 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f, signature=%s",
		mintAddress, tradeInfo.UserAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice, _signature)

	if userPubkey.String() == botWallet.String() {
		if tradeInfo.IsBuy {
			// 检查是否是自己的交易

			logger.Infof("买入交易: SOL=%d (%.4f), 代币=%d (%.2f), 价格=%.8f",
				tradeInfo.SolAmount, float64(tradeInfo.SolAmount)/SolDecimals,
				tradeInfo.TokenAmount, float64(tradeInfo.TokenAmount)/TokenDecimals,
				tradeInfo.CurrentPrice)
			// 设置入场价格
			global.SetTradeAssetOnce(
				mintAddress,
				solana.PublicKey{},
				botWallet.String(),
				tradeInfo.CurrentPrice,
				tradeInfo.TokenAmount,
				6,
			)
			assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				go queue.SendTokenToSellChannel(global.GetConfig().SellDelayTime, assetInfo, tradeInfo.CurrentPrice, "launchpad", "delay_sell", "延迟卖出")
			}
		} else {
			assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
			if ok {
				logger.Infof("bot 钱包卖出记录处理: %s, %v, %d, %d", mintAddress, ok, assetInfo.Amount, tradeInfo.TokenAmount)
				global.DelPendingHoldAssetWithAmount(mintAddress, tradeInfo.TokenAmount)
			}
		}
	}

	return tradeInfo
}

// RaydiumLaunchpadParse 解析Raydium Launchpad交易日志
func (b *Bot) RaydiumLaunchpadParse(out *yellowstone_geyser_pb.SubscribeUpdate, ctx context.Context) *global.LaunchpadTradeEventInfo {
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

	var mintAddress, userAddress string
	var isBuy bool = true
	var tradeInfo *global.LaunchpadTradeEventInfo
	accountList := b.GetAccountList(out)
	for _, instruction := range instructions {
		// 安全检查：确保账户数据不为空

		accounts := instruction.Accounts
		if len(accounts) < 15 {
			logger.Debugf("Not enough accounts in the instruction")
			continue
		}

		// 安全检查：确保数据长度足够
		data := instruction.GetData()
		if len(data) < 8 {
			logger.Debugf("指令数据不足8字节，跳过")
			continue
		}

		// 安全地获取判别器

		var discriminator [8]byte
		copy(discriminator[:], data[:8])
		// 比较判别器
		switch discriminator {
		case raydium_launchpad.Instruction_BuyExactIn:

			userAddress = accountList[accounts[PayerIdx]] // #18 - User Source Owner
			mintAddress = accountList[accounts[BaseMintIdx]]

			// Get balance changes
			//solChange := GetSwapBalances(txn, BaseVault, QuoteVault, accountList)
			//logger.Infof("mintAddress: %s, solAmount: %.8f, solReserve: %.8f", mintAddress, solChange.SolChangeAmount, solChange.SolReserve)

		case raydium_launchpad.Instruction_SellExactIn:

			// Extract account information using the optimized function
			_mintAddress, _poolState, _BaseVault, _QuoteVault, _userAddress := b.extractLaunchpadAccountInfo(accounts, accountList)
			mintAddress = _mintAddress
			userAddress = _userAddress
			isBuy = false
			logger.Debugf("naive Raydium Launchpad SellExactIn: User=%s, Mint=%s, AmmPool=%s, BaseVault=%s, QuoteVault=%s",
				_userAddress, _mintAddress, _poolState, _BaseVault, _QuoteVault)

			// Create trade info using the helper function

		default:
			logger.Debug("未匹配判别器")
		}
	}

	if mintAddress == "" {
		return nil

	}

	// 处理AMM交易信息
	userPk := solana.MustPublicKeyFromBase58(userAddress)
	innerInstructions := out.GetTransaction().Transaction.Meta.InnerInstructions

	tradeInfo = b.parseLaunchpadTransaction(accountList, innerInstructions, mintAddress, userPk, isBuy, _signature)

	// 处理AMM交易信息
	botWallet := b.privateKey.PublicKey()

	if tradeInfo != nil {
		global.SetLaunchpadTradeInfo(mintAddress, tradeInfo)
		logger.Debugf(" ********Naive Launchpad*******交易信息: 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f, signature=%s",
			tradeInfo.UserAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.IsBuy, tradeInfo.CurrentPrice, _signature)

		userPubkey := tradeInfo.UserAddress
		// 检查是否是自己的交易
		if userPubkey.String() == botWallet.String() {
			logger.Infof("bot 钱包交易记录处理: %s, %v", mintAddress, tradeInfo.IsBuy)
			if tradeInfo.IsBuy {

				logger.Infof("买入交易: SOL=%d , 代币=%d , 价格=%.8f",
					tradeInfo.SolAmount, tradeInfo.TokenAmount,
					tradeInfo.CurrentPrice)
				// 设置入场价格
				global.SetTradeAssetOnce(
					mintAddress,
					solana.PublicKey{},
					botWallet.String(),
					tradeInfo.CurrentPrice,
					tradeInfo.TokenAmount,
					6,
				)
				assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
				if ok {
					go queue.SendTokenToSellChannel(global.GetConfig().SellDelayTime, assetInfo, tradeInfo.CurrentPrice, "launchpad", "delay_sell", "延迟卖出")
				}
			} else {
				assetInfo, ok := global.GetPendingHoldAsset(mintAddress)
				if ok {
					logger.Infof("bot 钱包卖出记录处理: %s, %v, %d, %d", mintAddress, ok, assetInfo.Amount, tradeInfo.TokenAmount)
					global.DelPendingHoldAssetWithAmount(mintAddress, tradeInfo.TokenAmount)
				}
			}
		}

	}
	return tradeInfo
}

// RaydiumLaunchpadSell 处理 Raydium Launchpad 平台的代币卖出操作
func (b *Bot) RaydiumLaunchpadSell(tokenInfo queue.TokenInfo) {
	// 使用策略模式进行卖出
	logger.Infof("使用策略模式卖出代币: %s", tokenInfo.Mint.String())

	// 执行卖出策略
	b.ExecuteRaydiumLaunchpadSellOrder(tokenInfo)
}

// extractLaunchpadAccountInfo extracts and returns the account information from instruction accounts
func (b *Bot) extractLaunchpadAccountInfo(accounts []byte, accountList []string) (mintAddress, poolState, BaseVault, QuoteVault, userOwner string) {
	// First determine the version based on account count
	if len(accounts) >= 15 {
		// Extract accounts for V4
		poolState = accountList[accounts[PoolStateIdx]]   // #2 - Amm Id (Raydium Market)
		BaseVault = accountList[accounts[BaseVaultIdx]]   // #6 - Pool Coin Token Account (shifted due to target orders)
		QuoteVault = accountList[accounts[QuoteVaultIdx]] // #7 - Pool Pc Token Account (shifted due to target orders)
		userOwner = accountList[accounts[PayerIdx]]       // #18 - User Source Owner
		mintAddress = accountList[accounts[BaseMintIdx]]
	}
	return
}
