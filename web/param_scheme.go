package web

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/types"

	"github.com/go-redis/redis/v8"
)

// ParamScheme 参数方案结构
type ParamScheme struct {
	ID          string    `json:"id"`          // 参数方案ID
	Name        string    `json:"name"`        // 参数方案名称
	Description string    `json:"description"` // 参数方案描述
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
}

// ParamSchemeService 参数方案服务
type ParamSchemeService struct {
	Client     *redis.Client
	KeyPrefix  string
	Expiration time.Duration
}

// NewParamSchemeService 创建新的参数方案服务
func NewParamSchemeService(client *redis.Client, keyPrefix string) *ParamSchemeService {
	return &ParamSchemeService{
		Client:     client,
		KeyPrefix:  keyPrefix,
		Expiration: 0, // 永不过期
	}
}

// getSchemeKey 获取参数方案在Redis中的键名
func (s *ParamSchemeService) getSchemeKey(id string) string {
	return fmt.Sprintf("%s:param_scheme:%s", s.KeyPrefix, id)
}

// getSchemesListKey 获取参数方案列表在Redis中的键名
func (s *ParamSchemeService) getSchemesListKey() string {
	return fmt.Sprintf("%s:param_schemes", s.KeyPrefix)
}

// getDefaultSchemeKey 获取默认参数方案在Redis中的键名
func (s *ParamSchemeService) getDefaultSchemeKey() string {
	return fmt.Sprintf("%s:default_param_scheme", s.KeyPrefix)
}

// CreateScheme 创建新的参数方案
func (s *ParamSchemeService) CreateScheme(ctx context.Context, scheme *types.ParamScheme) error {
	// 设置创建和更新时间
	now := time.Now()
	scheme.CreatedAt = now
	scheme.UpdatedAt = now

	// 序列化参数方案数据
	schemeJSON, err := json.Marshal(scheme)
	if err != nil {
		return fmt.Errorf("序列化参数方案数据失败: %w", err)
	}

	// 保存到Redis
	key := s.getSchemeKey(scheme.ID)
	err = s.Client.Set(ctx, key, schemeJSON, s.Expiration).Err()
	if err != nil {
		return fmt.Errorf("保存参数方案数据失败: %w", err)
	}

	// 将方案ID添加到方案列表中
	err = s.Client.SAdd(ctx, s.getSchemesListKey(), scheme.ID).Err()
	if err != nil {
		return fmt.Errorf("添加参数方案到列表失败: %w", err)
	}

	// 如果是默认方案，设置默认方案ID
	if scheme.IsDefault {
		err = s.Client.Set(ctx, s.getDefaultSchemeKey(), scheme.ID, s.Expiration).Err()
		if err != nil {
			return fmt.Errorf("设置默认参数方案失败: %w", err)
		}
	}

	logger.Infof("成功创建参数方案: %s", scheme.Name)
	return nil
}

// GetScheme 获取指定ID的参数方案
func (s *ParamSchemeService) GetScheme(ctx context.Context, id string) (*types.ParamScheme, error) {
	key := s.getSchemeKey(id)
	schemeJSON, err := s.Client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("参数方案不存在: %s", id)
		}
		return nil, fmt.Errorf("获取参数方案数据失败: %w", err)
	}

	var scheme types.ParamScheme
	err = json.Unmarshal([]byte(schemeJSON), &scheme)
	if err != nil {
		return nil, fmt.Errorf("解析参数方案数据失败: %w", err)
	}

	return &scheme, nil
}

// UpdateScheme 更新参数方案
func (s *ParamSchemeService) UpdateScheme(ctx context.Context, scheme *types.ParamScheme) error {
	// 检查方案是否存在
	_, err := s.GetScheme(ctx, scheme.ID)
	if err != nil {
		return err
	}

	// 更新时间戳
	scheme.UpdatedAt = time.Now()

	// 序列化参数方案数据
	schemeJSON, err := json.Marshal(scheme)
	if err != nil {
		return fmt.Errorf("序列化参数方案数据失败: %w", err)
	}

	// 保存到Redis
	key := s.getSchemeKey(scheme.ID)
	err = s.Client.Set(ctx, key, schemeJSON, s.Expiration).Err()
	if err != nil {
		return fmt.Errorf("保存参数方案数据失败: %w", err)
	}

	// 如果是默认方案，设置默认方案ID
	if scheme.IsDefault {
		// 先将所有方案设为非默认
		schemes, err := s.ListSchemes(ctx)
		if err == nil {
			for _, existingScheme := range schemes {
				if existingScheme.ID != scheme.ID && existingScheme.IsDefault {
					existingScheme.IsDefault = false
					s.UpdateScheme(ctx, existingScheme)
				}
			}
		}

		err = s.Client.Set(ctx, s.getDefaultSchemeKey(), scheme.ID, s.Expiration).Err()
		if err != nil {
			return fmt.Errorf("设置默认参数方案失败: %w", err)
		}
	}

	logger.Infof("成功更新参数方案: %s", scheme.Name)
	return nil
}

// DeleteScheme 删除参数方案
func (s *ParamSchemeService) DeleteScheme(ctx context.Context, id string) error {
	// 检查是否是默认方案
	defaultID, err := s.GetDefaultSchemeID(ctx)
	if err == nil && defaultID == id {
		return fmt.Errorf("无法删除默认参数方案")
	}

	// 从Redis中删除
	key := s.getSchemeKey(id)
	err = s.Client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("删除参数方案失败: %w", err)
	}

	// 从方案列表中移除
	err = s.Client.SRem(ctx, s.getSchemesListKey(), id).Err()
	if err != nil {
		return fmt.Errorf("从列表中移除参数方案失败: %w", err)
	}

	logger.Infof("成功删除参数方案: %s", id)
	return nil
}

