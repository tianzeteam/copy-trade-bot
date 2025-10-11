package global

import (
	"copytrade/types"
	"strconv"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
)

// GlobalConfig 全局配置结构体，存储在内存中
type GlobalConfig struct {
	// 交易平台启用状态
	PumpTradeEnable        bool           `json:"pump_trade_enable"`        // PumpFun交易启用状态
	PumpAmmTradeEnable     bool           `json:"pump_amm_trade_enable"`    // PumpAMM交易启用状态
	RaydiumTradeEnable     bool           `json:"raydium_trade_enable"`     // Raydium交易启用状态
	LaunchpadTradeEnable   bool           `json:"launchpad_trade_enable"`   // Launchpad交易启用状态
	MeteoraDbcTradeEnable  bool           `json:"meteora_dbc_trade_enable"` // Meteora DBC交易启用状态
	RepeatTradeEnable      bool           `json:"repeat_trade_enable"`      // 重复交易启用状态
	SellDelayTime          int            `json:"sell_delay_time"`          // 延迟卖出时间（ms）
	SellIntervalLimit      int            `json:"sell_interval_limit"`      // 卖出时间间隔限制（秒）
	DevSell                bool           `json:"dev_sell"`                 // 开启ds卖出
	CapitalRecoveryPercent float64        `json:"capital_recovery_percent"` // 抽回本金盈利百分比
	TradeWalletKey         *solana.Wallet `json:"trade_wallet_key"`         // 交易钱包密钥
	NonceAccountAddress    string         `json:"nonce_account_address"`    // nonce账户地址
	GlobalBlockhash        *solana.Hash   `json:"global_blockhash"`         // 全局区块哈希
	// 交易触发和池子大小参数
	MinFollowTriggerAmount float64 `json:"min_follow_trigger_amount"` // 最小触发买入数量 (SOL)
	MaxFollowTriggerAmount float64 `json:"max_follow_trigger_amount"` // 最大触发买入数量 (SOL)
	MinPoolSize            float64 `json:"min_pool_size"`             // 最小池子大小 (SOL)
	MaxPoolSize            float64 `json:"max_pool_size"`             // 最大池子大小 (SOL)

	// 交易参数
	MaxBuyAmount         float64 `json:"max_buy_amount"`         // 最大买入金额 (SOL)
	MinBuyAmount         float64 `json:"min_buy_amount"`         // 最小买入金额 (SOL)
	BuySolAmount         float64 `json:"buy_sol_amount"`         // 买入金额 (SOL)
	SellPercentage       float64 `json:"sell_percentage"`        // 卖出百分比
	StopLossPercentage   float64 `json:"stop_loss_percentage"`   // 止损百分比
	TakeProfitPercentage float64 `json:"take_profit_percentage"` // 止盈百分比

	// 滑点参数
	BuySlippage  float64 `json:"buy_slippage"`  // 买入滑点
	SellSlippage float64 `json:"sell_slippage"` // 卖出滑点

	// 交易手续费参数
	BuyPriorityFee  float64 `json:"buy_priority_fee"`  // 买入优先费
	BuyTipFee       float64 `json:"buy_tip_fee"`       // 买入小费
	SellPriorityFee float64 `json:"sell_priority_fee"` // 卖出优先费
	SellTipFee      float64 `json:"sell_tip_fee"`      // 卖出小费

	// 钱包参数模板
	WalletParamSchemes map[string]*types.ParamScheme `json:"wallet_param_schemes"` // 钱包地址到参数模板的映射

	// 其他系统配置
	UpdatedAt time.Time `json:"updated_at"` // 最后更新时间
}

var (
	// Config 全局配置实例
	Config GlobalConfig
	// configMutex 配置读写锁
	configMutex sync.RWMutex
)

// GetConfig 获取全局配置的副本
func GetConfig() GlobalConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return Config
}
func GetConfigByWallet(copyAddress string) *types.ParamScheme {
	configMutex.RLock()
	defer configMutex.RUnlock()

	if Config.WalletParamSchemes == nil {
		return nil
	}
	scheme, ok := Config.WalletParamSchemes[copyAddress]
	if !ok {
		return nil
	}
	return scheme
}

// UpdateConfig 更新全局配置
func UpdateConfig(newConfig GlobalConfig) {
	configMutex.Lock()
	defer configMutex.Unlock()
	Config = newConfig
	Config.UpdatedAt = time.Now()
}

