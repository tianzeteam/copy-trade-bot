package pumpV3

// 判别码取自官方 idl/pump.json 与 idl/pump_fees.json（2026-09-14）。
// Anchor 判别码 = sha256("account:<ns>:<name>") 或 sha256("global:<name>") 的前 8 字节。

// 账户判别码。
var (
	AccountBondingCurve            = [8]byte{23, 183, 248, 55, 96, 216, 172, 96}
	AccountGlobal                  = [8]byte{167, 232, 232, 177, 200, 108, 114, 127}
	AccountFeeConfig               = [8]byte{143, 52, 146, 187, 219, 123, 76, 155}
	AccountSharingConfig           = [8]byte{216, 74, 9, 0, 56, 140, 93, 75}
	AccountGlobalVolumeAccumulator = [8]byte{202, 42, 246, 43, 142, 190, 30, 255}
	AccountUserVolumeAccumulator   = [8]byte{86, 255, 112, 14, 102, 53, 154, 250}
	AccountQuoteControl            = [8]byte{56, 244, 35, 238, 193, 213, 162, 201}
)

// 事件判别码。
var (
	EventTradeEvent                 = [8]byte{189, 219, 127, 211, 78, 230, 97, 238}
	EventCreateEvent                = [8]byte{27, 114, 169, 77, 222, 235, 99, 118}
	EventCompleteEvent              = [8]byte{95, 114, 97, 156, 212, 46, 152, 8}
	EventCompletePumpAmmMigration   = [8]byte{189, 233, 93, 185, 92, 148, 234, 148}
	EventCollectCreatorFee          = [8]byte{122, 2, 127, 1, 14, 191, 12, 175}
	EventClaimCashback              = [8]byte{226, 214, 246, 33, 7, 242, 147, 229}
	EventDistributeCreatorFees      = [8]byte{165, 55, 129, 112, 4, 179, 202, 40}
	EventDistributeFeeToHolders     = [8]byte{227, 190, 215, 206, 176, 180, 165, 132}
	EventMinimumDistributableFee    = [8]byte{168, 216, 132, 239, 235, 182, 49, 52}
	EventSetCreator                 = [8]byte{237, 52, 123, 37, 245, 251, 72, 210}
	EventSetParams                  = [8]byte{223, 195, 159, 246, 62, 48, 143, 131}
	EventUpdateGlobalAuthority      = [8]byte{182, 195, 137, 42, 35, 206, 207, 247}
	EventUpdateMayhemVirtualParams  = [8]byte{117, 123, 228, 182, 161, 168, 220, 214}
	EventUpdateCreatorFeeConfig     = [8]byte{152, 198, 124, 124, 106, 246, 127, 191}
	EventReservedFeeRecipients      = [8]byte{43, 188, 250, 18, 221, 75, 187, 95}
	EventExtendAccount              = [8]byte{97, 97, 215, 144, 93, 146, 22, 124}
	EventInitUserVolumeAccumulator  = [8]byte{134, 36, 13, 72, 232, 101, 130, 216}
	EventSyncUserVolumeAccumulator  = [8]byte{197, 122, 167, 124, 116, 81, 91, 255}
	EventCloseUserVolumeAccumulator = [8]byte{146, 159, 189, 172, 146, 88, 56, 244}
	EventClaimTokenIncentives       = [8]byte{79, 172, 246, 49, 205, 91, 206, 232}
	EventMigrateBondingCurveCreator = [8]byte{155, 167, 104, 220, 213, 108, 243, 3}
	EventSetMetaplexCreator         = [8]byte{142, 203, 6, 32, 127, 105, 191, 162}
	EventAdminSetIdlAuthority       = [8]byte{245, 59, 70, 34, 75, 185, 109, 92}
	EventAdminUpdateTokenIncentives = [8]byte{147, 250, 108, 120, 247, 29, 67, 222}
	EventAdminCto                   = [8]byte{110, 124, 226, 98, 170, 255, 17, 120}
	EventAddQuoteControlMint        = [8]byte{164, 175, 87, 74, 88, 45, 33, 62}
	EventRemoveQuoteControlMint     = [8]byte{46, 33, 86, 134, 0, 213, 209, 48}
	EventSetQuoteControlAdmin       = [8]byte{74, 248, 141, 69, 202, 81, 30, 247}
)

