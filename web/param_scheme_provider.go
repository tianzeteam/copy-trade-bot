package web

import (
	"context"
	"fmt"

	"copytrade/types"
	"copytrade/wallet"
)

// ParamSchemeProviderImpl 参数模板提供者实现
type ParamSchemeProviderImpl struct {
	paramSchemeService *ParamSchemeService
}

// NewParamSchemeProvider 创建新的参数模板提供者
func NewParamSchemeProvider(paramSchemeService *ParamSchemeService) wallet.ParamSchemeProvider {
	return &ParamSchemeProviderImpl{
		paramSchemeService: paramSchemeService,
	}
}

// GetParamSchemeIDs 获取所有参数模板ID
func (p *ParamSchemeProviderImpl) GetParamSchemeIDs(ctx context.Context) ([]string, error) {
	schemes, err := p.paramSchemeService.ListSchemes(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取参数模板列表失败: %w", err)
	}

	// 提取ID列表
	ids := make([]string, len(schemes))
	for i, scheme := range schemes {
		ids[i] = scheme.ID
	}

	return ids, nil
}

// GetDefaultParamSchemeID 获取默认参数模板ID
func (p *ParamSchemeProviderImpl) GetDefaultParamSchemeID(ctx context.Context) (string, error) {
	defaultID, err := p.paramSchemeService.GetDefaultSchemeID(ctx)
	if err != nil {
		return "", fmt.Errorf("获取默认参数模板ID失败: %w", err)
	}
	return defaultID, nil
}

// GetParamScheme 获取指定参数模板的详细信息
func (p *ParamSchemeProviderImpl) GetParamScheme(ctx context.Context, id string) (*types.ParamScheme, error) {
	scheme, err := p.paramSchemeService.GetScheme(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取参数模板详细信息失败: %w", err)
	}

	// 将web.ParamScheme转换为wallet.ParamScheme
	walletScheme := &types.ParamScheme{
		ID:                     scheme.ID,
		Name:                   scheme.Name,
		Description:            scheme.Description,
		IsDefault:              scheme.IsDefault,
		CreatedAt:              scheme.CreatedAt,
		UpdatedAt:              scheme.UpdatedAt,
		SellDelayTime:          scheme.SellDelayTime,
		MinFollowTriggerAmount: scheme.MinFollowTriggerAmount,
		MaxFollowTriggerAmount: scheme.MaxFollowTriggerAmount,
		MinPoolSize:            scheme.MinPoolSize,
		MaxPoolSize:            scheme.MaxPoolSize,
		MaxBuyAmount:           scheme.MaxBuyAmount,
		MinBuyAmount:           scheme.MinBuyAmount,
		BuySolAmount:           scheme.BuySolAmount,
		SellPercentage:         scheme.SellPercentage,
		StopLossPercentage:     scheme.StopLossPercentage,
		TakeProfitPercentage:   scheme.TakeProfitPercentage,
		BuySlippage:            scheme.BuySlippage,
		SellSlippage:           scheme.SellSlippage,
		BuyPriorityFee:         scheme.BuyPriorityFee,
		BuyTipFee:              scheme.BuyTipFee,
		SellPriorityFee:        scheme.SellPriorityFee,
		SellTipFee:             scheme.SellTipFee,
	}

	return walletScheme, nil
}
