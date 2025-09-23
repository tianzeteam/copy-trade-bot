package core

import (
	"copytrade/swqos"
	"copytrade/types"
	"copytrade/utils"

	"github.com/gagliardetto/solana-go"
)

// TradeAccounts 定义交易所需的账户
type TradeAccounts struct {
	Mint                    solana.PublicKey
	User                    solana.PublicKey
	BondingCurve            solana.PublicKey
	AssociatedBondingCurve  solana.PublicKey
	AssociatedTokenAccount  solana.PublicKey
	CreatorVault            solana.PublicKey
	GlobalVolumeAccumulator solana.PublicKey // 全局成交量累加器地址
	UserVolumeAccumulator   solana.PublicKey // 用户成交量累加器地址
}

// BuyParams 定义购买交易所需的参数
type BuyParams struct {
	Mode      types.TradeMode
	Accounts  TradeAccounts
	Amount    uint64      // token数量
	MaxCost   uint64      // 最大SOL花费
	Blockhash solana.Hash // 添加blockhash参数
}

// SellParams 定义卖出交易所需的参数
type SellParams struct {
	Mode      types.TradeMode
	Accounts  TradeAccounts
	Amount    uint64      // token数量
	MinOut    uint64      // 最小SOL获得
	Blockhash solana.Hash // 添加blockhash参数
}

// BuildOrderParams 从基础交易参数和swqos配置构建完整的OrderParams
func BuildOrderParams(tradeParams types.TradeParams, config *utils.Config, buyTip float64) *types.OrderParams {
	// 获取全局swqos配置
	swqosConfig := swqos.GetSwqosConfig()

	// 创建基础OrderParams
	orderParams := &types.OrderParams{
		TradeParams: tradeParams,
	}

	// 从swqos配置生成各个提供商的参数

	orderParams.JitoParams = &types.JitoParams{
		Enable:    config.Jito.Enable,
		BundleUrl: config.Jito.RpcUrl,
		TipAmount: uint64(buyTip * 1e9), // 转换为lamports
	}

	orderParams.NextBlockParams = &types.NextBlockParams{
		Enable:    config.NextBlock.Enable,
		BundleUrl: config.NextBlock.RpcUrl,
		ApiKey:    getFirstApiKey(swqosConfig.NextBlock.ApiKeys),
		TipAmount: uint64(buyTip * 1e9),
	}

	orderParams.TemporalParams = &types.TemporalParams{
		Enable:    config.Temporal.Enable,
		BundleUrl: config.Temporal.RpcUrl,
		ApiKey:    getFirstApiKey(swqosConfig.Temporal.ApiKeys),
		TipAmount: uint64(buyTip * 1e9),
	}

	orderParams.BloXrouteParams = &types.BloXrouteParams{
		Enable:    config.BloXroute.Enable,
		RpcUrl:    config.BloXroute.RpcUrl,
		ApiKeys:   swqosConfig.BloXroute.ApiKeys,
		TipAmount: uint64(buyTip * 1e9),
	}

	orderParams.ZeroslotParams = &types.ZeroslotParams{
		Enable:    config.Zeroslot.Enable,
		BundleUrl: config.Zeroslot.RpcUrl,
		RpcUrl:    config.Zeroslot.RpcUrl,
		ApiKeys:   swqosConfig.Zeroslot.ApiKeys,
		TipAmount: uint64(buyTip * 1e9),
	}

	orderParams.BlockrazorParams = &types.BlockrazorParams{
		Enable:         config.Blockrazor.Enable,
		RpcUrl:         config.Blockrazor.RpcUrl,
		HealthEndpoint: config.Blockrazor.HealthEndpoint,
		ApiKey:         getFirstApiKey(swqosConfig.Blockrazor.ApiKeys),
		Mode:           config.Blockrazor.Mode,
		TipAmount:      uint64(buyTip * 1e9),
	}

	orderParams.AstralaneParams = &types.AstralaneParams{
		Enable:    config.Astralane.Enable,
		RpcUrl:    config.Astralane.RpcUrl,
		ApiKey:    getFirstApiKey(swqosConfig.Astralane.ApiKeys),
		TipAmount: uint64(buyTip * 1e9),
	}

	orderParams.FlashBlockParams = &types.FlashBlockParams{
		Enable:    config.FlashBlock.Enable,
		RpcUrl:    config.FlashBlock.RpcUrl,
		ApiKey:    getFirstApiKey(swqosConfig.FlashBlock.ApiKeys),
		TipAmount: uint64(buyTip * 1e9),
	}

	return orderParams
}

