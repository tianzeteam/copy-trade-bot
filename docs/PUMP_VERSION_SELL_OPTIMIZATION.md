# Pump 版本卖出优化文档

## 概述

优化 `ExecuteSellOrder` 方法，使其根据 `pumpVersion` 字段（v1 或 v2）使用不同的账户计算方法和卖出指令，与买入逻辑保持完全一致。

## 优化内容

### 1. bot/sell_strategy.go - ExecuteSellOrder 方法

#### 账户计算优化

**优化前**：
```go
// 固定使用 V1 的账户计算方法
pumpAccounts, err := b.Trader.AccountsCalc.CalculatePumpAccounts(
    context.Background(), 
    tokenInfo.Creator, 
    tokenInfo.Mint, 
    tokenInfo.Owner
)
```

**优化后**：
```go
pumpVersion := tokenInfo.PumpVersion
logger.Infof("Pump版本: %s", pumpVersion)

// 根据 PumpVersion 选择不同的账户计算方法
var pumpAccounts *types.PumpAccounts
var err error

if pumpVersion == "v2" {
    // V2 版本：使用 Token-2022 Program
    tokenProgram := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
    pumpAccounts, err = b.Trader.AccountsCalc.CalculatePumpV2Accounts(
        context.Background(),
        tokenInfo.Creator,
        tokenInfo.Mint,
        tokenInfo.Owner,
        tokenProgram,
    )
    logger.Debugf("使用 Pump V2 账户计算 (Token-2022): mint=%s", tokenInfo.Mint.String())
} else {
    // V1 版本：使用标准 Token Program
    pumpAccounts, err = b.Trader.AccountsCalc.CalculatePumpAccounts(
        context.Background(),
        tokenInfo.Creator,
        tokenInfo.Mint,
        tokenInfo.Owner,
    )
    logger.Debugf("使用 Pump V1 账户计算 (Token Program): mint=%s", tokenInfo.Mint.String())
}

if err != nil {
    logger.Errorf("计算账户信息失败 (version=%s): %v", pumpVersion, err)
    return
}
```

#### TradeParams 设置优化

**优化前**：
```go
tradeParams := types.TradeParams{
    Accounts: types.TradeAccounts{
        User:                   tokenInfo.Owner,
        Mint:                   tokenInfo.Mint,
        BondingCurve:           pumpAccounts.BondingCurve,
        AssociatedBondingCurve: pumpAccounts.AssociatedBondingCurve,
        AssociatedTokenAccount: pumpAccounts.AssociatedTokenAccount,
        CreatorVault:           pumpAccounts.CreatorVaultPda,
        NonceAccount:           *noncecache.GetNonceManager().GetNonceAccountPubkey(),
        // ❌ 缺少 Version 字段
    },
    // ...
}
```

**优化后**：
```go
tradeParams := types.TradeParams{
    Accounts: types.TradeAccounts{
        User:                   tokenInfo.Owner,
        Mint:                   tokenInfo.Mint,
        BondingCurve:           pumpAccounts.BondingCurve,
        AssociatedBondingCurve: pumpAccounts.AssociatedBondingCurve,
        AssociatedTokenAccount: pumpAccounts.AssociatedTokenAccount,
        CreatorVault:           pumpAccounts.CreatorVaultPda,
        NonceAccount:           *noncecache.GetNonceManager().GetNonceAccountPubkey(),
        Version:                pumpVersion, // ✅ 设置版本信息
    },
    // ...
}
```

### 2. core/trader.go - Sell 方法

#### 指令构建优化

**优化前**：
```go
// 固定使用 Pump V1 的 Builder API
sellInstruction := pump.NewSellInstructionBuilder().
    SetGlobalAccount(GLOBAL_ACCOUNT).
    SetFeeRecipientAccount(FEE_RECIPIENT).
    // ... 其他设置
    SetTokenProgramAccount(solana.TokenProgramID). // 固定使用 Token Program
    // ...
```

