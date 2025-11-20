# Pump AMM Token-2022 Program ID 修复

## 问题描述

在 Pump.fun AMM 买入交易中（**快速模式下**），当代币使用 Token-2022 Program 时，交易在链上失败，错误信息：

```
Program Error: "Instruction #5 Failed - incorrect program id for instruction"
Program log: Error: IncorrectProgramId
```

## 错误原因

### 根本原因

在 `wallet/transaction.go` 的 `executePumpAmmCopyTrade` 函数中（第 601-626 行），构建 `TradeParams` 时**没有设置 `Version` 字段**：

```go
// wallet/transaction.go 第 601-626 行
tradeParams := types.TradeParams{
    Accounts: types.TradeAccounts{
        // ❌ 问题：缺少 Version 字段！
        Pool:                      tradeInfo.AmmPool,
        User:                      m.BotWallet,
        Mint:                      pumpAccounts.Mint,
        BaseMint:                  pumpAccounts.Mint,
        QuoteMint:                 pumpAccounts.QuoteMint,
        UserBaseTokenAccount:      pumpAccounts.UserBaseTokenAccount,
        UserQuoteTokenAccount:     pumpAccounts.UserQuoteTokenAccount,
        PoolBaseTokenAccount:      pumpAccounts.PoolBaseTokenAccount,
        PoolQuoteTokenAccount:     pumpAccounts.PoolQuoteTokenAccount,
        CreatorVault:              pumpAccounts.CoinCreatorVaultAta,
        CoinCreatorVaultAuthority: pumpAccounts.CoinCreatorVaultAuthority,
        GlobalVolumeAccumulator:   pumpAccounts.GlobalVolumeAccumulator,
        UserVolumeAccumulator:     pumpAccounts.UserVolumeAccumulator,
        NonceAccount:              *m.NonceManager.GetNonceAccountPubkey(),
    },
    ...
}
```

然后在 `core/trader.go` 的 Pump AMM 买入逻辑中（第 447 行）：

```go
// core/trader.go 第 447 行
if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
    // ❌ 问题：Version 字段为空字符串，条件永远不成立！
    baseTokenProgramID = solana.Token2022ProgramID
    createATAIx = ataBuilder.BuildV2()
}
```

### 错误流程

1. **数据源**：`tradeInfo.PumpVersion` 包含正确的版本信息（"v2"）
2. **账户计算**：`CalculateAmmAccounts` 接收 `pumpVersion` 参数并正确计算账户
3. **❌ 问题**：构建 `TradeParams` 时没有传递 `Version` 字段
4. **结果**：`params.TradeParams.Accounts.Version` 为空字符串
5. **判断失败**：`Version == constants.PumpVersionV2` 条件不成立
6. **错误选择**：使用默认的旧版 Token Program ID
7. **链上错误**：传递错误的 Token Program 导致 `IncorrectProgramId`

## 链上错误分析

从交易日志可以看到：

```
#5 - Associated Token Account Program: createIdempotent
> Invoking Token Program
  > Program log: Instruction: GetAccountDataSize
  > Program log: Error: IncorrectProgramId
  > Program Token Program consumed 884 of 193847 compute units
  > Program returned error: incorrect program id for instruction
```

**错误流程**：
1. ATA Program 尝试为 `tutu` 代币创建 ATA
2. 调用 Token Program 的 `GetAccountDataSize` 指令
3. 但 `tutu` 代币的 owner 是 Token-2022 Program
4. 传入的却是旧版 Token Program ID
5. Token Program 检测到 Program ID 不匹配，返回 `IncorrectProgramId` 错误

## 修复方案

### 代码修改

在 `wallet/transaction.go` 第 601-626 行，添加 `Version` 字段：

