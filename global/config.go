package global

import (
	"sync"
	"time"
)

// GlobalConfig 全局配置结构体，存储在内存中
type GlobalConfig struct {
	// 交易平台启用状态
	PumpTradeEnable    bool `json:"pump_trade_enable"`     // PumpFun交易启用状态
	PumpAmmTradeEnable bool `json:"pump_amm_trade_enable"` // PumpAMM交易启用状态
	RaydiumTradeEnable bool `json:"raydium_trade_enable"`  // Raydium交易启用状态
	RepeatTradeEnable  bool `json:"repeat_trade_enable"`    // 重复交易启用状态
	SellDelayTime      int  `json:"sell_delay_time"`       // 延迟卖出时间（ms）
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
	case "repeat_trade_enable":
		if val, ok := value.(bool); ok {
			Config.RepeatTradeEnable = val
		}
	case "sell_delay_time":
		if val, ok := value.(int); ok {
			Config.SellDelayTime = val
		}

	// 交易触发和池子大小参数
	case "min_follow_trigger_amount":
		if val, ok := value.(float64); ok {
			Config.MinFollowTriggerAmount = val
		}
	case "max_follow_trigger_amount":
		if val, ok := value.(float64); ok {
			Config.MaxFollowTriggerAmount = val
		}
	case "min_pool_size":
		if val, ok := value.(float64); ok {
			Config.MinPoolSize = val
		}
	case "max_pool_size":
		if val, ok := value.(float64); ok {
			Config.MaxPoolSize = val
		}

	// 交易参数
	case "buy_sol_amount":
		if val, ok := value.(float64); ok {
			Config.BuySolAmount = val
		}
	case "max_buy_amount":
		if val, ok := value.(float64); ok {
			Config.MaxBuyAmount = val
		}
	case "min_buy_amount":
		if val, ok := value.(float64); ok {
			Config.MinBuyAmount = val
		}
	case "sell_percentage":
		if val, ok := value.(float64); ok {
			Config.SellPercentage = val
		}
	case "stop_loss_percentage":
		if val, ok := value.(float64); ok {
			Config.StopLossPercentage = val
		}
	case "take_profit_percentage":
		if val, ok := value.(float64); ok {
			Config.TakeProfitPercentage = val
		}

	// 滑点参数
	case "buy_slippage":
		if val, ok := value.(float64); ok {
			Config.BuySlippage = val
		}
	case "sell_slippage":
		if val, ok := value.(float64); ok {
			Config.SellSlippage = val
		}

	// 交易手续费参数
	case "buy_priority_fee":
		if val, ok := value.(float64); ok {
			Config.BuyPriorityFee = val
		}
	case "buy_tip_fee":
		if val, ok := value.(float64); ok {
			Config.BuyTipFee = val
		}
	case "sell_priority_fee":
		if val, ok := value.(float64); ok {
			Config.SellPriorityFee = val
		}
	case "sell_tip_fee":
		if val, ok := value.(float64); ok {
			Config.SellTipFee = val
		}

	// 兼容旧的键名
	case "maxPoolSize":
		if val, ok := value.(float64); ok {
			Config.MaxPoolSize = val
		}
	case "minPoolSize":
		if val, ok := value.(float64); ok {
			Config.MinPoolSize = val
		}
	case "minFollowTriggerAmount":
		if val, ok := value.(float64); ok {
			Config.MinFollowTriggerAmount = val
		}
	case "maxFollowTriggerAmount":
		if val, ok := value.(float64); ok {
			Config.MaxFollowTriggerAmount = val
		}
	case "sellDelayTime":
		if val, ok := value.(int); ok {
			Config.SellDelayTime = val
		}
	case "buySolAmount":
		if val, ok := value.(float64); ok {
			Config.BuySolAmount = val
		}
	case "maxBuyAmount":
		if val, ok := value.(float64); ok {
			Config.MaxBuyAmount = val
		}
	case "minBuyAmount":
		if val, ok := value.(float64); ok {
			Config.MinBuyAmount = val
		}
	case "sellPercentage":
		if val, ok := value.(float64); ok {
			Config.SellPercentage = val
		}
	case "stopLossPercentage":
		if val, ok := value.(float64); ok {
			Config.StopLossPercentage = val
		}
	case "takeProfitPercentage":
		if val, ok := value.(float64); ok {
			Config.TakeProfitPercentage = val
		}
	}

	Config.UpdatedAt = time.Now()
}

// InitConfig 初始化全局配置
func InitConfig(config map[string]any) {
	configMutex.Lock()
	defer configMutex.Unlock()

	// 初始化默认配置
	Config = GlobalConfig{
		// 交易平台启用状态
		PumpTradeEnable:    getBoolValue(config, "pump_trade_enable", true),
		PumpAmmTradeEnable: getBoolValue(config, "pump_amm_trade_enable", false),
		RaydiumTradeEnable: getBoolValue(config, "raydium_trade_enable", false),
		RepeatTradeEnable:  getBoolValue(config, "repeat_trade_enable", false),
		SellDelayTime:      getIntValue(config, "sell_delay_time", 0),

		// 交易触发和池子大小参数
		MinFollowTriggerAmount: getFloat64Value(config, "min_follow_trigger_amount", 0.5),
		MaxFollowTriggerAmount: getFloat64Value(config, "max_follow_trigger_amount", 10.0),
		MinPoolSize:            getFloat64Value(config, "min_pool_size", 200.0),
		MaxPoolSize:            getFloat64Value(config, "max_pool_size", 1000.0),

		// 交易参数
		BuySolAmount:         getFloat64Value(config, "buy_sol_amount", 0.002),
		MaxBuyAmount:         getFloat64Value(config, "max_buy_amount", 1.0),
		MinBuyAmount:         getFloat64Value(config, "min_buy_amount", 0.5),
		SellPercentage:       getFloat64Value(config, "sell_percentage", 100.0),
		StopLossPercentage:   getFloat64Value(config, "stop_loss_percentage", 10.0),
		TakeProfitPercentage: getFloat64Value(config, "take_profit_percentage", 50.0),

		// 滑点参数
		BuySlippage:  getFloat64Value(config, "buy_slippage", 0.2),
		SellSlippage: getFloat64Value(config, "sell_slippage", 0.2),

		// 交易手续费参数
		BuyPriorityFee:  getFloat64Value(config, "buy_priority_fee", 0.005),
		BuyTipFee:       getFloat64Value(config, "buy_tip_fee", 0.005),
		SellPriorityFee: getFloat64Value(config, "sell_priority_fee", 0.03),
		SellTipFee:      getFloat64Value(config, "sell_tip_fee", 0.03),

		// 更新时间
		UpdatedAt: time.Now(),
	}
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

// getFloat64Value 从配置中获取浮点值，如果不存在则使用默认值
func getFloat64Value(config map[string]any, key string, defaultValue float64) float64 {
	if val, ok := config[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case float32:
			return float64(v)
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