**优化后**：
```go
// 根据 Version 确定 token program
var tokenProgram solana.PublicKey
if params.TradeParams.Accounts.Version == "v2" {
    tokenProgram = solana.Token2022ProgramID
    logger.Infof("使用 Pump V2 (Token-2022 Program) 卖出")
} else {
    tokenProgram = solana.TokenProgramID
    logger.Infof("使用 Pump V1 (Token Program) 卖出")
}

// 根据 Version 构建不同的 Sell 指令
var buildInstruction solana.Instruction
var err error

if params.TradeParams.Accounts.Version == "v2" {
    // V2 版本：使用 pumpV2 的函数式 API
    buildInstruction, err = pumpV2.NewSellInstruction(
        params.TradeParams.Amount,
        params.TradeParams.Cost,
        GLOBAL_ACCOUNT,
        FEE_RECIPIENT,
        params.TradeParams.Accounts.Mint,
        params.TradeParams.Accounts.BondingCurve,
        params.TradeParams.Accounts.AssociatedBondingCurve,
        params.TradeParams.Accounts.AssociatedTokenAccount,
        params.TradeParams.Accounts.User,
        solana.SystemProgramID,
        tokenProgram, // Token-2022 Program
        params.TradeParams.Accounts.CreatorVault,
        EVENT_AUTHORITY,
        PUMP_PROGRAM_ID,
        solana.MustPublicKeyFromBase58("8Wf5TiAheLUqBrKXeYg2JtAFFMWtKdG2BSFgqUcPVwTt"),
        solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ"),
    )
} else {
    // V1 版本：使用 pump 的 Builder API
    sellInstruction := pump.NewSellInstructionBuilder().
        SetGlobalAccount(GLOBAL_ACCOUNT).
        // ... 其他设置
        SetTokenProgramAccount(tokenProgram). // 标准 Token Program
        // ...
    buildInstruction, err = sellInstruction.ValidateAndBuild()
}
```

## 参考实现

### wallet/transaction.go - executeCopyTrade 方法

买入逻辑的账户计算方式（作为参考）：

```go
// 根据 PumpVersion 选择不同的账户计算方法
var pumpAccounts *types.PumpAccounts

if tradeInfo.PumpVersion == "v2" {
    // V2 版本：使用 Token-2022 Program
    tokenProgram := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
    pumpAccounts, err = m.TradeBot.Trader.AccountsCalc.CalculatePumpV2Accounts(
        ctx,
        creator,
        mintAddr,
        botWallet,
        tokenProgram,
    )
    logger.Debugf("使用 Pump V2 账户计算 (Token-2022): mint=%s", mintAddr.String())
} else {
    // V1 版本：使用标准 Token Program
    pumpAccounts, err = m.TradeBot.Trader.AccountsCalc.CalculatePumpAccounts(
        ctx,
        creator,
        mintAddr,
        botWallet,
    )
    logger.Debugf("使用 Pump V1 账户计算 (Token Program): mint=%s", mintAddr.String())
}
```

## 版本差异总结

### Pump V1
- **Token Program**: `solana.TokenProgramID` (标准 SPL Token)
- **账户计算**: `CalculatePumpAccounts()`
- **指令构建**: `pump.NewSellInstructionBuilder()` (Builder API)
- **日志**: "使用 Pump V1 账户计算 (Token Program)"

### Pump V2
- **Token Program**: `solana.Token2022ProgramID` (Token-2022 扩展)
- **Token Program ID**: `TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb`
- **账户计算**: `CalculatePumpV2Accounts(tokenProgram)`
- **指令构建**: `pumpV2.NewSellInstruction()` (函数式 API)
- **日志**: "使用 Pump V2 账户计算 (Token-2022)"

## 优势

### 1. 完整的版本支持
- ✅ 支持 Pump V1 和 V2 两个版本
- ✅ 自动根据 tokenInfo.PumpVersion 选择正确的处理方式
- ✅ 与买入逻辑保持完全一致

### 2. 正确的账户计算
- ✅ V1 使用标准 Token Program 计算账户
- ✅ V2 使用 Token-2022 Program 计算账户
- ✅ 避免账户地址计算错误导致交易失败

### 3. 正确的指令构建
- ✅ V1 使用 pump Builder API
- ✅ V2 使用 pumpV2 函数式 API
- ✅ 使用正确的 Token Program ID

### 4. 完善的日志记录
- ✅ 记录使用的版本信息
- ✅ 记录账户计算方法
- ✅ 记录指令构建方式
- ✅ 便于调试和问题排查

## 测试建议

### 1. V1 代币卖出测试
```bash
# 测试 Pump V1 代币的卖出功能
# 确保 pumpVersion = "v1"
# 验证使用 CalculatePumpAccounts
# 验证使用 pump.NewSellInstructionBuilder
```

### 2. V2 代币卖出测试
```bash
# 测试 Pump V2 代币的卖出功能
# 确保 pumpVersion = "v2"
# 验证使用 CalculatePumpV2Accounts
# 验证使用 pumpV2.NewSellInstruction
```

### 3. 混合场景测试
```bash
# 同时持有 V1 和 V2 代币
# 验证卖出时正确识别版本
# 验证不同版本使用不同的处理逻辑
```

## 相关文件

- `bot/sell_strategy.go` - 卖出策略（ExecuteSellOrder 方法）
- `core/trader.go` - 交易执行（Sell 方法）
- `wallet/transaction.go` - 买入参考（executeCopyTrade 方法）
- `constants/pump_version.go` - 版本常量定义

## 状态

✅ 所有功能已实现并编译通过
✅ 与买入逻辑保持完全一致
⏳ 待进行端到端测试