```go
tradeParams := types.TradeParams{
    Accounts: types.TradeAccounts{
        Version:                   tradeInfo.PumpVersion, // ✅ 添加：设置 Version 字段
        Pool:                      tradeInfo.AmmPool,
        User:                      m.BotWallet,
        Mint:                      pumpAccounts.Mint,
        BaseMint:                  pumpAccounts.Mint,
        QuoteMint:                 pumpAccounts.QuoteMint,
        UserBaseTokenAccount:      pumpAccounts.UserBaseTokenAccount,
        UserQuoteTokenAccount:     pumpAccounts.UserQuoteTokenAccount,
        PoolBaseTokenAccount:      pumpAccounts.PoolBaseTokenAccount,
        PoolQuoteTokenAccount:     pumpAccounts.PoolQuoteTokenAccount,
        CreatorVault:              pumpAccounts.CoinCreatorVaultAta,
        CoinCreatorVaultAuthority: pumpAccounts.CoinCreatorVaultAuthority,
        GlobalVolumeAccumulator:   pumpAccounts.GlobalVolumeAccumulator,
        UserVolumeAccumulator:     pumpAccounts.UserVolumeAccumulator,
        NonceAccount:              *m.NonceManager.GetNonceAccountPubkey(),
    },
    Amount:    amounts.TokenAmount,
    Cost:      amounts.MaxCost,
    Blockhash: _blockHash,
    FeeConfig: types.FeeConfig{
        ComputeUnits: m.TradeBot.Config.ComputeUnits,
        Slippage:     specialScheme.BuySlippage,
        PriorityFee:  uint64(specialScheme.BuyPriorityFee * 1e9),
    },
}
```

### 修复内容

添加一行代码：`Version: tradeInfo.PumpVersion`

这样 `core/trader.go` 中的判断逻辑就能正确工作：
- 如果 `Version == "v2"`：使用 Token-2022 Program
- 否则：使用标准 Token Program

## Token Program 对比

| Token Program | Program ID | 用途 |
|--------------|------------|------|
| Token Program (旧版) | `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA` | Pump.fun V1 代币 |
| Token-2022 Program | `TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb` | Pump.fun V2 代币 |

## 验证方法

### 1. 检查代币的 Token Program

```bash
solana account <MINT_ADDRESS> --output json | jq '.owner'
```

- 如果返回 `TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA`：使用旧版 Token Program
- 如果返回 `TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb`：使用 Token-2022 Program

### 2. 测试交易

使用修复后的代码，对 Token-2022 代币进行买入测试：
- 快速模式（SkipATACheck=true）：应该成功
- 安全模式（SkipATACheck=false）：应该成功

## 影响范围

- **影响平台**：Pump.fun AMM
- **影响操作**：买入 Token-2022 代币（跟单交易）
- **影响模式**：所有模式（快速模式和安全模式）
- **不影响**：旧版 Token Program 代币（V1）

## 相关文件

- `wallet/transaction.go`：**主要修复文件**（第 603 行添加 Version 字段）
- `core/trader.go`：Token Program 判断逻辑（第 447 行）
- `calculator/accounts.go`：账户计算逻辑（第 273 行接收 pumpVersion 参数）
- `constants/constants.go`：Version 常量定义
- `pumpammV2/buy.go`：Pump AMM V2 Buy 指令

## 卖出操作修复

在 `core/trader.go` 的 `AmmSell` 函数中（第 1938-1945 行），也存在相同问题：

### 问题代码

```go
// 第 1953 行：硬编码为标准 Token Program
sellInstruction, err := pumpammV2.NewSellInstruction(
    ...
    solana.TokenProgramID,  // ❌ 硬编码
    ...
)
```

### 修复代码

```go
// 根据代币版本确定 Token Program
var baseTokenProgramID = solana.TokenProgramID
if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
    baseTokenProgramID = solana.Token2022ProgramID
    logger.Infof("卖出使用 Token-2022 Program")
} else {
    logger.Infof("卖出使用标准 Token Program")
}

sellInstruction, err := pumpammV2.NewSellInstruction(
    ...
    baseTokenProgramID,  // ✅ 动态选择
    ...
)
```

**注意**：卖出逻辑中 `TradeParams.Accounts.Version` 已经正确设置（`bot/sell_strategy.go` 第 424 行），所以只需修复 `core/trader.go` 中的硬编码问题。

## 注意事项

1. **Version 字段来源**：`tradeInfo.PumpVersion` 必须正确标识代币版本（"v2" 或其他）
2. **数据流完整性**：确保 Version 信息从事件解析 → 交易信息 → TradeParams 的完整传递
3. **买入和卖出一致性**：买入和卖出都需要根据 Version 动态选择 Token Program
4. **其他平台检查**：检查其他平台（Raydium Launchpad、CPMM 等）是否有类似问题

## 修复日期

2025-11-20

## 相关问题

- 链上交易签名：`5cisp2VcyGxDU3K8aaMr86o9HLCeycLeGoSocauimeDoLeqfL71wt4nrQ9YKsfumo9yK4DTZ649c4wANDfa2sb23`
- 错误信息：`Instruction #5 Failed - incorrect program id for instruction`
