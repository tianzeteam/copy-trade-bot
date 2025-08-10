package wallet

import (
	"context"
	"fmt"
	"time"

	"copytrade/logger"
	"copytrade/types"

	"github.com/gagliardetto/solana-go"
)

// GetCachedSchemeTemplate 从缓存中获取参数模板(通过模板ID)
func (m *WalletMonitor) GetCachedSchemeTemplate(schemeID string) (*types.ParamScheme, error) {
	// 如果未指定模板ID，尝试使用默认模板ID
	if schemeID == "" {
		m.ParamSchemesMux.RLock()
		schemeID = m.DefaultSchemeID
		m.ParamSchemesMux.RUnlock()

		if schemeID == "" {
			return nil, fmt.Errorf("未指定参数模板ID且无默认模板")
		}
	}

	// 从模板缓存中获取参数模板
	m.ParamSchemesMux.RLock()
	scheme, exists := m.SchemeTemplates[schemeID]
	m.ParamSchemesMux.RUnlock()

	if !exists {
		// 如果缓存中不存在，尝试从提供者获取
		logger.Debugf("参数模板 %s 不在缓存中，尝试从提供者获取", schemeID)

		// 确保参数模板提供者存在
		if m.ParamSchemeProvider == nil {
			return nil, fmt.Errorf("参数模板提供者未设置")
		}

		// 从提供者获取参数模板
		scheme, err := m.ParamSchemeProvider.GetParamScheme(context.Background(), schemeID)
		if err != nil {
			return nil, fmt.Errorf("获取参数模板失败: %w", err)
		}

		// 将获取到的模板添加到模板缓存中
		m.ParamSchemesMux.Lock()
		m.SchemeTemplates[schemeID] = scheme
		m.ParamSchemesMux.Unlock()

		logger.Debugf("成功获取并缓存参数模板: %s", schemeID)
		return scheme, nil
	}

	return scheme, nil
}

// GetCachedParamScheme 从缓存中获取钱包的参数模板(通过钱包地址)
func (m *WalletMonitor) GetCachedParamScheme(walletAddress string) (*types.ParamScheme, error) {
	// 直接从钱包地址映射中获取参数模板
	m.ParamSchemesMux.RLock()
	scheme, exists := m.ParamSchemes[walletAddress]
	m.ParamSchemesMux.RUnlock()

	if exists {
		return scheme, nil
	}

	// 如果没有找到，尝试获取钱包绑定的模板ID并从模板缓存中获取
	var schemeID string

	// 获取钱包设置
	m.Mux.RLock()
	for _, wallet := range m.Wallets {
		if wallet.Address.String() == walletAddress {
			schemeID = wallet.ParamSchemeID
			break
		}
	}
	m.Mux.RUnlock()

	if schemeID != "" {
		// 如果钱包有绑定的模板ID，从模板缓存中获取
		scheme, err := m.GetCachedSchemeTemplate(schemeID)
		if err == nil {
			// 将模板添加到钱包地址映射中
			m.ParamSchemesMux.Lock()
			m.ParamSchemes[walletAddress] = scheme
			m.ParamSchemesMux.Unlock()
			return scheme, nil
		}
		return nil, err
	}

	// 如果钱包没有绑定模板，尝试使用默认模板
	m.ParamSchemesMux.RLock()
	defaultID := m.DefaultSchemeID
	m.ParamSchemesMux.RUnlock()

	if defaultID != "" {
		scheme, err := m.GetCachedSchemeTemplate(defaultID)
		if err == nil {
			// 将默认模板添加到钱包地址映射中
			m.ParamSchemesMux.Lock()
			m.ParamSchemes[walletAddress] = scheme
			m.ParamSchemesMux.Unlock()
			return scheme, nil
		}
		return nil, err
	}

	return nil, fmt.Errorf("钱包 %s 没有绑定参数模板且无默认模板", walletAddress)
}

// GetParamSchemeForWallet 获取指定钱包的参数模板
func (m *WalletMonitor) GetParamSchemeForWallet(walletAddress string) (*types.ParamScheme, error) {
	// 直接调用新的获取方法
	return m.GetCachedParamScheme(walletAddress)
}

// GetParamSchemeForPublicKey 通过PublicKey获取钱包的参数模板
func (m *WalletMonitor) GetParamSchemeForPublicKey(pubKey solana.PublicKey) (*types.ParamScheme, error) {
	return m.GetCachedParamScheme(pubKey.String())
}

// RefreshParamSchemeCache 强制刷新参数模板缓存
func (m *WalletMonitor) RefreshParamSchemeCache() {
	logger.Infof("强制刷新参数模板缓存")
	m.refreshParamSchemes()
}

// GetAllCachedSchemeTemplates 获取所有缓存的参数模板模板
func (m *WalletMonitor) GetAllCachedSchemeTemplates() map[string]*types.ParamScheme {
	m.ParamSchemesMux.RLock()
	defer m.ParamSchemesMux.RUnlock()

	// 创建一个副本以避免并发访问问题
	result := make(map[string]*types.ParamScheme, len(m.SchemeTemplates))
	for id, scheme := range m.SchemeTemplates {
		result[id] = scheme
	}

	return result
}

// GetAllCachedWalletParamSchemes 获取所有缓存的钱包参数模板
func (m *WalletMonitor) GetAllCachedWalletParamSchemes() map[string]*types.ParamScheme {
	m.ParamSchemesMux.RLock()
	defer m.ParamSchemesMux.RUnlock()

	// 创建一个副本以避免并发访问问题
	result := make(map[string]*types.ParamScheme, len(m.ParamSchemes))
	for addr, scheme := range m.ParamSchemes {
		result[addr] = scheme
	}

	return result
}

// GetCachedDefaultParamScheme 获取缓存的默认参数模板
func (m *WalletMonitor) GetCachedDefaultParamScheme() (*types.ParamScheme, error) {
	m.ParamSchemesMux.RLock()
	defaultID := m.DefaultSchemeID
	m.ParamSchemesMux.RUnlock()

	if defaultID == "" {
		return nil, fmt.Errorf("未设置默认参数模板")
	}

	return m.GetCachedSchemeTemplate(defaultID)
}

// GetLastRefreshTime 获取最后一次刷新时间
func (m *WalletMonitor) GetLastRefreshTime() time.Time {
	// 这个方法可以在将来扩展，记录最后一次刷新时间
	return time.Now()
}
