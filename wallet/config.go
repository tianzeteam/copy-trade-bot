package wallet

import (
	"fmt"
	"os"

	"github.com/gagliardetto/solana-go"
	"gopkg.in/yaml.v3"
)

// WalletConfig 钱包配置结构体
type WalletConfig struct {
	Address       string `yaml:"address" json:"address"`                 // 钱包地址
	Name          string `yaml:"name" json:"name"`                       // 钱包名称或标识
	Type          string `yaml:"type" json:"type"`                       // 钱包类型: high_frequency 或 low_frequency
	TakeProfitPct int    `yaml:"take_profit_pct" json:"take_profit_pct"` // 止盈百分比
	StopLossPct   int    `yaml:"stop_loss_pct" json:"stop_loss_pct"`     // 止损百分比
	ExitTimeSec   int    `yaml:"exit_time_sec" json:"exit_time_sec"`     // 持仓时间(秒)
	Enabled       bool   `yaml:"enabled" json:"enabled"`                 // 是否启用
	FollowSell    bool   `yaml:"follow_sell" json:"follow_sell"`         // 是否跟卖
	ParamSchemeID string `yaml:"param_scheme_id" json:"param_scheme_id"` // 参数方案ID
}

// MonitorConfig 监控配置结构体
type MonitorConfig struct {
	Wallets []WalletConfig `yaml:"wallets"` // 监控的钱包列表
}

// LoadMonitorConfig 从文件加载监控配置
func LoadMonitorConfig(filename string) (*MonitorConfig, error) {
	// 读取文件
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := &MonitorConfig{}
	err = yaml.Unmarshal(buf, config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证配置
	if err := validateMonitorConfig(config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return config, nil
}

// validateMonitorConfig 验证监控配置是否有效
func validateMonitorConfig(config *MonitorConfig) error {
	if len(config.Wallets) == 0 {
		return fmt.Errorf("至少需要配置一个监控钱包")
	}

	for i, wallet := range config.Wallets {
		// 验证钱包地址
		if wallet.Address == "" {
			return fmt.Errorf("钱包 #%d 地址不能为空", i+1)
		}

		// 验证钱包地址格式
		_, err := solana.PublicKeyFromBase58(wallet.Address)
		if err != nil {
			return fmt.Errorf("钱包 #%d 地址格式无效: %s", i+1, wallet.Address)
		}

		// 验证钱包名称
		if wallet.Name == "" {
			return fmt.Errorf("钱包 #%d 名称不能为空", i+1)
		}

		// 验证钱包类型
		if wallet.Type != "high_frequency" && wallet.Type != "low_frequency" {
			return fmt.Errorf("钱包 #%d 类型无效，必须为 high_frequency 或 low_frequency", i+1)
		}

		// 验证止盈百分比
		if wallet.TakeProfitPct <= 0 {
			return fmt.Errorf("钱包 #%d 止盈百分比必须大于 0", i+1)
		}

		// 验证止损百分比
		if wallet.StopLossPct <= 0 {
			return fmt.Errorf("钱包 #%d 止损百分比必须大于 0", i+1)
		}

		// 验证持仓时间
		if wallet.ExitTimeSec <= 0 {
			return fmt.Errorf("钱包 #%d 持仓时间必须大于 0 秒", i+1)
		}
	}

	return nil
}

// CreateWalletSettings 从配置创建钱包设置
func CreateWalletSettings(config WalletConfig) (*WalletSettings, error) {
	// 解析钱包地址
	address, err := solana.PublicKeyFromBase58(config.Address)
	if err != nil {
		return nil, fmt.Errorf("解析钱包地址失败: %w", err)
	}

	// 解析钱包类型
	var walletType WalletType
	switch config.Type {
	case "high_frequency":
		walletType = HighFrequency
	case "low_frequency":
		walletType = LowFrequency
	default:
		return nil, fmt.Errorf("无效的钱包类型: %s", config.Type)
	}

	return &WalletSettings{
		Address:       address,
		Name:          config.Name,
		Type:          walletType,
		TakeProfitPct: config.TakeProfitPct,
		StopLossPct:   config.StopLossPct,
		ExitTimeSec:   config.ExitTimeSec,
		Enabled:       config.Enabled,
		IsFollowSell:  config.FollowSell,
		ParamSchemeID: config.ParamSchemeID,
	}, nil
}

// CreateWalletMonitorFromRedis 从Redis创建钱包监控器
func CreateWalletMonitorFromRedis(redisAddr string, redisDB int, redisKeyPrefix string, recoveryPct float64) (*WalletMonitor, error) {
	// 创建Redis钱包服务
	redisService, err := NewRedisWalletService(redisAddr, redisDB, redisKeyPrefix)
	if err != nil {
		return nil, fmt.Errorf("创建Redis钱包服务失败: %w", err)
	}

	// 创建钱包监控器
	// 注意：参数模板提供者将在web包中设置，这里传入nil
	monitor := NewWalletMonitor(redisService, nil)

	return monitor, nil
}
