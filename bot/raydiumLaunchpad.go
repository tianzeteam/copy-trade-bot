package bot

import (
	"context"
	"fmt"
	"math"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mr-tron/base58"
	"github.com/near/borsh-go"

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

func RaydiumLaunchpadParse(tx *solana.Transaction) (*LaunchpadSwapData, error) {
	for _, inst := range tx.Message.Instructions {
		progID, err := tx.Message.Program(inst.ProgramIDIndex)
		if err != nil {
			continue
		}

		if progID.String() == core.RAYDIUM_LAUNCHPAD_PROGRAM_ID.String() {
			// Placeholder for instruction discriminator and data parsing
			// This will be updated once the correct discriminator and data layout are known.
			if len(inst.Data) > 8 {
				// Assuming the first 8 bytes are the discriminator
				// and the rest is the data.
				// The actual discriminator needs to be identified.
				// For now, we'll just try to decode the data part.
				data := inst.Data[8:]
				var swapData LaunchpadSwapData
				if err := bin.NewBinDecoder(data).Decode(&swapData); err == nil {
					return &swapData, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Raydium Launchpad swap instruction not found")
}

// RaydiumLaunchpadParse 解析Raydium Launchpad交易日志
func (b *Bot) RaydiumLaunchpadParse(out *yellowstone_geyser_pb.SubscribeUpdate, ctx context.Context) *global.AmmRaydiumV4TradeInfo {
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
	accountList := b.GetAccountList(out)
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
		case raydium_launchpad.Instruction_BuyExactIn:
			var swapData LaunchpadSwapData
			err := borsh.Deserialize(&swapData, data)
			if err != nil {
				return nil
			}
			logger.Debugf("BuyExactIn instruction found, signature: %s", _signature)
			accounts := instruction.Accounts
			if len(accounts) < 15 {
				logger.Debug("Not enough accounts in the instruction")
				return nil
			}

			// Extract account information using the optimized function
			mintAddress, ammId, openOrders, targetOrders, poolCoinTokenAccount, poolPcTokenAccount, serumMarket, userOwner, hasTargetOrders, version := b.extractLaunchpadAccountInfo(accounts, accountList)

			logger.Debugf("Raydium Launchpad BuyExactIn: User=%s, Market=%s, AmmPool=%s, poolCoinTokenAccount=%s, poolPcTokenAccount=%s",
				userOwner, serumMarket, ammId, poolCoinTokenAccount, poolPcTokenAccount)

			// Get balance changes
			solChange := GetSwapBalances(txn, poolCoinTokenAccount, poolPcTokenAccount, accountList)
			logger.Debugf("mintAddress: %s, solAmount: %.8f, solReserve: %.8f", mintAddress, solChange.SolChangeAmount, solChange.SolReserve)

			// Create trade info using the helper function
			tradeInfo = b.createTradeInfo(
				mintAddress, ammId, openOrders, targetOrders,
				poolCoinTokenAccount, poolPcTokenAccount, serumMarket,
				userOwner, _signature, solChange, version, hasTargetOrders, solChange.SolReserve)

		case raydium_launchpad.Instruction_SellExactIn:
			logger.Debugf("SellExactIn instruction found, signature: %s", _signature)
			accounts := instruction.Accounts
			if len(accounts) < 15 {
				logger.Warn("Not enough accounts in the instruction")
				return nil
			}

			// Get account list once
			accountList := b.GetAccountList(out)

			// Extract account information using the optimized function
			mintAddress, ammId, openOrders, targetOrders, poolCoinTokenAccount, poolPcTokenAccount, serumMarket, userOwner, hasTargetOrders, version := b.extractLaunchpadAccountInfo(accounts, accountList)

			// Extract mint address for tracking
			logger.Debugf("serumMarket: %s", serumMarket)

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
		// 检查是否是自己的交易
		if userPubkey.String() == botWallet.String() {
			if tradeInfo.IsBuy {

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
					go queue.SendTokenToSellChannel(global.GetConfig().SellDelayTime, assetInfo, entryPrice, "raydiumV4", "delay_sell", "延迟卖出")
				}
			} else {
				_, ok := global.GetPendingHoldAsset(mintAddress)
				if ok {
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
func (b *Bot) extractLaunchpadAccountInfo(accounts []byte, accountList []string) (mintAddress, ammId, openOrders, targetOrders, poolCoinTokenAccount, poolPcTokenAccount, serumMarket, userOwner string, hasTargetOrders bool, version uint32) {
	// First determine the version based on account count
	if len(accounts) >= 15 {
		// Extract accounts for V4
		ammId = accountList[accounts[AmmIdIdx]]               // #2 - Amm Id (Raydium Market)
		openOrders = accountList[accounts[OpenOrdersIdx]]     // #4 - Amm Open Orders
		targetOrders = accountList[accounts[TargetOrdersIdx]] // #5 - Amm Target Orders
		poolCoinTokenAccount = accountList[accounts[5]]       // #6 - Pool Coin Token Account (shifted due to target orders)
		poolPcTokenAccount = accountList[accounts[6]]         // #7 - Pool Pc Token Account (shifted due to target orders)
		serumMarket = accountList[accounts[8]]                // #9 - Serum Market (shifted due to target orders)
		userOwner = accountList[accounts[UserOwnerIdx]]       // #18 - User Source Owner
		mintAddress = accountList[accounts[BaseMintIdx]]
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
