package test

import (
	"copytrade/global"
	"encoding/json"
	"fmt"
	"testing"
)

func TestMemoryConfig(t *testing.T) {
	// 初始化全局配置
	configMap := map[string]interface{}{
		// 交易平台启用状态
		"pump_trade_enable":     true,
		"pumpamm_trade_enable":  false,
		"raydium_trade_enable":  false,

		// 交易触发和池子大小参数
		"min_follow_trigger_amount": 1.0,
		"max_follow_trigger_amount": 10.0,
		"min_pool_size":            200.0,
		"max_pool_size":            1000.0,

		// 交易参数
		"max_buy_amount":         1.0,
		"min_buy_amount":         0.5,
		"sell_percentage":       100.0,
		"stop_loss_percentage":   10.0,
		"take_profit_percentage": 50.0,

		// 滑点参数
		"buy_slippage":  0.2,
		"sell_slippage": 0.2,

		// 交易手续费参数
		"buy_priority_fee":  0.005,
		"buy_tip_fee":       0.005,
		"sell_priority_fee": 0.03,
		"sell_tip_fee":      0.03,
	}

	global.InitConfig(configMap)

	// 获取当前配置
	config := global.GetConfig()

	// 打印当前配置
	fmt.Println("当前内存配置:")
	configJSON, _ := json.MarshalIndent(config, "", "  ")
	fmt.Println(string(configJSON))

	// 更新配置字段
	fmt.Println("\n更新配置字段:")
	global.UpdateConfigField("max_pool_size", 15.0)
	global.UpdateConfigField("stop_loss_percentage", 15.0)
	global.UpdateConfigField("take_profit_percentage", 200.0)
	global.UpdateConfigField("buy_slippage", 0.5)
	global.UpdateConfigField("pump_trade_enable", false)

	// 获取更新后的配置
	updatedConfig := global.GetConfig()

	// 打印更新后的配置
	fmt.Println("\n更新后的内存配置:")
	updatedJSON, _ := json.MarshalIndent(updatedConfig, "", "  ")
	fmt.Println(string(updatedJSON))
}