// 交易指令判别码。
//
// 旧版（legacy）指令仍然可用，但只能交易 SOL 配对币；
// USDC 配对币必须走 *_v2 统一接口。
var (
	// 旧版指令
	InstructionBuy                        = [8]byte{102, 6, 61, 18, 1, 218, 235, 234}
	InstructionSell                       = [8]byte{51, 230, 133, 164, 1, 127, 131, 173}
	InstructionBuyExactSolIn              = [8]byte{56, 252, 116, 8, 158, 223, 205, 95}
	InstructionCreate                     = [8]byte{24, 30, 200, 40, 5, 28, 7, 119}
	InstructionCreateV2                   = [8]byte{214, 144, 76, 236, 95, 139, 49, 180}
	InstructionMigrate                    = [8]byte{155, 234, 231, 146, 236, 158, 162, 30}
	InstructionExtendAccount              = [8]byte{234, 102, 194, 203, 150, 72, 62, 229}
	InstructionInitialize                 = [8]byte{175, 175, 109, 31, 13, 152, 155, 237}
	InstructionSetParams                  = [8]byte{27, 234, 178, 52, 147, 2, 187, 141}
	InstructionSetCreator                 = [8]byte{254, 148, 255, 112, 207, 142, 170, 165}
	InstructionSetMetaplexCreator         = [8]byte{138, 96, 174, 217, 48, 85, 197, 246}
	InstructionUpdateGlobalAuthority      = [8]byte{227, 181, 74, 196, 208, 21, 97, 213}
	InstructionCollectCreatorFee          = [8]byte{20, 22, 86, 123, 198, 28, 219, 132}
	InstructionCollectCreatorFeeV2        = [8]byte{207, 17, 138, 242, 4, 34, 19, 56}
	InstructionClaimCashback              = [8]byte{37, 58, 35, 126, 190, 53, 228, 197}
	InstructionClaimCashbackV2            = [8]byte{122, 243, 204, 65, 94, 116, 29, 55}
	InstructionDistributeCreatorFees      = [8]byte{165, 114, 103, 0, 121, 206, 247, 81}
	InstructionDistributeCreatorFeesV2    = [8]byte{255, 203, 19, 79, 244, 68, 8, 159}
	InstructionDistributeFeeToHolders     = [8]byte{98, 54, 145, 97, 2, 70, 173, 43}
	InstructionGetMinimumDistributableFee = [8]byte{117, 225, 127, 202, 134, 95, 68, 35}
	InstructionInitUserVolumeAccumulator  = [8]byte{94, 6, 202, 115, 255, 96, 232, 183}
	InstructionSyncUserVolumeAccumulator  = [8]byte{86, 31, 192, 87, 163, 87, 79, 238}
	InstructionCloseUserVolumeAccumulator = [8]byte{249, 69, 164, 218, 150, 103, 84, 138}
	InstructionClaimTokenIncentives       = [8]byte{16, 4, 71, 28, 204, 1, 40, 27}
	InstructionAdminUpdateTokenIncentives = [8]byte{209, 11, 115, 87, 213, 23, 124, 204}
	InstructionAdminSetIdlAuthority       = [8]byte{8, 217, 96, 231, 144, 104, 192, 5}
	InstructionAdminCto                   = [8]byte{125, 126, 214, 134, 77, 229, 188, 89}
	InstructionToggleMayhemMode           = [8]byte{1, 9, 111, 208, 100, 31, 255, 163}
	InstructionToggleCreateV2             = [8]byte{28, 255, 230, 240, 172, 107, 203, 171}
	InstructionToggleCashbackEnabled      = [8]byte{115, 103, 224, 255, 189, 89, 86, 195}
	InstructionSetMayhemVirtualParams     = [8]byte{61, 169, 188, 191, 153, 149, 42, 97}
	InstructionSetReservedFeeRecipients   = [8]byte{111, 172, 162, 232, 114, 89, 213, 142}
	InstructionSetVirtualQuoteReserves    = [8]byte{101, 135, 191, 104, 9, 88, 20, 96}
	InstructionUpdateBuybackConfig        = [8]byte{251, 224, 171, 146, 160, 26, 113, 233}
	InstructionUpdateCreatorFeeConfig     = [8]byte{61, 175, 160, 249, 66, 66, 136, 175}
	InstructionUpdateHolderRewardConfig   = [8]byte{225, 252, 66, 4, 199, 35, 236, 16}
	InstructionMigrateBondingCurveCreator = [8]byte{87, 124, 52, 191, 52, 38, 214, 232}
	InstructionAddQuoteMint               = [8]byte{111, 121, 21, 56, 40, 24, 94, 209}
	InstructionRemoveQuoteMint            = [8]byte{177, 65, 223, 38, 88, 209, 158, 155}
	InstructionInitializeQuoteControl     = [8]byte{239, 73, 245, 173, 209, 177, 84, 66}
	InstructionAddQuoteControlMint        = [8]byte{2, 14, 61, 138, 170, 142, 14, 95}
	InstructionRemoveQuoteControlMint     = [8]byte{223, 7, 253, 26, 81, 165, 218, 166}
	InstructionSetQuoteControlAdmin       = [8]byte{62, 79, 161, 211, 165, 170, 214, 210}

	// 统一交易接口（新）。所有账户均为必填，顺序固定，
	// SOL 与 USDC 配对币共用同一套账户布局。
	InstructionBuyV2             = [8]byte{184, 23, 238, 97, 103, 197, 211, 61}
	InstructionSellV2            = [8]byte{93, 246, 130, 60, 231, 233, 64, 178}
	InstructionBuyExactQuoteInV2 = [8]byte{194, 171, 28, 70, 104, 77, 91, 47}
	InstructionMigrateV2         = [8]byte{187, 203, 18, 31, 206, 237, 254, 41}
)

