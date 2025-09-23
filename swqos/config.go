package swqos

import (
	"copytrade/types"
)

// SwqosProviderConfig 定义单个swqos提供商的配置
type SwqosProviderConfig struct {
	BuyTip         float64  `json:"buy_tip"`
	SellTip        float64  `json:"sell_tip"`
	BuyEnable      bool     `json:"buy_enable"`
	SellEnable     bool     `json:"sell_enable"`
	Mode           string   `json:"mode"`
	HealthEndpoint string   `json:"health_endpoint"`
	RpcUrl         string   `json:"rpc_url"`
	ApiKeys        []string `json:"api_keys"`
}

// IsEnabled 检查提供商是否启用（买入或卖出任一启用即为启用）
func (c *SwqosProviderConfig) IsEnabled() bool {
	return c.BuyEnable || c.SellEnable
}

// IsBuyEnabled 检查是否启用买入
func (c *SwqosProviderConfig) IsBuyEnabled() bool {
	return c.BuyEnable
}

// IsSellEnabled 检查是否启用卖出
func (c *SwqosProviderConfig) IsSellEnabled() bool {
	return c.SellEnable
}

// SwqosConfig 定义所有swqos提供商的配置
type SwqosConfig struct {
	Normal     SwqosProviderConfig `json:"normal"`
	Jito       SwqosProviderConfig `json:"jito"`
	NextBlock  SwqosProviderConfig `json:"nextblock"`
	Temporal   SwqosProviderConfig `json:"temporal"`
	BloXroute  SwqosProviderConfig `json:"bloxroute"`
	Zeroslot   SwqosProviderConfig `json:"zeroslot"`
	Blockrazor SwqosProviderConfig `json:"blockrazor"`
	Astralane  SwqosProviderConfig `json:"astralane"`
	FlashBlock SwqosProviderConfig `json:"flashblock"`
}

// GetDefaultSwqosConfig 返回默认的swqos配置
func GetDefaultSwqosConfig() *SwqosConfig {
	return &SwqosConfig{
		Normal: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      false,
			SellEnable:     false,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "",
			ApiKeys:        []string{},
		},
		Jito: SwqosProviderConfig{
			BuyTip:         0.0001,
			SellTip:        0.0001,
			BuyEnable:      true,
			SellEnable:     true,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "https://frankfurt.mainnet.block-engine.jito.wtf/api/v1",
			ApiKeys:        []string{},
		},
		NextBlock: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      false,
			SellEnable:     false,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "http://fra.nextblock.io/api/v2/submit",
			ApiKeys:        []string{"intermediate1758252459-v9QhSQo%2FtRkXFjR2Yo6q%2FUhx0HbnYXOXQWxsb7DPoSg%3D"},
		},
		Temporal: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      false,
			SellEnable:     false,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "http://ams2.nozomi.temporal.xyz/",
			ApiKeys:        []string{"298b5025-d944-4584-ab1c-91872a055323"},
		},
		BloXroute: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      false,
			SellEnable:     false,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "http://germany.solana.dex.blxrbdn.com",
			ApiKeys:        []string{"YTU2YjIzNjktMzA1Zi00OWU3LTlkMDEtOGIwNWI4ODk5YmQ3OjQ0ZDk2NTAzNDgwNTk0ZGE1NWQ1YTVlYTc4NDhiNTZh"},
		},
		Zeroslot: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      true,
			SellEnable:     true,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "http://ams1.0slot.trade",
			ApiKeys:        []string{"04a967a0e03a4ec3a1f10c971be97934"},
		},
		Blockrazor: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      false,
			SellEnable:     false,
			Mode:           "fast",
			HealthEndpoint: "http://frankfurt.solana.blockrazor.xyz:443/health",
			RpcUrl:         "http://frankfurt.solana.blockrazor.xyz:443/sendTransaction",
			ApiKeys:        []string{"fCpKRdHHLHornJqbEk4tfm494BcCBYL5gC3MuQEt7Y2PTwxwSFDyuLUGpJNfNQyi8lWx7VpAjSIhGn8DhLHh2IRJEixvIriT"},
		},
		Astralane: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      true,
			SellEnable:     true,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "http://45.139.132.160",
			ApiKeys:        []string{"ruruyIhQjzCzTJBRsJpiWP69gPrwPHkkuhgXsFgZaxz8kUiq3fSQLiycCejO6w3v"},
		},
		FlashBlock: SwqosProviderConfig{
			BuyTip:         0.001,
			SellTip:        0.001,
			BuyEnable:      true,
			SellEnable:     true,
			Mode:           "",
			HealthEndpoint: "",
			RpcUrl:         "http://fra.flashblock.trade",
			ApiKeys:        []string{"e07ffbb746914910"},
		},
	}
}