// UpdateConfigField 更新全局配置的单个字段
func UpdateConfigField(key string, value any) {
	configMutex.Lock()
	defer configMutex.Unlock()

	switch key {
	// 交易平台启用状态
	case "pump_trade_enable":
		if val, ok := value.(bool); ok {
			Config.PumpTradeEnable = val
		}
	case "pump_amm_trade_enable":
		if val, ok := value.(bool); ok {
			Config.PumpAmmTradeEnable = val
		}
	case "raydium_trade_enable":
		if val, ok := value.(bool); ok {
			Config.RaydiumTradeEnable = val
		}
	case "launchpad_trade_enable":
		if val, ok := value.(bool); ok {
			Config.LaunchpadTradeEnable = val
		}
	case "meteora_dbc_trade_enable":
		if val, ok := value.(bool); ok {
			Config.MeteoraDbcTradeEnable = val
		}
	case "repeat_trade_enable":
		if val, ok := value.(bool); ok {
			Config.RepeatTradeEnable = val
		}
	case "sell_delay_time":
		if val, ok := value.(int); ok {
			Config.SellDelayTime = val
		}
	case "sell_interval_limit":
		if val, ok := value.(int); ok {
			Config.SellIntervalLimit = val
		}
	case "dev_sell":
		if val, ok := value.(bool); ok {
			Config.DevSell = val
		}
	case "capital_recovery_percent":
		if val, ok := value.(float64); ok {
			Config.CapitalRecoveryPercent = val
		} else if val, ok := value.(int); ok {
			Config.CapitalRecoveryPercent = float64(val)
		} else if val, ok := value.(string); ok {
			if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
				Config.CapitalRecoveryPercent = floatVal
			}
		}
	case "trade_wallet_key":
		if val, ok := value.(*solana.Wallet); ok {
			Config.TradeWalletKey = val
		} else if val, ok := value.(solana.Wallet); ok {
			Config.TradeWalletKey = &val
		}

	// 钱包参数模板
	case "wallet_param_schemes":
		if val, ok := value.(map[string]*types.ParamScheme); ok {
			Config.WalletParamSchemes = val
		}

	//nonce_account_pubkey
	case "nonce_account_pubkey":
		if val, ok := value.(string); ok {
			Config.NonceAccountAddress = val
		}
	}

	Config.UpdatedAt = time.Now()
}

// InitConfig 初始化全局配置
func InitConfig(config map[string]any) {
	configMutex.Lock()
	defer configMutex.Unlock()

	// 初始化默认配置
	_privateKey := getStringValue(config, "trade_wallet_key", "")
	privateKey := solana.MustPrivateKeyFromBase58(_privateKey)
	Config = GlobalConfig{
		// 交易平台启用状态
		PumpTradeEnable:        getBoolValue(config, "pump_trade_enable", true),
		PumpAmmTradeEnable:     getBoolValue(config, "pump_amm_trade_enable", false),
		RaydiumTradeEnable:     getBoolValue(config, "raydium_trade_enable", false),
		LaunchpadTradeEnable:   getBoolValue(config, "launchpad_trade_enable", false),
		MeteoraDbcTradeEnable:  getBoolValue(config, "meteora_dbc_trade_enable", false),
		RepeatTradeEnable:      getBoolValue(config, "repeat_trade_enable", false),
		SellIntervalLimit:      getIntValue(config, "sell_interval_limit", 2),
		DevSell:                getBoolValue(config, "dev_sell", false),
		CapitalRecoveryPercent: getFloatValue(config, "capital_recovery_percent", 0),
		TradeWalletKey:         &solana.Wallet{PrivateKey: privateKey}, //
		// 钱包参数模板
		WalletParamSchemes:  getWalletParamSchemes(config),
		NonceAccountAddress: getStringValue(config, "nonce_account_address", ""),
		// 更新时间
		UpdatedAt: time.Now(),
	}
}

func getFloatValue(config map[string]any, s string, defaultValue float64) float64 {
	if val, ok := config[s]; ok {
		if floatVal, ok := val.(float64); ok {
			return floatVal
		}
	}
	return defaultValue
}

// getBoolValue 从配置中获取布尔值，如果不存在则使用默认值
func getBoolValue(config map[string]any, key string, defaultValue bool) bool {
	if val, ok := config[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return defaultValue
}

// getIntValue 从配置中获取整数值，如果不存在则使用默认值
func getIntValue(config map[string]any, key string, defaultValue int) int {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case float32:
			return int(v)
		}
	}
	return defaultValue
}

// getStringValue 从配置中获取字符串值，如果不存在则使用默认值
func getStringValue(config map[string]any, key string, defaultValue string) string {
	if val, ok := config[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return defaultValue
}

// getHashValue 从配置中获取Hash值，如果不存在则使用默认值
func getHashValue(config map[string]any, key string, defaultValue string) *solana.Hash {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case *solana.Hash:
			return v
		case solana.Hash:
			return &v
		case string:
			if v != "" && v != defaultValue {
				// 尝试从base58字符串解析Hash
				if hashBytes, err := solana.HashFromBase58(v); err == nil {
					return &hashBytes
				}
			}
		}
	}

	// 如果默认值不为空，尝试解析
	if defaultValue != "" {
		if hashBytes, err := solana.HashFromBase58(defaultValue); err == nil {
			return &hashBytes
		}
	}

	return nil
}

// getWalletParamSchemes 从配置中获取钱包参数模板映射
func getWalletParamSchemes(config map[string]any) map[string]*types.ParamScheme {
	// 如果配置中存在钱包参数模板映射，直接返回
	if val, ok := config["wallet_param_schemes"]; ok {
		if schemes, ok := val.(map[string]*types.ParamScheme); ok {
			return schemes
		}
	}

	// 如果不存在或类型不匹配，返回空映射
	return make(map[string]*types.ParamScheme)
}