// BuildSellOrderParams 为卖出操作构建OrderParams，使用卖出小费
func BuildSellOrderParams(tradeParams types.TradeParams) *types.OrderParams {
	// 获取全局swqos配置
	swqosConfig := swqos.GetSwqosConfig()

	// 创建基础OrderParams
	orderParams := &types.OrderParams{
		TradeParams: tradeParams,
	}

	// 从swqos配置生成各个提供商的参数，使用卖出小费
	if swqosConfig.Jito.IsSellEnabled() {
		orderParams.JitoParams = &types.JitoParams{
			Enable:    swqosConfig.Jito.IsSellEnabled(),
			BundleUrl: swqosConfig.Jito.RpcUrl,
			TipAmount: uint64(swqosConfig.Jito.SellTip * 1e9), // 使用卖出小费
		}
	}

	if swqosConfig.NextBlock.IsSellEnabled() {
		orderParams.NextBlockParams = &types.NextBlockParams{
			Enable:    swqosConfig.NextBlock.IsSellEnabled(),
			BundleUrl: swqosConfig.NextBlock.RpcUrl,
			ApiKey:    getFirstApiKey(swqosConfig.NextBlock.ApiKeys),
			TipAmount: uint64(swqosConfig.NextBlock.SellTip * 1e9),
		}
	}

	if swqosConfig.Temporal.IsSellEnabled() {
		orderParams.TemporalParams = &types.TemporalParams{
			Enable:    swqosConfig.Temporal.IsSellEnabled(),
			BundleUrl: swqosConfig.Temporal.RpcUrl,
			ApiKey:    getFirstApiKey(swqosConfig.Temporal.ApiKeys),
			TipAmount: uint64(swqosConfig.Temporal.SellTip * 1e9),
		}
	}

	if swqosConfig.BloXroute.IsSellEnabled() {
		orderParams.BloXrouteParams = &types.BloXrouteParams{
			Enable:    swqosConfig.BloXroute.IsSellEnabled(),
			RpcUrl:    swqosConfig.BloXroute.RpcUrl,
			ApiKeys:   swqosConfig.BloXroute.ApiKeys,
			TipAmount: uint64(swqosConfig.BloXroute.SellTip * 1e9),
		}
	}

	if swqosConfig.Zeroslot.IsSellEnabled() {
		orderParams.ZeroslotParams = &types.ZeroslotParams{
			Enable:    swqosConfig.Zeroslot.IsSellEnabled(),
			BundleUrl: swqosConfig.Zeroslot.RpcUrl,
			RpcUrl:    swqosConfig.Zeroslot.RpcUrl,
			ApiKeys:   swqosConfig.Zeroslot.ApiKeys,
			TipAmount: uint64(swqosConfig.Zeroslot.SellTip * 1e9),
		}
	}

	if swqosConfig.Blockrazor.IsSellEnabled() {
		orderParams.BlockrazorParams = &types.BlockrazorParams{
			Enable:         swqosConfig.Blockrazor.IsSellEnabled(),
			RpcUrl:         swqosConfig.Blockrazor.RpcUrl,
			HealthEndpoint: swqosConfig.Blockrazor.HealthEndpoint,
			ApiKey:         getFirstApiKey(swqosConfig.Blockrazor.ApiKeys),
			Mode:           swqosConfig.Blockrazor.Mode,
			TipAmount:      uint64(swqosConfig.Blockrazor.SellTip * 1e9),
		}
	}

	if swqosConfig.Astralane.IsSellEnabled() {
		orderParams.AstralaneParams = &types.AstralaneParams{
			Enable:    swqosConfig.Astralane.IsSellEnabled(),
			RpcUrl:    swqosConfig.Astralane.RpcUrl,
			ApiKey:    getFirstApiKey(swqosConfig.Astralane.ApiKeys),
			TipAmount: uint64(swqosConfig.Astralane.SellTip * 1e9),
		}
	}

	if swqosConfig.FlashBlock.IsSellEnabled() {
		orderParams.FlashBlockParams = &types.FlashBlockParams{
			Enable:    swqosConfig.FlashBlock.IsSellEnabled(),
			RpcUrl:    swqosConfig.FlashBlock.RpcUrl,
			ApiKey:    getFirstApiKey(swqosConfig.FlashBlock.ApiKeys),
			TipAmount: uint64(swqosConfig.FlashBlock.SellTip * 1e9),
		}
	}

	return orderParams
}

// getFirstApiKey 获取API密钥列表中的第一个密钥
func getFirstApiKey(apiKeys []string) string {
	if len(apiKeys) > 0 {
		return apiKeys[0]
	}
	return ""
}
