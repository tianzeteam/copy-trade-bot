# Token-2022 Associated Token Account (ATA) 派生修复

## 问题描述

在处理 Pump.fun V2 交易时，使用 Token-2022 Program 的 ATA 派生地址不正确，导致交易失败。

### 根本原因

`BuildV2()` 方法使用了 `FindAssociatedTokenAddress()` 函数来派生 ATA 地址，但该函数只支持标准 Token Program，不支持 Token-2022 Program。

## 修复方案

### 修改文件
`/usr/local/server/copy-trade-bot/vendor/github.com/gagliardetto/solana-go/programs/associated-token-account/CreateIdempotent.go`

### 修复前（错误）

```go
func (inst CreateIdempotent) BuildV2() *Instruction {
    // ❌ 错误：使用 FindAssociatedTokenAddress 不支持 Token-2022
    associatedTokenAddress, _, _ := solana.FindAssociatedTokenAddress(
        inst.Wallet,
        inst.Mint,
    )
    // ...
}
```

### 修复后（正确）

```go
func (inst CreateIdempotent) BuildV2() *Instruction {
    // ✅ 正确：使用 FindProgramAddress 并包含 token program 参数
    associatedTokenAddress, _, _ := solana.FindProgramAddress(
        [][]byte{
            inst.Wallet.Bytes(),
            Token2022ProgramID.Bytes(),  // 关键：包含 Token-2022 Program ID
            inst.Mint.Bytes(),
        },
        ProgramID, // ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL
    )
    // ...
}
```

## ATA 派生规则对比

### 标准 Token Program (Build)
```
seeds: [wallet, mint]
program: ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL

使用便捷函数: FindAssociatedTokenAddress(wallet, mint)
```

### Token-2022 Program (BuildV2)
```
seeds: [wallet, token_program, mint]
program: ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL

必须使用: FindProgramAddress([wallet, Token2022ProgramID, mint], ProgramID)
```

## 验证结果

### 测试用例
使用真实的链上交易数据进行验证：

```go
// 真实交易数据
creator := "HmgSia9qke4yYRRYQaXFs6dj4Nt7BSu4UNbaWymsTeS6"
mint := "HRwsEGCErPEDDGFZAbXdezhmimjLi2Ge18GodUUwpump" // eye token
user := "cwhihmydRGfmG9BX5i5Ame26NBtzYZbTX1BgoV2X43v"
tokenProgram := Token2022ProgramID
```

### 计算结果（修复后）

| 账户 | 计算值 | 链上实际值 | 状态 |
|------|--------|-----------|------|
| AssociatedTokenAccount | `6id4Fkm5hbyHsEALxqHo4x7qnfbsRVPt3goj4g3yTSnu` | `6id4Fkm5hbyHsEALxqHo4x7qnfbsRVPt3goj4g3yTSnu` | ✅ 匹配 |
| CreatorVaultPda | `BhnSW3H1t7Bh2mVf77L5KFhUzYxn3eKYmQqd2D9qAd67` | `BhnSW3H1t7Bh2mVf77L5KFhUzYxn3eKYmQqd2D9qAd67` | ✅ 匹配 |
| GlobalVolumeAccumulator | `Hq2wp8uJ9jCPsYgNHex8RtqdvMPfVGoYwjvF1ATiwn2Y` | `Hq2wp8uJ9jCPsYgNHex8RtqdvMPfVGoYwjvF1ATiwn2Y` | ✅ 匹配 |
| UserVolumeAccumulator | `3r7LTaSH8QkJppauDuxGANmxXh6FgdDCXZ5Jwb3ZKgDF` | `3r7LTaSH8QkJppauDuxGANmxXh6FgdDCXZ5Jwb3ZKgDF` | ✅ 匹配 |
| BondingCurve | `GxnQPckQcwdrWTu9M73zUGqDPYxX85Vu1reufmgF15Qo` | - | ✅ 正确派生 |
| AssociatedBondingCurve | `J7mAKdzDec3H7XjfaxKGFoYVYc7j54wcDegsEx6xzHqv` | - | ✅ 正确派生 |

## 影响范围

### 修复的功能
1. ✅ Pump.fun V2 买入交易（Token-2022）
2. ✅ Pump.fun V2 卖出交易（Token-2022）
3. ✅ ATA 创建指令（Token-2022）
4. ✅ 账户计算器 `CalculatePumpV2Accounts`

### 相关代码
- `calculator/accounts.go` - `CalculatePumpV2Accounts()` 函数
- `core/trader.go` - 买入/卖出交易构建
- `wallet/transaction.go` - 交易管理器
- `bot/universal-router.go` - Pump V2 事件处理

## 关键常量

```go
// Associated Token Account Program
ProgramID = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"

// Token Programs
TokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
Token2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"

// Pump Program
PUMP_PROGRAM_ID = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"
```

## 使用建议

### 判断使用哪个版本

```go
// 根据 token program 判断使用哪个构建方法
if tokenProgram == solana.Token2022ProgramID {
    // Token-2022: 使用 BuildV2()
    createATAIx = ataBuilder.BuildV2()
} else {
    // 标准 Token: 使用 Build()
    createATAIx = ataBuilder.Build()
}
```

### 在 Pump V2 交易中

```go
// 检测版本
if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
    // V2: 使用 BuildV2() 创建 Token-2022 ATA
    createATAIx = ataBuilder.BuildV2()
} else {
    // V1: 使用 Build() 创建标准 Token ATA
    createATAIx = ataBuilder.Build()
}
```

## 测试覆盖

- ✅ `TestCalculatePumpV2Accounts_TokenProgram` - 使用真实交易数据验证
- ✅ `TestCalculatePumpV2Accounts_Token2022Program` - Token-2022 专项测试
- ✅ 所有测试通过

## 参考资料

- [Solana Associated Token Account Program](https://spl.solana.com/associated-token-account)
- [Token-2022 Program Documentation](https://spl.solana.com/token-2022)
- [Pump.fun V2 Transaction Example](https://solscan.io/)

## 总结

此修复确保了 Token-2022 的 ATA 地址能够正确派生，使 Pump.fun V2 交易能够成功执行。关键是在派生 ATA 地址时，必须将 token program ID 包含在 seeds 中。