// ToOrderParams 将swqos配置转换为OrderParams中的各个参数结构
func (c *SwqosConfig) ToOrderParams() *types.OrderParams {
	params := &types.OrderParams{}

	// 转换各个提供商的配置
	if c.Jito.IsBuyEnabled() {
		params.JitoParams = &types.JitoParams{
			Enable:    c.Jito.IsBuyEnabled(),
			BundleUrl: c.Jito.RpcUrl,
			TipAmount: uint64(c.Jito.BuyTip * 1e9), // 转换为lamports
		}
	}

	if c.NextBlock.IsBuyEnabled() {
		params.NextBlockParams = &types.NextBlockParams{
			Enable:    c.NextBlock.IsBuyEnabled(),
			BundleUrl: c.NextBlock.RpcUrl,
			ApiKey:    getFirstApiKey(c.NextBlock.ApiKeys),
			TipAmount: uint64(c.NextBlock.BuyTip * 1e9),
		}
	}

	if c.Temporal.IsBuyEnabled() {
		params.TemporalParams = &types.TemporalParams{
			Enable:    c.Temporal.IsBuyEnabled(),
			BundleUrl: c.Temporal.RpcUrl,
			ApiKey:    getFirstApiKey(c.Temporal.ApiKeys),
			TipAmount: uint64(c.Temporal.BuyTip * 1e9),
		}
	}

	if c.BloXroute.IsBuyEnabled() {
		params.BloXrouteParams = &types.BloXrouteParams{
			Enable:    c.BloXroute.IsBuyEnabled(),
			RpcUrl:    c.BloXroute.RpcUrl,
			ApiKeys:   c.BloXroute.ApiKeys,
			TipAmount: uint64(c.BloXroute.BuyTip * 1e9),
		}
	}

	if c.Zeroslot.IsBuyEnabled() {
		params.ZeroslotParams = &types.ZeroslotParams{
			Enable:    c.Zeroslot.IsBuyEnabled(),
			BundleUrl: c.Zeroslot.RpcUrl,
			RpcUrl:    c.Zeroslot.RpcUrl,
			ApiKeys:   c.Zeroslot.ApiKeys,
			TipAmount: uint64(c.Zeroslot.BuyTip * 1e9),
		}
	}

	if c.Blockrazor.IsBuyEnabled() {
		params.BlockrazorParams = &types.BlockrazorParams{
			Enable:         c.Blockrazor.IsBuyEnabled(),
			RpcUrl:         c.Blockrazor.RpcUrl,
			HealthEndpoint: c.Blockrazor.HealthEndpoint,
			ApiKey:         getFirstApiKey(c.Blockrazor.ApiKeys),
			Mode:           c.Blockrazor.Mode,
			TipAmount:      uint64(c.Blockrazor.BuyTip * 1e9),
		}
	}

	if c.Astralane.IsBuyEnabled() {
		params.AstralaneParams = &types.AstralaneParams{
			Enable:    c.Astralane.IsBuyEnabled(),
			RpcUrl:    c.Astralane.RpcUrl,
			ApiKey:    getFirstApiKey(c.Astralane.ApiKeys),
			TipAmount: uint64(c.Astralane.BuyTip * 1e9),
		}
	}

	if c.FlashBlock.IsBuyEnabled() {
		params.FlashBlockParams = &types.FlashBlockParams{
			Enable:    c.FlashBlock.IsBuyEnabled(),
			RpcUrl:    c.FlashBlock.RpcUrl,
			ApiKey:    getFirstApiKey(c.FlashBlock.ApiKeys),
			TipAmount: uint64(c.FlashBlock.BuyTip * 1e9),
		}
	}

	return params
}

// GetEnabledProviders 返回所有启用的提供商列表
func (c *SwqosConfig) GetEnabledProviders() []string {
	var enabled []string

	if c.Normal.IsEnabled() {
		enabled = append(enabled, "normal")
	}
	if c.Jito.IsEnabled() {
		enabled = append(enabled, "jito")
	}
	if c.NextBlock.IsEnabled() {
		enabled = append(enabled, "nextblock")
	}
	if c.Temporal.IsEnabled() {
		enabled = append(enabled, "temporal")
	}
	if c.BloXroute.IsEnabled() {
		enabled = append(enabled, "bloxroute")
	}
	if c.Zeroslot.IsEnabled() {
		enabled = append(enabled, "zeroslot")
	}
	if c.Blockrazor.IsEnabled() {
		enabled = append(enabled, "blockrazor")
	}
	if c.Astralane.IsEnabled() {
		enabled = append(enabled, "astralane")
	}
	if c.FlashBlock.IsEnabled() {
		enabled = append(enabled, "flashblock")
	}

	return enabled
}

// UpdateProvider 更新指定提供商的配置
func (c *SwqosConfig) UpdateProvider(provider string, config SwqosProviderConfig) {
	switch provider {
	case "normal":
		c.Normal = config
	case "jito":
		c.Jito = config
	case "nextblock":
		c.NextBlock = config
	case "temporal":
		c.Temporal = config
	case "bloxroute":
		c.BloXroute = config
	case "zeroslot":
		c.Zeroslot = config
	case "blockrazor":
		c.Blockrazor = config
	case "astralane":
		c.Astralane = config
	case "flashblock":
		c.FlashBlock = config
	}
}

// GetProvider 获取指定提供商的配置
func (c *SwqosConfig) GetProvider(provider string) *SwqosProviderConfig {
	switch provider {
	case "normal":
		return &c.Normal
	case "jito":
		return &c.Jito
	case "nextblock":
		return &c.NextBlock
	case "temporal":
		return &c.Temporal
	case "bloxroute":
		return &c.BloXroute
	case "zeroslot":
		return &c.Zeroslot
	case "blockrazor":
		return &c.Blockrazor
	case "astralane":
		return &c.Astralane
	case "flashblock":
		return &c.FlashBlock
	default:
		return nil
	}
}

// getFirstApiKey 获取API密钥列表中的第一个密钥
func getFirstApiKey(apiKeys []string) string {
	if len(apiKeys) > 0 {
		return apiKeys[0]
	}
	return ""
}

// 全局swqos配置实例
var GlobalSwqosConfig *SwqosConfig

// InitSwqosConfig 初始化全局swqos配置
func InitSwqosConfig() {
	GlobalSwqosConfig = GetDefaultSwqosConfig()
}

// GetSwqosConfig 获取全局swqos配置
func GetSwqosConfig() *SwqosConfig {
	if GlobalSwqosConfig == nil {
		InitSwqosConfig()
	}
	return GlobalSwqosConfig
}
