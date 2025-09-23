package types

import (
	"time"
)

// ParamScheme 参数模板结构体
type ParamScheme struct {
	ID          string    `json:"id"`          // 参数模板ID
	Name        string    `json:"name"`        // 参数模板名称
	Description string    `json:"description"` // 参数模板描述
	IsDefault   bool      `json:"is_default"`  // 是否为默认方案
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`  // 更新时间

	// 交易参数
	SellDelayTime int `json:"sell_delay_time"` // 延迟卖出时间（ms）

	// 交易触发和池子大小参数
	MinFollowTriggerAmount float64 `json:"min_follow_trigger_amount"` // 最小触发买入数量 (SOL)
	MaxFollowTriggerAmount float64 `json:"max_follow_trigger_amount"` // 最大触发买入数量 (SOL)
	MinPoolSize            float64 `json:"min_pool_size"`             // 最小池子大小 (SOL)
	MaxPoolSize            float64 `json:"max_pool_size"`             // 最大池子大小 (SOL)

	// 代币创建时间过滤参数 (防止聪明钱包反向收割)
	TokenAgeMinMinutes int `json:"token_age_min_minutes"` // 代币最小创建时间 (分钟)
	TokenAgeMaxMinutes int `json:"token_age_max_minutes"` // 代币最大创建时间 (分钟)

	// 交易参数
	BuySolAmount float64 `json:"buy_sol_amount"` // 买入金额 (SOL)
	//MaxBuyAmount float64 `json:"max_buy_amount"` // 最大买入金额 (SOL)
	//MinBuyAmount float64 `json:"min_buy_amount"` // 最小买入金额 (SOL)
	//SellPercentage       float64 `json:"sell_percentage"`        // 卖出比例 (%)
	//StopLossPercentage   float64 `json:"stop_loss_percentage"`   // 止损比例 (%)
	//TakeProfitPercentage float64 `json:"take_profit_percentage"` // 止盈比例 (%)

	// 滑点参数
	BuySlippage  float64 `json:"buy_slippage"`  // 买入滑点 (%)
	SellSlippage float64 `json:"sell_slippage"` // 卖出滑点 (%)

	// 交易手续费参数
	BuyPriorityFee  float64 `json:"buy_priority_fee"`  // 买入优先费
	BuyTipFee       float64 `json:"buy_tip_fee"`       // 买入小费
	SellPriorityFee float64 `json:"sell_priority_fee"` // 卖出优先费
	SellTipFee      float64 `json:"sell_tip_fee"`      // 卖出小费
}
