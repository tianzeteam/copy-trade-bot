package shredstream

import (
	"bytes"
	"fmt"
	"time"

	"copytrade/constants"
	"copytrade/core"
	"copytrade/global"
	"copytrade/logger"
	"copytrade/queue"

	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// PumpFun 指令 discriminator
var (
	PumpBuyDiscriminator  = [8]byte{102, 6, 61, 18, 1, 218, 235, 234}
	PumpSellDiscriminator = [8]byte{51, 230, 133, 164, 1, 127, 131, 173}

	PumpBuyExactSolInDiscriminator = [8]byte{56, 252, 116, 8, 158, 223, 205, 95}
)

// PumpBuyParams Buy 指令参数
type PumpBuyParams struct {
	Amount     uint64
	MaxSolCost uint64
}

// PumpSellParams Sell 指令参数
type PumpSellParams struct {
	Amount    uint64
	MinSolOut uint64
}

// PumpBuyExactSolInParams BuyExactSolIn 指令参数
// 与 Buy 不同: Buy 是指定代币数量，BuyExactSolIn 是指定 SOL 数量
type PumpBuyExactSolInParams struct {
	SpendableSolIn uint64 // 花费的 SOL 数量
	MinTokensOut   uint64 // 最小获得的代币数量
}

// ParsePumpFunTransaction 从 ShredStream 事件解析 PumpFun 交易
// 注意: ShredStream 获取的是未执行的交易，没有执行日志
// 因此只能从指令数据解析基本信息，无法获取实际的储备量和价格
func ParsePumpFunTransaction(event *ShredEvent) *global.TradeEventInfo {
	if event == nil || event.Transaction == nil {
		return nil
	}

	tx := event.Transaction
	msg := tx.Message

	// 获取账户列表
	accountKeys := msg.AccountKeys
	if len(accountKeys) == 0 {
		return nil
	}

	// 查找 PumpFun 指令

	for _, ix := range msg.Instructions {

		if int(ix.ProgramIDIndex) >= len(accountKeys) {
			continue
		}

		programID := accountKeys[ix.ProgramIDIndex]
		if programID.String() != core.PUMP_PROGRAM_ID.String() {
			continue
		}

		// 解析指令
		tradeInfo := parseInstruction(ix, accountKeys, event)
		if tradeInfo != nil {
			// ds清仓
			if global.GetConfig().DevSell {
				if tradeInfo.UserAddress == tradeInfo.Creator.String() && !tradeInfo.IsBuy {
					assetInfo, ok := global.GetPendingHoldAsset(tradeInfo.MintAddress)
					if ok {
						currentTradeInfo := global.GetTradeInfo(tradeInfo.MintAddress)
						if currentTradeInfo != nil {
							queue.SendTokenToSellChannel(0, assetInfo, assetInfo.Amount, currentTradeInfo.CurrentPrice, tradeInfo.PumpVersion, "pumpfun", "DevSell", "ds清仓")
						}
					}
				}
			}
			return tradeInfo
		}
	}

	return nil
}

// parseInstruction 解析 PumpFun 指令
func parseInstruction(ix solana.CompiledInstruction, accountKeys []solana.PublicKey, event *ShredEvent) *global.TradeEventInfo {
	if len(ix.Data) < 8 {
		return nil
	}

	discriminator := [8]byte{}
	copy(discriminator[:], ix.Data[:8])

	switch discriminator {
	case PumpBuyDiscriminator:
		return parseBuyInstruction(ix, accountKeys, event)
	case PumpBuyExactSolInDiscriminator:
		return parseBuyExactSolInInstruction(ix, accountKeys, event)
	case PumpSellDiscriminator:
		return parseSellInstruction(ix, accountKeys, event)
	}

	return nil
}

// parseBuyInstruction 解析 Buy 指令
// Buy 账户布局 (pump/buy.go):
// [0] = global
// [1] = fee_recipient (WRITE)
// [2] = mint
// [3] = bonding_curve (WRITE)
// [4] = associated_bonding_curve (WRITE)
// [5] = associated_user (WRITE)
// [6] = user (WRITE, SIGNER)
// [7] = system_program
// [8] = token_program
// [9] = creator_vault (WRITE)
// [10] = event_authority
// [11] = program
func parseBuyInstruction(ix solana.CompiledInstruction, accountKeys []solana.PublicKey, event *ShredEvent) *global.TradeEventInfo {
	// 至少需要 7 个账户索引
	if len(ix.Accounts) < 7 {
		logger.Debugf("ShredStream: Buy instruction has insufficient accounts: %d", len(ix.Accounts))
		return nil
	}

	// 解析参数
	params, err := decodeBuyParams(ix.Data[8:])
	if err != nil {
		logger.Debugf("ShredStream: Failed to decode buy params: %v", err)
		return nil
	}

	// 提取账户
	mintIndex := ix.Accounts[2]
	userIndex := ix.Accounts[6]
	feeRecipientIndex := ix.Accounts[1]

	if int(mintIndex) >= len(accountKeys) || int(userIndex) >= len(accountKeys) {
		return nil
	}

	mintAddress := accountKeys[mintIndex]
	userAddress := accountKeys[userIndex]

	var feeRecipient solana.PublicKey
	if int(feeRecipientIndex) < len(accountKeys) {
		feeRecipient = accountKeys[feeRecipientIndex]
	}

	// 提取 creator (如果存在)
	var creator solana.PublicKey
	if len(ix.Accounts) > 9 {
		creatorVaultIndex := ix.Accounts[9]
		if int(creatorVaultIndex) < len(accountKeys) {
			creator = accountKeys[creatorVaultIndex]
		}
	}
	latestTradeInfo := global.GetTradeInfo(mintAddress.String())
	if latestTradeInfo == nil {
		return nil
	}
	// 创建交易信息
	// 注意: ShredStream 无法获取执行后的储备量和价格
	// 这些需要从后续的 gRPC 事件或链上查询获取
	tradeInfo := &global.TradeEventInfo{
		PumpVersion:  constants.PumpVersionV1, // 默认 V1，后续可通过账户判断
		MintAddress:  mintAddress.String(),
		UserAddress:  userAddress.String(),
		TokenAmount:  params.Amount,
		SolAmount:    params.MaxSolCost, // 注意: 这是最大成本，非实际成本
		IsBuy:        true,
		FeeRecipient: feeRecipient,
		Creator:      creator,
		TimeStamp:    time.Now().Unix(),
		TradeTime:    time.Now().Format("2006-01-02 15:04:05"),
		ReceivedAt:   event.ReceivedAt,
		// 储备量信息无法从未执行的交易获取
		VirtualSolReserves:   latestTradeInfo.VirtualSolReserves,
		VirtualTokenReserves: latestTradeInfo.VirtualTokenReserves,
		RealSolReserves:      latestTradeInfo.RealSolReserves,
		RealTokenReserves:    latestTradeInfo.RealTokenReserves,
		CurrentPrice:         latestTradeInfo.CurrentPrice, // 需要后续更新
	}

	// 检测是否为 PumpV2 (Token2022)
	for _, accIdx := range ix.Accounts {
		if int(accIdx) < len(accountKeys) {
			if accountKeys[accIdx].String() == solana.Token2022ProgramID.String() {
				tradeInfo.PumpVersion = constants.PumpVersionV2
				break
			}
		}
	}

	logger.Debugf("ShredStream: Parsed PumpFun Buy - mint=%s, user=%s, amount=%d, maxSol=%d",
		mintAddress.String(), userAddress.String(), params.Amount, params.MaxSolCost)

	return tradeInfo
}

// parseSellInstruction 解析 Sell 指令
// Sell 账户布局 (pump/sell.go):
// [0] = global
// [1] = fee_recipient (WRITE)
// [2] = mint
// [3] = bonding_curve (WRITE)
// [4] = associated_bonding_curve (WRITE)
// [5] = associated_user (WRITE)
// [6] = user (WRITE, SIGNER)
// [7] = system_program
// [8] = associated_token_program
// [9] = token_program
// [10] = creator_vault (WRITE)
// [11] = event_authority
// [12] = program
func parseSellInstruction(ix solana.CompiledInstruction, accountKeys []solana.PublicKey, event *ShredEvent) *global.TradeEventInfo {
	// 至少需要 7 个账户索引
	if len(ix.Accounts) < 7 {
		logger.Debugf("ShredStream: Sell instruction has insufficient accounts: %d", len(ix.Accounts))
		return nil
	}

	// 解析参数
	params, err := decodeSellParams(ix.Data[8:])
	if err != nil {
		logger.Debugf("ShredStream: Failed to decode sell params: %v", err)
		return nil
	}

	// 提取账户
	mintIndex := ix.Accounts[2]
	userIndex := ix.Accounts[6]
	feeRecipientIndex := ix.Accounts[1]

	if int(mintIndex) >= len(accountKeys) || int(userIndex) >= len(accountKeys) {
		return nil
	}

	mintAddress := accountKeys[mintIndex]
	userAddress := accountKeys[userIndex]

	var feeRecipient solana.PublicKey
	if int(feeRecipientIndex) < len(accountKeys) {
		feeRecipient = accountKeys[feeRecipientIndex]
	}

	latestTradeInfo := global.GetTradeInfo(mintAddress.String())
	if latestTradeInfo == nil {
		return nil
	}

	// 创建交易信息
	tradeInfo := &global.TradeEventInfo{
		PumpVersion:          constants.PumpVersionV1,
		MintAddress:          mintAddress.String(),
		UserAddress:          userAddress.String(),
		TokenAmount:          params.Amount,
		SolAmount:            params.MinSolOut, // 注意: 这是最小收入，非实际收入
		IsBuy:                false,
		FeeRecipient:         feeRecipient,
		TimeStamp:            time.Now().Unix(),
		TradeTime:            time.Now().Format("2006-01-02 15:04:05"),
		ReceivedAt:           event.ReceivedAt,
		VirtualSolReserves:   latestTradeInfo.VirtualSolReserves,
		VirtualTokenReserves: latestTradeInfo.VirtualTokenReserves,
		RealSolReserves:      latestTradeInfo.RealSolReserves,
		RealTokenReserves:    latestTradeInfo.RealTokenReserves,
		CurrentPrice:         latestTradeInfo.CurrentPrice,
	}

	// 检测是否为 PumpV2
	for _, accIdx := range ix.Accounts {
		if int(accIdx) < len(accountKeys) {
			if accountKeys[accIdx].String() == solana.Token2022ProgramID.String() {
				tradeInfo.PumpVersion = constants.PumpVersionV2
				break
			}
		}
	}

	logger.Debugf("ShredStream: Parsed PumpFun Sell - mint=%s, user=%s, amount=%d, minSol=%d",
		mintAddress.String(), userAddress.String(), params.Amount, params.MinSolOut)

	return tradeInfo
}

// decodeBuyParams 解码 Buy 指令参数
func decodeBuyParams(data []byte) (*PumpBuyParams, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("invalid buy instruction data length: %d", len(data))
	}

	decoder := ag_binary.NewBorshDecoder(data)
	params := &PumpBuyParams{}

	if err := decoder.Decode(&params.Amount); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&params.MaxSolCost); err != nil {
		return nil, err
	}

	return params, nil
}