// IsTradeInstruction 判断判别码是否属于会产生 TradeEvent 的交易指令。
// 跟单与自成交识别都依赖这个集合，漏掉任何一个都会导致该指令的交易被静默忽略。
func IsTradeInstruction(disc [8]byte) bool {
	switch disc {
	case InstructionBuy,
		InstructionSell,
		InstructionBuyExactSolIn,
		InstructionBuyV2,
		InstructionSellV2,
		InstructionBuyExactQuoteInV2:
		return true
	default:
		return false
	}
}

// IsBuyInstruction 判断判别码是否为买入方向。
func IsBuyInstruction(disc [8]byte) bool {
	switch disc {
	case InstructionBuy, InstructionBuyExactSolIn, InstructionBuyV2, InstructionBuyExactQuoteInV2:
		return true
	default:
		return false
	}
}

// InstructionName 返回判别码对应的指令名，未知时返回空串。
func InstructionName(disc [8]byte) string {
	switch disc {
	case InstructionBuy:
		return "buy"
	case InstructionSell:
		return "sell"
	case InstructionBuyExactSolIn:
		return "buy_exact_sol_in"
	case InstructionBuyV2:
		return "buy_v2"
	case InstructionSellV2:
		return "sell_v2"
	case InstructionBuyExactQuoteInV2:
		return "buy_exact_quote_in_v2"
	case InstructionMigrate:
		return "migrate"
	case InstructionMigrateV2:
		return "migrate_v2"
	case InstructionCreate:
		return "create"
	case InstructionCreateV2:
		return "create_v2"
	default:
		return ""
	}
}
