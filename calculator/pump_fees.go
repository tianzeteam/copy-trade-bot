package calculator

import (
	"context"
	"fmt"
	"time"

	"copytrade/global"
	"copytrade/logger"
	"copytrade/pumpV3"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// pumpFeeConfigMaxAge 是费率缓存的最长有效期。
// FeeConfig / Global 由管理员手动更新，正常情况下几天都不会变；
// 1 小时刷新一次足以覆盖突发调整，也不会给 RPC 带来压力。
const pumpFeeConfigMaxAge = time.Hour

// EnsurePumpFeeConfig 确保全局缓存里有可用的费率配置：
// 缓存存在且未过期时直接返回，否则重新拉取。
// 拉取失败但缓存仍可用时不视为错误。
func (c *AmountsCalculator) EnsurePumpFeeConfig(ctx context.Context) error {
	return ensurePumpFeeConfig(ctx, c.client)
}

// FetchPumpFeeConfig 强制从链上拉取 FeeConfig 与 Global 并写入全局缓存。
func (c *AmountsCalculator) FetchPumpFeeConfig(ctx context.Context) error {
	return fetchPumpFeeConfig(ctx, c.client)
}

// RunPumpFeeConfigRefresh 预热并周期性刷新 Pump 费率配置。
// 交易组装热路径只读缓存，不负责同步访问 RPC。
func (c *AmountsCalculator) RunPumpFeeConfigRefresh(ctx context.Context) {
	interval := pumpFeeConfigMaxAge
	for {
		if err := c.FetchPumpFeeConfig(ctx); err != nil {
			logger.Warnf("预热泵费率配置失败，将在 1 分钟后重试: %v", err)
			interval = time.Minute
		} else {
			interval = pumpFeeConfigMaxAge
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func ensurePumpFeeConfig(ctx context.Context, client *rpc.Client) error {
	feeConfig, _ := global.GetPumpFeeConfig()
	if feeConfig != nil && time.Since(global.PumpFeeConfigUpdatedAt()) < pumpFeeConfigMaxAge {
		return nil
	}
	return fetchPumpFeeConfig(ctx, client)
}

// fetchPumpFeeConfig 从链上拉取 FeeConfig 与 Global 并写入全局缓存。
// 两个账户独立拉取：任一失败只影响该账户，另一个照常更新。
func fetchPumpFeeConfig(ctx context.Context, client *rpc.Client) error {
	feeConfigAddr, err := pumpV3.FeeConfigPDA()
	if err != nil {
		return fmt.Errorf("派生 FeeConfig PDA 失败: %w", err)
	}

	var feeConfig *pumpV3.FeeConfig
	feeRaw, err := getPumpAccountData(ctx, client, feeConfigAddr)
	if err != nil {
		logger.Warnf("拉取泵费率配置失败 (%s): %v", feeConfigAddr, err)
	} else {
		feeConfig, err = decodeFeeConfig(feeRaw)
		if err != nil {
			logger.Warnf("解码泵费率配置失败: %v", err)
			feeConfig = nil
		} else {
			logger.Infof("泵费率配置已更新: %d 个 SOL 档位, %d 个稳定币档位", len(feeConfig.FeeTiers), len(feeConfig.StableFeeTiers))
		}
	}

	var globalConfig *pumpV3.Global
	globalRaw, err := getPumpAccountData(ctx, client, pumpV3.GlobalAccount)
	if err != nil {
		logger.Warnf("拉取泵 Global 配置失败 (%s): %v", pumpV3.GlobalAccount, err)
	} else {
		globalConfig, err = decodeGlobal(globalRaw)
		if err != nil {
			logger.Warnf("解码泵 Global 配置失败: %v", err)
			globalConfig = nil
		} else {
			logger.Infof("泵 Global 配置已更新: feeBps=%d, creatorFeeBps=%d, creatorFeeConfigurable=%v",
				globalConfig.FeeBasisPoints, globalConfig.CreatorFeeBasisPoints, globalConfig.CreatorFeeConfigurable)
		}
	}

	if feeConfig == nil && globalConfig == nil {
		return fmt.Errorf("FeeConfig 与 Global 均拉取失败")
	}
	global.SetPumpFeeConfig(feeConfig, globalConfig)
	return nil
}

// getPumpAccountData 拉取账户原始数据。
func getPumpAccountData(ctx context.Context, client *rpc.Client, addr solana.PublicKey) ([]byte, error) {
	if client == nil {
		return nil, fmt.Errorf("RPC 客户端未初始化")
	}
	res, err := client.GetAccountInfoWithOpts(ctx, addr, &rpc.GetAccountInfoOpts{
		Encoding: solana.EncodingBase64,
	})
	if err != nil {
		return nil, err
	}
	if res == nil || res.Value == nil {
		return nil, fmt.Errorf("账户不存在: %s", addr)
	}
	return res.Value.Data.GetBinary(), nil
}

// DecodePumpBondingCurve 校验账户判别码后解码内盘曲线。
func DecodePumpBondingCurve(data []byte) (*pumpV3.BondingCurve, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("账户数据过短: %d 字节", len(data))
	}
	var disc [8]byte
	copy(disc[:], data[:8])
	if disc != pumpV3.AccountBondingCurve {
		return nil, fmt.Errorf("BondingCurve 判别码不匹配: %x", disc)
	}
	out := new(pumpV3.BondingCurve)
	if err := out.UnmarshalWithDecoder(bin.NewBorshDecoder(data[8:])); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeFeeConfig 校验账户判别码后解码 FeeConfig。
func decodeFeeConfig(data []byte) (*pumpV3.FeeConfig, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("账户数据过短: %d 字节", len(data))
	}
	var disc [8]byte
	copy(disc[:], data[:8])
	if disc != pumpV3.AccountFeeConfig {
		return nil, fmt.Errorf("FeeConfig 判别码不匹配: %x", disc)
	}
	out := new(pumpV3.FeeConfig)
	if err := out.UnmarshalWithDecoder(bin.NewBorshDecoder(data[8:])); err != nil {
		return nil, err
	}
	return out, nil
}

// decodeGlobal 校验账户判别码后解码 Global。
func decodeGlobal(data []byte) (*pumpV3.Global, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("账户数据过短: %d 字节", len(data))
	}
	var disc [8]byte
	copy(disc[:], data[:8])
	if disc != pumpV3.AccountGlobal {
		return nil, fmt.Errorf("Global 判别码不匹配: %x", disc)
	}
	out := new(pumpV3.Global)
	if err := out.UnmarshalWithDecoder(bin.NewBorshDecoder(data[8:])); err != nil {
		return nil, err
	}
	return out, nil
}
