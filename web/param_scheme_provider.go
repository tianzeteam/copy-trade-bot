package web

import (
	"context"
	"fmt"

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
