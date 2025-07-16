package bot

import (
	"context"
	"time"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mr-tron/base58"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"
	"copytrade/raydium_cp_swap"
	yellowstone_geyser_pb "copytrade/yellowstone_geyser/pb"
)

// Constants for account indices in Raydium V4 instructions
const (

	// Constants for account indices in Raydium V4 instructions
	CpmmPayerIdx           = 0  // #1 - Payer
	CpmmPoolStateIdx       = 3  // #2 - Pool State
	CpmmInputTokenMintIdx  = 10 // #10 - Input Token Mint
	CpmmOutputTokenMintIdx = 11 // #11 - Output Token Mint

)

type CpmmSwapData struct {
	AmountIn         uint64 `json:"amount_in"`
	MinimumAmountOut uint64 `json:"minimum_amount_out"`
	ShareFeeRate     uint64 `json:"share_fee_rate"`
}

// parseLaunchpadTransaction 解析AMM交易事件
func (b *Bot) parseCpmmTransaction(logMessages []string, mintAddress string, userAddress solana.PublicKey, isBuy bool, _signature string) *global.CPMMTradeEventInfo {
	/* var eventBinaries [][]byte
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
	} */

	events, err := raydium_cp_swap.DecodeEventsInLogs(logMessages)
	if err != nil {
		logger.Errorf("解析AMM事件失败: %v", err)
		return nil
	}

	// 遍历解析出的事件
	for _, event := range events {

		// 检查是否是买入或卖出事件
		switch event.Name {
		case "SwapEvent":

			if tradeEvent, ok := event.Data.(*raydium_cp_swap.SwapEventEventData); ok {

				// 创建交易事件信息对象
				//baseInput := tradeEvent.BaseInput
				_inputVaultBefore := tradeEvent.InputVaultBefore
				_outputVaultBefore := tradeEvent.OutputVaultBefore
				_inputAmount := tradeEvent.InputAmount
				_outputAmount := tradeEvent.OutputAmount

				solReserve := _inputVaultBefore + _inputAmount
				tokenReserve := _outputVaultBefore + _outputAmount
				if !isBuy {
					solReserve = _outputVaultBefore + _outputAmount
					tokenReserve = _inputVaultBefore + _inputAmount
					_inputAmount = tradeEvent.OutputAmount
					_outputAmount = tradeEvent.InputAmount
				}

				tradeInfo := &global.CPMMTradeEventInfo{
					UserAddress:  userAddress,
					SolAmount:    _inputAmount,
					TokenAmount:  _outputAmount,
					IsBuy:        isBuy,
					MintAddress:  solana.MustPublicKeyFromBase58(mintAddress),
					PoolState:    tradeEvent.PoolId,
					BaseReserve:  tokenReserve,
					QuoteReserve: solReserve,
					SolReserve:   solReserve,
					ReceivedAt:   time.Now(),
				}
				numerator := float64(solReserve)
				denominator := (float64(tokenReserve)) * 1e3

				tradeInfo.CurrentPrice = numerator / denominator

				logger.Debugf("解析到CPMM[ %s] 交易事件: 用户=%s, 代币=%s,sol金额=%d,token数量=%d, 价格=%.8f,sol储备=%d, signature=%s",
					isBuy, tradeInfo.UserAddress, tradeInfo.MintAddress, tradeInfo.SolAmount, tradeInfo.TokenAmount, tradeInfo.CurrentPrice, tradeInfo.SolReserve, _signature)

				return tradeInfo
			}

		case "LpChangeEvent":
			if poolCreateEvent, ok := event.Data.(*raydium_cp_swap.LpChangeEventEventData); ok {
				logger.Debugf("解析到CPMM池创建事件: pool changeType=%d, mint=%s, pool=%s",
					poolCreateEvent.ChangeType, mintAddress, poolCreateEvent.PoolId.String())
			}

		}
	}

	return nil
}

