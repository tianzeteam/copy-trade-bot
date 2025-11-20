# Pump V2 Sell 指令账户顺序修复

## 问题描述

Pump.fun V2 卖出交易失败，错误信息：
```
Program Error: "custom program error: 3008 | Program ID was not as expected"
```

### 错误原因

在构建 Pump V2 Sell 指令时，`creator_vault` 和 `token_program` 账户的顺序**颠倒**了。

## 错误的账户顺序

### 实际发送到链上（错误）
```
Account #8: System Program        ✅ 正确
Account #9: Token 2022 Program    ❌ 错误 - 应该是 Creator Vault
Account #10: Creator Vault        ❌ 错误 - 应该是 Token Program
Account #11: Event Authority      ✅ 正确
```

### 代码中的错误（修复前）
```go
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
    tokenProgram,                              // ❌ Account 9 (错误位置)
    params.TradeParams.Accounts.CreatorVault,  // ❌ Account 10 (错误位置)
    EVENT_AUTHORITY,
    PUMP_PROGRAM_ID,
    // ...
)
```

## 正确的账户顺序

### 根据 IDL 定义
查看 `/usr/local/server/copy-trade-bot/IDL/generated/pumpV2/instructions.go`:

```go
func NewSellInstruction(
    // Params:
    amountParam uint64,
    minSolOutputParam uint64,

    // Accounts:
    globalAccount solanago.PublicKey,              // Account 0
    feeRecipientAccount solanago.PublicKey,        // Account 1
    mintAccount solanago.PublicKey,                // Account 2
    bondingCurveAccount solanago.PublicKey,        // Account 3
    associatedBondingCurveAccount solanago.PublicKey, // Account 4
    associatedUserAccount solanago.PublicKey,      // Account 5
    userAccount solanago.PublicKey,                // Account 6
    systemProgramAccount solanago.PublicKey,       // Account 7
    creatorVaultAccount solanago.PublicKey,        // Account 8 ✅
    tokenProgramAccount solanago.PublicKey,        // Account 9 ✅
    eventAuthorityAccount solanago.PublicKey,      // Account 10
    programAccount solanago.PublicKey,             // Account 11
    feeConfigAccount solanago.PublicKey,           // Account 12
    feeProgramAccount solanago.PublicKey,          // Account 13
)
```

### 账户元数据定义
```go
// Account 8 "creator_vault": Writable, Non-signer, Required
accounts__.Append(solanago.NewAccountMeta(creatorVaultAccount, true, false))

// Account 9 "token_program": Read-only, Non-signer, Required
accounts__.Append(solanago.NewAccountMeta(tokenProgramAccount, false, false))
```

## 修复方案

### 修改文件
`/usr/local/server/copy-trade-bot/core/trader.go`

### 修复后的代码
```go
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
        params.TradeParams.Accounts.CreatorVault, // ✅ Account 8: creator_vault
        tokenProgram,                              // ✅ Account 9: token_program (Token-2022)
        EVENT_AUTHORITY,
        PUMP_PROGRAM_ID,
        solana.MustPublicKeyFromBase58("8Wf5TiAheLUqBrKXeYg2JtAFFMWtKdG2BSFgqUcPVwTt"),
        solana.MustPublicKeyFromBase58("pfeeUxB6jkeY1Hxd7CsFCAjcbHA9rWtchMGdZ6VojVZ"),
    )
    // ...
}
```

## Buy vs Sell 账户顺序对比

### Buy 指令（正确，无需修改）
```go
// Account 8: token_program
tokenProgram,
// Account 9: creator_vault
params.TradeParams.Accounts.CreatorVault,
```

### Sell 指令（已修复）
```go
// Account 8: creator_vault
params.TradeParams.Accounts.CreatorVault,
// Account 9: token_program
tokenProgram,
```

**关键区别**: Buy 和 Sell 指令中 `creator_vault` 和 `token_program` 的顺序是**相反**的！

## 完整的 Sell 指令账户列表

| Index | 账户名称 | 权限 | 说明 |
|-------|---------|------|------|
| 0 | global | Read-only | 全局配置 |
| 1 | fee_recipient | Writable | 手续费接收账户 |
| 2 | mint | Read-only | 代币 Mint 地址 |
| 3 | bonding_curve | Writable | Bonding Curve 账户 |
| 4 | associated_bonding_curve | Writable | Bonding Curve 的 ATA |
| 5 | associated_user | Writable | 用户的代币账户 |
| 6 | user | Writable, Signer | 用户钱包 |
| 7 | system_program | Read-only | 系统程序 |
| 8 | **creator_vault** | **Writable** | **创建者金库** |
| 9 | **token_program** | **Read-only** | **Token Program (2022)** |
| 10 | event_authority | Read-only | 事件授权 |
| 11 | program | Read-only | Pump 程序 |
| 12 | fee_config | Read-only | 费用配置 |
| 13 | fee_program | Read-only | 费用程序 |

## 验证方法

### 1. 检查 IDL 定义
查看生成的 IDL 代码中的账户顺序和注释。

### 2. 对比链上交易
查看成功的链上交易，验证账户顺序是否匹配。

### 3. 错误码 3008
此错误通常表示程序期望的账户类型与实际提供的不匹配，常见于账户顺序错误。

## 影响范围

### 修复的功能
- ✅ Pump.fun V2 卖出交易（Token-2022）
- ✅ 所有使用 Pump V2 Sell 指令的功能

### 相关代码
- `core/trader.go` - `Sell()` 方法
- `IDL/generated/pumpV2/instructions.go` - `NewSellInstruction()`

## 注意事项

1. **Buy 和 Sell 的账户顺序不同** - 不要假设它们相同
2. **始终参考 IDL 定义** - 这是账户顺序的权威来源
3. **Token-2022 支持** - 确保使用正确的 token program
4. **测试两个方向** - Buy 和 Sell 都需要测试

## 相关文档

- [TOKEN2022_ATA_FIX.md](./TOKEN2022_ATA_FIX.md) - Token-2022 ATA 派生修复
- [Pump V2 IDL](../IDL/generated/pumpV2/instructions.go) - 指令定义

## 总结

此修复解决了 Pump V2 Sell 指令中 `creator_vault` 和 `token_program` 账户顺序颠倒的问题。关键教训是：
1. 严格按照 IDL 定义的顺序传递账户
2. Buy 和 Sell 指令的账户顺序可能不同
3. 错误码 3008 通常指示账户类型或顺序错误