// decodeSellParams 解码 Sell 指令参数
func decodeSellParams(data []byte) (*PumpSellParams, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("invalid sell instruction data length: %d", len(data))
	}

	decoder := ag_binary.NewBorshDecoder(data)
	params := &PumpSellParams{}

	if err := decoder.Decode(&params.Amount); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&params.MinSolOut); err != nil {
		return nil, err
	}

	return params, nil
}

// decodeBuyExactSolInParams 解码 BuyExactSolIn 指令参数
func decodeBuyExactSolInParams(data []byte) (*PumpBuyExactSolInParams, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("invalid buy_exact_sol_in instruction data length: %d", len(data))
	}

	decoder := ag_binary.NewBorshDecoder(data)
	params := &PumpBuyExactSolInParams{}

	if err := decoder.Decode(&params.SpendableSolIn); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&params.MinTokensOut); err != nil {
		return nil, err
	}

	return params, nil
}

// parseBuyExactSolInInstruction 解析 BuyExactSolIn 指令
// BuyExactSolIn 账户布局与 Buy 相同 (pumpV2/instructions.go):
// [0] = global
// [1] = fee_recipient (WRITE)
// [2] = mint
// [3] = bonding_curve (WRITE)
// [4] = associated_bonding_curve (WRITE)
// [5] = associated_user (WRITE)
// [6] = user (WRITE, SIGNER)
// [7] = system_program
// [8] = token_program
// [9] = creator_vault (WRITE)
// [10] = event_authority
// [11] = program
// [12] = global_volume_accumulator (WRITE)
// [13] = user_volume_accumulator (WRITE)
// [14] = fee_config
// [15] = fee_program
func parseBuyExactSolInInstruction(ix solana.CompiledInstruction, accountKeys []solana.PublicKey, event *ShredEvent) *global.TradeEventInfo {
	// 至少需要 7 个账户索引
	if len(ix.Accounts) < 7 {
		logger.Debugf("ShredStream: BuyExactSolIn instruction has insufficient accounts: %d", len(ix.Accounts))
		return nil
	}

	// 解析参数
	params, err := decodeBuyExactSolInParams(ix.Data[8:])
	if err != nil {
		logger.Debugf("ShredStream: Failed to decode buy_exact_sol_in params: %v", err)
		return nil
	}

	// 提取账户 (与 Buy 相同的布局)
	mintIndex := ix.Accounts[2]
	userIndex := ix.Accounts[6]
	feeRecipientIndex := ix.Accounts[1]

	if int(mintIndex) >= len(accountKeys) || int(userIndex) >= len(accountKeys) {
		return nil
	}

	mintAddress := accountKeys[mintIndex]
	userAddress := accountKeys[userIndex]

	var feeRecipient solana.PublicKey
	if int(feeRecipientIndex) < len(accountKeys) {
		feeRecipient = accountKeys[feeRecipientIndex]
	}

	// 提取 creator (如果存在)
	var creator solana.PublicKey
	if len(ix.Accounts) > 9 {
		creatorVaultIndex := ix.Accounts[9]
		if int(creatorVaultIndex) < len(accountKeys) {
			creator = accountKeys[creatorVaultIndex]
		}
	}

	// 创建交易信息
	// 注意: BuyExactSolIn 是指定 SOL 数量买入，与 Buy 不同
	// SolAmount = SpendableSolIn (实际花费的 SOL)
	// TokenAmount = MinTokensOut (最小获得的代币，实际可能更多)
	latestTradeInfo := global.GetTradeInfo(mintAddress.String())
	if latestTradeInfo == nil {
		return nil
	}
	tradeInfo := &global.TradeEventInfo{
		PumpVersion:  constants.PumpVersionV1, // BuyExactSolIn 是 pumpV2 的指令
		MintAddress:  mintAddress.String(),
		UserAddress:  userAddress.String(),
		TokenAmount:  params.MinTokensOut,   // 最小获得的代币数量
		SolAmount:    params.SpendableSolIn, // 花费的 SOL 数量
		IsBuy:        true,
		FeeRecipient: feeRecipient,
		Creator:      creator,
		TimeStamp:    time.Now().Unix(),
		TradeTime:    time.Now().Format("2006-01-02 15:04:05"),
		ReceivedAt:   event.ReceivedAt,
		// 储备量信息无法从未执行的交易获取
		VirtualSolReserves:   latestTradeInfo.VirtualSolReserves,
		VirtualTokenReserves: latestTradeInfo.VirtualTokenReserves,
		RealSolReserves:      latestTradeInfo.RealSolReserves,
		RealTokenReserves:    latestTradeInfo.RealTokenReserves,
		CurrentPrice:         latestTradeInfo.CurrentPrice, // 需要后续更新
	}

	// 检测是否为 PumpV2 (Token2022)
	for _, accIdx := range ix.Accounts {
		if int(accIdx) < len(accountKeys) {
			if accountKeys[accIdx].String() == solana.Token2022ProgramID.String() {
				tradeInfo.PumpVersion = constants.PumpVersionV2
				break
			}
		}
	}

	logger.Debugf("ShredStream: Parsed PumpFun BuyExactSolIn - mint=%s, user=%s, solIn=%d, minTokens=%d",
		mintAddress.String(), userAddress.String(), params.SpendableSolIn, params.MinTokensOut)

	return tradeInfo
}

