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
	SellEnable     bool     `yaml:"sell_enable"`
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

// ProfitTargetMonitorConfig 盈利目标监控配置
type ProfitTargetMonitorConfig struct {
	Enable              bool    `yaml:"enable"`                // 是否启用盈利目标监控
	CheckInterval       int     `yaml:"check_interval"`        // 检查间隔（秒）
	Timeout             int     `yaml:"timeout"`               // 监控超时时间（秒）
	TargetProfitPercent float64 `yaml:"target_profit_percent"` // 目标盈利百分比
	PartialSellPercent  int     `yaml:"partial_sell_percent"`  // 未达标时部分卖出比例
	FullExitOnTimeout   bool    `yaml:"full_exit_on_timeout"`  // 超时未达标是否全部清仓
}

// BuyActivityMonitorConfig 买单活跃度监控配置（仅pumpfun内盘）
type BuyActivityMonitorConfig struct {
	Enable        bool    `yaml:"enable"`         // 是否启用买单活跃度监控
	Generalized   bool    `yaml:"generalized"`    // 是否泛化监控：true=持续监控，false=首次超时内有新单则永久取消
	CheckInterval int     `yaml:"check_interval"` // 检查间隔（毫秒）
	Timeout       int     `yaml:"timeout"`        // 无新买单超时时间（秒）
	SellPercent   int     `yaml:"sell_percent"`   // 超时卖出比例（%）
	MinBuyAmount  float64 `yaml:"min_buy_amount"` // 最小买单金额（SOL），低于此金额不算活跃买单
}

// LogConfig 表示日志配置
type LogConfig struct {
	Level  string `yaml:"level"`  // 日志级别: debug, info, warn, error, fatal
	Format string `yaml:"format"` // 日志格式: text, json
	File   string `yaml:"file"`   // 日志文件路径，为空则输出到控制台
}

// ShredStreamConfig ShredStream 配置 (默认关闭，不影响原有逻辑)
type ShredStreamConfig struct {
	Enabled           bool   `yaml:"enabled"`            // 是否启用 ShredStream (默认 false)
	Endpoint          string `yaml:"endpoint"`           // ShredStream 服务端点
	ReconnectInterval int    `yaml:"reconnect_interval"` // 重连间隔（秒）
	MaxRetries        int    `yaml:"max_retries"`        // 最大重试次数
}

// EventDedupConfig 事件去重配置 (仅在 ShredStream 启用时生效)
type EventDedupConfig struct {
	CacheSize  int `yaml:"cache_size"`   // LRU 缓存大小
	TTLSeconds int `yaml:"ttl_seconds"`  // 缓存过期时间（秒）
}

// Config 表示整个配置文件结构
type Config struct {
	SkipCreateNonceAccount bool                      `yaml:"skip_create_nonce_account"`
	NonceAccountPubkey     string                    `yaml:"nonce_account_pubkey"`
	SimulateMode           bool                      `yaml:"simulate_mode"`
	PrivateKey             string                    `yaml:"private_key"`
	GRPCUrl                string                    `yaml:"grpc_url"`
	RPCUrl                 string                    `yaml:"rpc_url"`
	RedisDb                int                       `yaml:"redis_db"`
	RedisKeyPrefix         string                    `yaml:"redis_key_prefix"`
	TradeChannel           string                    `yaml:"trade_channel"`
	WebPort                int                       `yaml:"web_port"`
	BuySolAmount           float64                   `yaml:"buy_sol_amount"`
	DevSell                bool                      `yaml:"dev_sell"`
	CapitalRecoveryPercent float64                   `yaml:"capital_recovery_percent"`
	SellIntervalLimit      int                       `yaml:"sell_interval_limit"`
	SellMode               string                    `yaml:"sell_mode"`
	SellDelayTime          int                       `yaml:"sell_delay_time"`
	MultiSellLevels        []MultiSellLevel          `yaml:"multi_sell_levels"`
	BatchSellConfig        BatchSellConfig           `yaml:"batch_sell_config"`
	ProfitTargetMonitor    ProfitTargetMonitorConfig `yaml:"profit_target_monitor"`
	BuyActivityMonitor     BuyActivityMonitorConfig  `yaml:"buy_activity_monitor"`
	SkipATACheck           bool                      `yaml:"skip_ata_check"`
	PumpTradeEnable        bool                      `yaml:"pump_trade_enable"`
	PumpAmmTradeEnable     bool                      `yaml:"pump_amm_trade_enable"`
	RaydiumTradeEnable     bool                      `yaml:"raydium_trade_enable"`
	LaunchpadTradeEnable   bool                      `yaml:"launchpad_trade_enable"`
	MeteoraDbcTradeEnable  bool                      `yaml:"meteora_dbc_trade_enable"`
	RepeatTradeEnable      bool                      `yaml:"repeat_trade_enable"`
	MinFollowTriggerAmount float64                   `yaml:"min_follow_trigger_amount"` // 最小触发买入数量
	MaxFollowTriggerAmount float64                   `yaml:"max_follow_trigger_amount"` // 最大触发买入数量
	MinPoolSize            float64                   `yaml:"min_pool_size"`             // 最小pool大小
	MaxPoolSize            float64                   `yaml:"max_pool_size"`             // 最大pool大小
	BuySlippage            float64                   `yaml:"buy_slippage"`              // 全局买入滑点
	SellSlippage           float64                   `yaml:"sell_slippage"`             // 全局卖出滑点
	BuyPriorityFee         float64                   `yaml:"buy_priority_fee"`          // 全局买入优先级费用
	BuyTipFee              float64                   `yaml:"buy_tip_fee"`               // 全局买入Tip费用
	SellPriorityFee        float64                   `yaml:"sell_priority_fee"`         // 全局卖出优先级费用
	SellTipFee             float64                   `yaml:"sell_tip_fee"`              // 全局卖出Tip费用
	ComputeUnits           uint32                    `yaml:"compute_units"`             // 计算单元
	Logging                LogConfig                 `yaml:"logging"`                   // 日志配置
	Normal                 ModeConfig                `yaml:"normal"`
	Jito                   ModeConfig                `yaml:"jito"`
	NextBlock              ModeConfig                `yaml:"nextblock"`
	Temporal               ModeConfig                `yaml:"temporal"`
	BloXroute              ModeConfig                `yaml:"bloXroute"`
	Zeroslot               ModeConfig                `yaml:"zeroslot"`
	Blockrazor             ModeConfig                `yaml:"blockrazor"`
	Astralane              ModeConfig                `yaml:"astralane"`
	FlashBlock             ModeConfig                `yaml:"flashblock"`

	// ========== ShredStream 配置 (新增，默认关闭) ==========
	ShredStream ShredStreamConfig `yaml:"shred_stream"` // ShredStream 配置
	EventDedup  EventDedupConfig  `yaml:"event_dedup"`  // 事件去重配置
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
		"astralane":  config.Astralane,
		"flashblock": config.FlashBlock,
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