// ListSchemes 获取所有参数方案
func (s *ParamSchemeService) ListSchemes(ctx context.Context) ([]*types.ParamScheme, error) {
	// 获取所有方案ID
	ids, err := s.Client.SMembers(ctx, s.getSchemesListKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("获取参数方案列表失败: %w", err)
	}

	// 如果没有方案，返回空列表
	if len(ids) == 0 {
		return []*types.ParamScheme{}, nil
	}

	// 获取每个方案的详细信息
	schemes := make([]*types.ParamScheme, 0, len(ids))
	for _, id := range ids {
		scheme, err := s.GetScheme(ctx, id)
		if err != nil {
			logger.Warnf("获取参数方案失败: %s, %v", id, err)
			continue
		}
		schemes = append(schemes, scheme)
	}

	return schemes, nil
}

// GetDefaultSchemeID 获取默认参数方案ID
func (s *ParamSchemeService) GetDefaultSchemeID(ctx context.Context) (string, error) {
	id, err := s.Client.Get(ctx, s.getDefaultSchemeKey()).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("未设置默认参数方案")
		}
		return "", fmt.Errorf("获取默认参数方案ID失败: %w", err)
	}
	return id, nil
}

// GetDefaultScheme 获取默认参数方案
func (s *ParamSchemeService) GetDefaultScheme(ctx context.Context) (*types.ParamScheme, error) {
	id, err := s.GetDefaultSchemeID(ctx)
	if err != nil {
		return nil, err
	}
	return s.GetScheme(ctx, id)
}

// CreateDefaultSchemeFromConfig 从全局配置创建默认参数方案
func (s *ParamSchemeService) CreateDefaultSchemeFromConfig(ctx context.Context, config interface{}) (*types.ParamScheme, error) {
	// 使用类型断言将配置转换为全局配置类型
	globalConfig, ok := config.(*global.GlobalConfig)
	if !ok {
		return nil, fmt.Errorf("无效的配置类型")
	}

	// 创建新的参数方案，只包含非全局启用状态参数
	scheme := &types.ParamScheme{
		ID:                     "default",
		Name:                   "默认方案",
		Description:            "从配置文件自动生成的默认参数方案",
		IsDefault:              true,
		SellDelayTime:          globalConfig.SellDelayTime,
		MinFollowTriggerAmount: globalConfig.MinFollowTriggerAmount,
		MaxFollowTriggerAmount: globalConfig.MaxFollowTriggerAmount,
		MinPoolSize:            globalConfig.MinPoolSize,
		MaxPoolSize:            globalConfig.MaxPoolSize,
		//MaxBuyAmount:           globalConfig.MaxBuyAmount,
		//MinBuyAmount:           globalConfig.MinBuyAmount,
		BuySolAmount: globalConfig.BuySolAmount,
		//SellPercentage:         globalConfig.SellPercentage,
		//StopLossPercentage:   globalConfig.StopLossPercentage,
		//TakeProfitPercentage: globalConfig.TakeProfitPercentage,
		BuySlippage:     globalConfig.BuySlippage,
		SellSlippage:    globalConfig.SellSlippage,
		BuyPriorityFee:  globalConfig.BuyPriorityFee,
		BuyTipFee:       globalConfig.BuyTipFee,
		SellPriorityFee: globalConfig.SellPriorityFee,
		SellTipFee:      globalConfig.SellTipFee,
	}

	// 保存方案
	err := s.CreateScheme(ctx, scheme)
	if err != nil {
		return nil, err
	}

	return scheme, nil
}

// CreateSchemeFromGlobalConfig 从全局配置创建参数方案
func (s *ParamSchemeService) CreateSchemeFromGlobalConfig(ctx context.Context, id string, name string, description string) (*types.ParamScheme, error) {
	// 获取全局配置
	config := global.GetConfig()

	// 创建新的参数方案
	scheme := &types.ParamScheme{
		ID:                     id,
		Name:                   name,
		Description:            description,
		IsDefault:              false,
		SellDelayTime:          config.SellDelayTime,
		MinFollowTriggerAmount: config.MinFollowTriggerAmount,
		MaxFollowTriggerAmount: config.MaxFollowTriggerAmount,
		MinPoolSize:            config.MinPoolSize,
		MaxPoolSize:            config.MaxPoolSize,
		//MaxBuyAmount:           config.MaxBuyAmount,
		//MinBuyAmount:           config.MinBuyAmount,
		BuySolAmount: config.BuySolAmount,
		//SellPercentage:         config.SellPercentage,
		//StopLossPercentage:   config.StopLossPercentage,
		//TakeProfitPercentage: config.TakeProfitPercentage,
		BuySlippage:     config.BuySlippage,
		SellSlippage:    config.SellSlippage,
		BuyPriorityFee:  config.BuyPriorityFee,
		BuyTipFee:       config.BuyTipFee,
		SellPriorityFee: config.SellPriorityFee,
		SellTipFee:      config.SellTipFee,
	}

	// 保存方案
	err := s.CreateScheme(ctx, scheme)
	if err != nil {
		return nil, err
	}

	return scheme, nil
}