// ContainsPumpProgram 检查账户列表是否包含 PumpFun 程序
func ContainsPumpProgram(accountKeys []solana.PublicKey) bool {
	pumpProgramID := core.PUMP_PROGRAM_ID.String()
	for _, key := range accountKeys {
		if key.String() == pumpProgramID {
			return true
		}
	}
	return false
}

// FilterPumpFunEvents 从 ShredStream 事件中过滤出 PumpFun 相关交易
func FilterPumpFunEvents(events []*ShredEvent) []*ShredEvent {
	var filtered []*ShredEvent

	for _, event := range events {
		if event == nil || event.Transaction == nil {
			continue
		}

		accountKeys := event.Transaction.Message.AccountKeys
		if ContainsPumpProgram(accountKeys) {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// GetTransactionAccounts 获取交易的账户地址列表
func GetTransactionAccounts(tx *solana.Transaction) []string {
	if tx == nil {
		return nil
	}

	accounts := make([]string, len(tx.Message.AccountKeys))
	for i, key := range tx.Message.AccountKeys {
		accounts[i] = key.String()
	}
	return accounts
}

// HasPumpFunInstruction 检查交易是否包含 PumpFun 指令
func HasPumpFunInstruction(tx *solana.Transaction) bool {
	if tx == nil {
		return false
	}

	msg := tx.Message
	accountKeys := msg.AccountKeys
	pumpProgramID := core.PUMP_PROGRAM_ID.String()

	for _, ix := range msg.Instructions {
		if int(ix.ProgramIDIndex) >= len(accountKeys) {
			continue
		}

		if accountKeys[ix.ProgramIDIndex].String() == pumpProgramID {
			if len(ix.Data) >= 8 {
				discriminator := [8]byte{}
				copy(discriminator[:], ix.Data[:8])

				if bytes.Equal(discriminator[:], PumpBuyDiscriminator[:]) ||
					bytes.Equal(discriminator[:], PumpBuyExactSolInDiscriminator[:]) ||
					bytes.Equal(discriminator[:], PumpSellDiscriminator[:]) {
					return true
				}
			}
		}
	}

	return false
}
