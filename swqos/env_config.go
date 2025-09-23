package swqos

import (
	"os"
	"strconv"
	"strings"
)

// LoadConfigFromEnv 从环境变量加载swqos配置，覆盖默认值
func LoadConfigFromEnv() *SwqosConfig {
	config := GetDefaultSwqosConfig()

	// 加载各个提供商的配置
	loadProviderFromEnv(&config.Normal, "SWQOS_NORMAL")
	loadProviderFromEnv(&config.Jito, "SWQOS_JITO")
	loadProviderFromEnv(&config.NextBlock, "SWQOS_NEXTBLOCK")
	loadProviderFromEnv(&config.Temporal, "SWQOS_TEMPORAL")
	loadProviderFromEnv(&config.BloXroute, "SWQOS_BLOXROUTE")
	loadProviderFromEnv(&config.Zeroslot, "SWQOS_ZEROSLOT")
	loadProviderFromEnv(&config.Blockrazor, "SWQOS_BLOCKRAZOR")
	loadProviderFromEnv(&config.Astralane, "SWQOS_ASTRALANE")
	loadProviderFromEnv(&config.FlashBlock, "SWQOS_FLASHBLOCK")

	return config
}

// loadProviderFromEnv 从环境变量加载单个提供商的配置
func loadProviderFromEnv(provider *SwqosProviderConfig, prefix string) {
	// 加载 BuyTip
	if val := os.Getenv(prefix + "_BUY_TIP"); val != "" {
		if tip, err := strconv.ParseFloat(val, 64); err == nil {
			provider.BuyTip = tip
		}
	}

	// 加载 SellTip
	if val := os.Getenv(prefix + "_SELL_TIP"); val != "" {
		if tip, err := strconv.ParseFloat(val, 64); err == nil {
			provider.SellTip = tip
		}
	}

	// 加载 BuyEnable
	if val := os.Getenv(prefix + "_BUY_ENABLE"); val != "" {
		if enable, err := strconv.ParseBool(val); err == nil {
			provider.BuyEnable = enable
		}
	}

	// 加载 SellEnable
	if val := os.Getenv(prefix + "_SELL_ENABLE"); val != "" {
		if enable, err := strconv.ParseBool(val); err == nil {
			provider.SellEnable = enable
		}
	}

	// 向后兼容：如果设置了旧的ENABLE字段，同时设置BuyEnable和SellEnable
	if val := os.Getenv(prefix + "_ENABLE"); val != "" {
		if enable, err := strconv.ParseBool(val); err == nil {
			// 只有在新字段未设置时才使用旧字段的值
			if os.Getenv(prefix+"_BUY_ENABLE") == "" {
				provider.BuyEnable = enable
			}
			if os.Getenv(prefix+"_SELL_ENABLE") == "" {
				provider.SellEnable = enable
			}
		}
	}

	// 加载 Mode
	if val := os.Getenv(prefix + "_MODE"); val != "" {
		provider.Mode = val
	}

	// 加载 HealthEndpoint
	if val := os.Getenv(prefix + "_HEALTH_ENDPOINT"); val != "" {
		provider.HealthEndpoint = val
	}

	// 加载 RpcUrl
	if val := os.Getenv(prefix + "_RPC_URL"); val != "" {
		provider.RpcUrl = val
	}

	// 加载 ApiKeys (逗号分隔)
	if val := os.Getenv(prefix + "_API_KEYS"); val != "" {
		keys := strings.Split(val, ",")
		for i, key := range keys {
			keys[i] = strings.TrimSpace(key)
		}
		provider.ApiKeys = keys
	}
}