// WrapLaunchpadParse 解析raydium launchpad交易日志
func (b *Bot) WrapCpmmParse(out *yellowstone_geyser_pb.SubscribeUpdate, currentInstruction *yellowstone_geyser_pb.InnerInstruction, ctx context.Context) *global.CPMMTradeEventInfo {
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
	case raydium_cp_swap.Instruction_SwapBaseInput:
		userAddress = accountList[currentInstruction.Accounts[CpmmPayerIdx]] // #18 - User Source Owner
		inputTokenAddress := accountList[currentInstruction.Accounts[CpmmInputTokenMintIdx]]
		outputTokenAddress := accountList[currentInstruction.Accounts[CpmmOutputTokenMintIdx]]

		if inputTokenAddress == solana.SolMint.String() {
			isBuy = true
			mintAddress = outputTokenAddress
		} else {
			isBuy = false
			mintAddress = inputTokenAddress
		}

		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------买入------------判别器")
		logger.Debugf("-------------------------------------------")
	case raydium_cp_swap.Instruction_SwapBaseOutput:

		userAddress = accountList[currentInstruction.Accounts[CpmmPayerIdx]] // #18 - User Source Owner
		inputTokenAddress := accountList[currentInstruction.Accounts[CpmmInputTokenMintIdx]]
		outputTokenAddress := accountList[currentInstruction.Accounts[CpmmOutputTokenMintIdx]]

		if inputTokenAddress == solana.SolMint.String() {
			isBuy = true
			mintAddress = outputTokenAddress
		} else {
			isBuy = false
			mintAddress = inputTokenAddress
		}
		//pool := accountList[currentInstruction.Accounts[0]]
		logger.Debugf("匹配-------卖出------------判别器")

		logger.Debugf("_signature: %s", _signature)
		logger.Debugf("-------------------------------------------")
	default:
		logger.Debug("未匹配判别器")
	}

	if mintAddress == "" {
		logger.Debugf("Launchpad未找到代币地址，跳过 %s", _signature)
		return nil
	}

	// 处理AMM交易信息
	userPubkey := solana.MustPublicKeyFromBase58(userAddress)
	//innerInstructions := out.GetTransaction().Transaction.Meta.InnerInstructions

	tradeInfo := b.parseCpmmTransaction(logMessages, mintAddress, userPubkey, isBuy, _signature)
	botWallet := b.privateKey.PublicKey()

	if tradeInfo == nil {
		logger.Debugf("未找到交易信息，跳过 %s", _signature)
		return nil
	}
	tradeInfo.Router = "externalRouter"
	global.SetCpmmTradeInfo(mintAddress, tradeInfo)
	logger.Infof("$$$$$$ externalRouter CPMM $$$$$$$$$ 交易信息: mint=%s, 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f, signature=%s",
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
func (b *Bot) RaydiumCpmmParse(out *yellowstone_geyser_pb.SubscribeUpdate, ctx context.Context) *global.CPMMTradeEventInfo {
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
	var tradeInfo *global.CPMMTradeEventInfo
	accountList := b.GetAccountList(out)
	for _, instruction := range instructions {
		// 安全检查：确保账户数据不为空

		accounts := instruction.Accounts
		if len(accounts) < 13 {
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
		case raydium_cp_swap.Instruction_SwapBaseInput:

			userAddress = accountList[accounts[CpmmPayerIdx]] // #18 - User Source Owner

			inputTokenAddress := accountList[accounts[CpmmInputTokenMintIdx]]
			outputTokenAddress := accountList[accounts[CpmmOutputTokenMintIdx]]

			if inputTokenAddress == solana.SolMint.String() {
				isBuy = true
				mintAddress = outputTokenAddress
			} else {
				isBuy = false
				mintAddress = inputTokenAddress
			}
			// Get balance changes
			//solChange := GetSwapBalances(txn, BaseVault, QuoteVault, accountList)
			//logger.Infof("mintAddress: %s, solAmount: %.8f, solReserve: %.8f", mintAddress, solChange.SolChangeAmount, solChange.SolReserve)

		case raydium_cp_swap.Instruction_SwapBaseOutput:

			userAddress = accountList[accounts[CpmmPayerIdx]] // #18 - User Source Owner
			inputTokenAddress := accountList[accounts[CpmmInputTokenMintIdx]]
			outputTokenAddress := accountList[accounts[CpmmOutputTokenMintIdx]]

			if inputTokenAddress == solana.SolMint.String() {
				isBuy = true
				mintAddress = outputTokenAddress
			} else {
				isBuy = false
				mintAddress = inputTokenAddress
			}
		default:
			logger.Debug("未匹配判别器")
		}
	}

	if mintAddress == "" {
		return nil

	}

	// 处理AMM交易信息
	userPk := solana.MustPublicKeyFromBase58(userAddress)
	//innerInstructions := out.GetTransaction().Transaction.Meta.InnerInstructions

	tradeInfo = b.parseCpmmTransaction(logMessages, mintAddress, userPk, isBuy, _signature)

	// 处理AMM交易信息
	botWallet := b.privateKey.PublicKey()

	if tradeInfo != nil {
		global.SetCpmmTradeInfo(mintAddress, tradeInfo)
		logger.Debugf(" ********Naive CPMM*******交易信息: 用户=%s, SOL=%d, 代币=%d, 是否买入=%v, 价格=%.8f, signature=%s",
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
					go queue.SendTokenToSellChannel(global.GetConfig().SellDelayTime, assetInfo, tradeInfo.CurrentPrice, "raydiumV4", "delay_sell", "延迟卖出")
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
func (b *Bot) RaydiumCpmmSell(tokenInfo queue.TokenInfo) {
	// 使用策略模式进行卖出
	logger.Infof("使用策略模式卖出代币: %s", tokenInfo.Mint.String())

	// 执行卖出策略
	//b.ExecuteRaydiumCpmmSellOrder(tokenInfo)
}

// extractLaunchpadAccountInfo extracts and returns the account information from instruction accounts
func (b *Bot) extractCpmmAccountInfo(accounts []byte, accountList []string) (mintAddress, poolState, BaseVault, QuoteVault, userOwner string) {
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
