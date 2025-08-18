package utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModeConfig 表示每个模式的配置
type ModeConfig struct {
	BuyTip         float64  `yaml:"buy_tip"`
	SellTip        float64  `yaml:"sell_tip"`
	Enable         bool     `yaml:"enable"`
	Mode           string   `yaml:"mode"`
	HealthEndpoint string   `yaml:"health_endpoint"`
	RpcUrl         string   `yaml:"rpc_url,omitempty"`  // 可选的RPC URL
	ApiKeys        []string `yaml:"api_keys,omitempty"` // NextBlock的API密钥列表
}

// MultiSellLevel 表示多级卖出的配置
type MultiSellLevel struct {
	PricePercent int `yaml:"price_percent"` // 价格百分比
	SellPercent  int `yaml:"sell_percent"`  // 卖出百分比
}

// BatchSellConfig 批量卖出策略配置
type BatchSellConfig struct {
	NumBatches int   `yaml:"num_batches"` // 卖出批次数量
	Thresholds []int `yaml:"thresholds"`  // 每批卖出的价格阈值（百分比）
}

// LogConfig 表示日志配置
type LogConfig struct {
	Level  string `yaml:"level"`  // 日志级别: debug, info, warn, error, fatal
	Format string `yaml:"format"` // 日志格式: text, json
	File   string `yaml:"file"`   // 日志文件路径，为空则输出到控制台
}

// Config 表示整个配置文件结构
type Config struct {
	CreateNonceAccount     bool             `yaml:"create_nonce_account"`
	NonceAccountPubkey     string           `yaml:"nonce_account_pubkey"`
	SimulateMode           bool             `yaml:"simulate_mode"`
	PrivateKey             string           `yaml:"private_key"`
	GRPCUrl                string           `yaml:"grpc_url"`
	RPCUrl                 string           `yaml:"rpc_url"`
	RedisDb                int              `yaml:"redis_db"`
	RedisKeyPrefix         string           `yaml:"redis_key_prefix"`
	TradeChannel           string           `yaml:"trade_channel"`
	WebPort                int              `yaml:"web_port"`
	BuySolAmount           float64          `yaml:"buy_sol_amount"`
	SellIntervalLimit      int              `yaml:"sell_interval_limit"`
	SellMode               string           `yaml:"sell_mode"`
	SellDelayTime          int              `yaml:"sell_delay_time"`
	MultiSellLevels        []MultiSellLevel `yaml:"multi_sell_levels"`
	BatchSellConfig        BatchSellConfig  `yaml:"batch_sell_config"`
	SkipATACheck           bool             `yaml:"skip_ata_check"`
	PumpTradeEnable        bool             `yaml:"pump_trade_enable"`
	PumpAmmTradeEnable     bool             `yaml:"pump_amm_trade_enable"`
	RaydiumTradeEnable     bool             `yaml:"raydium_trade_enable"`
	LaunchpadTradeEnable   bool             `yaml:"launchpad_trade_enable"`
	RepeatTradeEnable      bool             `yaml:"repeat_trade_enable"`
	MinFollowTriggerAmount float64          `yaml:"min_follow_trigger_amount"` // 最小触发买入数量
	MaxFollowTriggerAmount float64          `yaml:"max_follow_trigger_amount"` // 最大触发买入数量
	MinPoolSize            float64          `yaml:"min_pool_size"`             // 最小pool大小
	MaxPoolSize            float64          `yaml:"max_pool_size"`             // 最大pool大小
	BuySlippage            float64          `yaml:"buy_slippage"`              // 全局买入滑点
	SellSlippage           float64          `yaml:"sell_slippage"`             // 全局卖出滑点
	BuyPriorityFee         float64          `yaml:"buy_priority_fee"`          // 全局买入优先级费用
	BuyTipFee              float64          `yaml:"buy_tip_fee"`               // 全局买入Tip费用
	SellPriorityFee        float64          `yaml:"sell_priority_fee"`         // 全局卖出优先级费用
	SellTipFee             float64          `yaml:"sell_tip_fee"`              // 全局卖出Tip费用
	ComputeUnits           uint32           `yaml:"compute_units"`             // 计算单元
	Logging                LogConfig        `yaml:"logging"`                   // 日志配置
	Normal                 ModeConfig       `yaml:"normal"`
	Jito                   ModeConfig       `yaml:"jito"`
	NextBlock              ModeConfig       `yaml:"nextblock"`
	Temporal               ModeConfig       `yaml:"temporal"`
	BloXroute              ModeConfig       `yaml:"bloXroute"`
	Zeroslot               ModeConfig       `yaml:"zeroslot"`
	Blockrazor             ModeConfig       `yaml:"blockrazor"`
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	// 读取文件
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := &Config{}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
}

// validateConfig 验证配置是否有效
func validateConfig(config *Config) error {
	// 验证每个模式的配置
	modes := map[string]ModeConfig{
		"normal":     config.Normal,
		"jito":       config.Jito,
		"nextblock":  config.NextBlock,
		"temportal":  config.Temporal,
		"bloXroute":  config.BloXroute,
		"zeroslot":   config.Zeroslot,
		"blockrazor": config.Blockrazor,
	}

	for name, mode := range modes {
		if mode.BuyTip < 0 || mode.SellTip < 0 {
			return fmt.Errorf("%s 模式的 tip 值不能为负", name)
		}
	}

	// 设置日志配置默认值
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}

	return nil
}