// SetEnvDefaults 设置环境变量的默认值示例
func SetEnvDefaults() {
	envDefaults := map[string]string{
		// Jito 配置
		"SWQOS_JITO_ENABLE":   "true",
		"SWQOS_JITO_BUY_TIP":  "0.0001",
		"SWQOS_JITO_SELL_TIP": "0.0001",
		"SWQOS_JITO_RPC_URL":  "https://frankfurt.mainnet.block-engine.jito.wtf/api/v1",

		// Zeroslot 配置
		"SWQOS_ZEROSLOT_ENABLE":   "true",
		"SWQOS_ZEROSLOT_BUY_TIP":  "0.001",
		"SWQOS_ZEROSLOT_SELL_TIP": "0.001",
		"SWQOS_ZEROSLOT_RPC_URL":  "http://ams1.0slot.trade",
		"SWQOS_ZEROSLOT_API_KEYS": "04a967a0e03a4ec3a1f10c971be97934",

		// Astralane 配置
		"SWQOS_ASTRALANE_ENABLE":   "true",
		"SWQOS_ASTRALANE_BUY_TIP":  "0.001",
		"SWQOS_ASTRALANE_SELL_TIP": "0.001",
		"SWQOS_ASTRALANE_RPC_URL":  "http://45.139.132.160",
		"SWQOS_ASTRALANE_API_KEYS": "ruruyIhQjzCzTJBRsJpiWP69gPrwPHkkuhgXsFgZaxz8kUiq3fSQLiycCejO6w3v",

		// FlashBlock 配置
		"SWQOS_FLASHBLOCK_ENABLE":   "true",
		"SWQOS_FLASHBLOCK_BUY_TIP":  "0.001",
		"SWQOS_FLASHBLOCK_SELL_TIP": "0.001",
		"SWQOS_FLASHBLOCK_RPC_URL":  "http://fra.flashblock.trade",
		"SWQOS_FLASHBLOCK_API_KEYS": "e07ffbb746914910",

		// Temporal 配置 (默认禁用)
		"SWQOS_TEMPORAL_ENABLE":   "false",
		"SWQOS_TEMPORAL_BUY_TIP":  "0.001",
		"SWQOS_TEMPORAL_SELL_TIP": "0.001",
		"SWQOS_TEMPORAL_RPC_URL":  "http://ams2.nozomi.temporal.xyz/",
		"SWQOS_TEMPORAL_API_KEYS": "298b5025-d944-4584-ab1c-91872a055323",

		// NextBlock 配置 (默认禁用)
		"SWQOS_NEXTBLOCK_ENABLE":   "false",
		"SWQOS_NEXTBLOCK_BUY_TIP":  "0.001",
		"SWQOS_NEXTBLOCK_SELL_TIP": "0.001",
		"SWQOS_NEXTBLOCK_RPC_URL":  "http://fra.nextblock.io/api/v2/submit",
		"SWQOS_NEXTBLOCK_API_KEYS": "intermediate1758252459-v9QhSQo%2FtRkXFjR2Yo6q%2FUhx0HbnYXOXQWxsb7DPoSg%3D",

		// BloXroute 配置 (默认禁用)
		"SWQOS_BLOXROUTE_ENABLE":   "false",
		"SWQOS_BLOXROUTE_BUY_TIP":  "0.001",
		"SWQOS_BLOXROUTE_SELL_TIP": "0.001",
		"SWQOS_BLOXROUTE_RPC_URL":  "http://germany.solana.dex.blxrbdn.com",
		"SWQOS_BLOXROUTE_API_KEYS": "YTU2YjIzNjktMzA1Zi00OWU3LTlkMDEtOGIwNWI4ODk5YmQ3OjQ0ZDk2NTAzNDgwNTk0ZGE1NWQ1YTVlYTc4NDhiNTZh",

		// Blockrazor 配置 (默认禁用)
		"SWQOS_BLOCKRAZOR_ENABLE":          "false",
		"SWQOS_BLOCKRAZOR_BUY_TIP":         "0.001",
		"SWQOS_BLOCKRAZOR_SELL_TIP":        "0.001",
		"SWQOS_BLOCKRAZOR_MODE":            "fast",
		"SWQOS_BLOCKRAZOR_HEALTH_ENDPOINT": "http://frankfurt.solana.blockrazor.xyz:443/health",
		"SWQOS_BLOCKRAZOR_RPC_URL":         "http://frankfurt.solana.blockrazor.xyz:443/sendTransaction",
		"SWQOS_BLOCKRAZOR_API_KEYS":        "fCpKRdHHLHornJqbEk4tfm494BcCBYL5gC3MuQEt7Y2PTwxwSFDyuLUGpJNfNQyi8lWx7VpAjSIhGn8DhLHh2IRJEixvIriT",
	}

	// 只设置未定义的环境变量
	for key, value := range envDefaults {
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

// ValidateConfig 验证配置的有效性
func ValidateConfig(config *SwqosConfig) []string {
	var errors []string

	// 检查是否至少有一个提供商启用
	enabledCount := 0
	providers := []struct {
		name   string
		config SwqosProviderConfig
	}{
		{"Normal", config.Normal},
		{"Jito", config.Jito},
		{"NextBlock", config.NextBlock},
		{"Temporal", config.Temporal},
		{"BloXroute", config.BloXroute},
		{"Zeroslot", config.Zeroslot},
		{"Blockrazor", config.Blockrazor},
		{"Astralane", config.Astralane},
		{"FlashBlock", config.FlashBlock},
	}

	for _, provider := range providers {
		if provider.config.IsEnabled() {
			enabledCount++

			// 检查启用的提供商是否有必要的配置
			if provider.config.RpcUrl == "" {
				errors = append(errors, provider.name+" provider is enabled but missing RPC URL")
			}

			// 检查需要API密钥的提供商
			needsApiKey := []string{"NextBlock", "Temporal", "BloXroute", "Zeroslot", "Blockrazor", "Astralane", "FlashBlock"}
			for _, needsKey := range needsApiKey {
				if provider.name == needsKey && len(provider.config.ApiKeys) == 0 {
					errors = append(errors, provider.name+" provider is enabled but missing API keys")
				}
			}

			// 检查小费金额是否合理
			if provider.config.BuyTip < 0 || provider.config.BuyTip > 1 {
				errors = append(errors, provider.name+" provider has invalid buy tip amount")
			}
			if provider.config.SellTip < 0 || provider.config.SellTip > 1 {
				errors = append(errors, provider.name+" provider has invalid sell tip amount")
			}
		}
	}

	if enabledCount == 0 {
		errors = append(errors, "No swqos providers are enabled")
	}

	return errors
}

// PrintConfigSummary 打印配置摘要
func PrintConfigSummary(config *SwqosConfig) {
	enabled := config.GetEnabledProviders()
	if len(enabled) > 0 {
		println("Enabled SWQOS providers:", strings.Join(enabled, ", "))
	} else {
		println("No SWQOS providers enabled")
	}
}
