# Pump V2 Accounts 单元测试文档

## 概述

为 `CalculatePumpV2Accounts` 函数创建了全面的单元测试，覆盖了多种场景和边界情况。

## 测试文件

- **文件路径**: `/usr/local/server/copy-trade-bot/test/pump_v2_accounts_test.go`
- **测试数量**: 5 个测试用例
- **测试框架**: `testify/assert`

## 测试用例

### 1. TestCalculatePumpV2Accounts_TokenProgram

**目的**: 测试使用标准 Token Program 的 Pump V2 账户计算

**测试内容**:
- 验证所有账户地址都不为零地址
- 验证 BondingCurve 的 PDA 派生正确性
- 验证 AssociatedBondingCurve 使用 ATA Program 派生
- 验证 AssociatedTokenAccount 使用 ATA Program 派生
- 验证 CreatorVaultPda 的派生
- 验证 GlobalVolumeAccumulator 的派生
- 验证 UserVolumeAccumulator 的派生

**关键验证点**:
```go
tokenProgram := solana.TokenProgramID // TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA
```

### 2. TestCalculatePumpV2Accounts_Token2022Program

**目的**: 测试使用 Token-2022 Program 的 Pump V2 账户计算

**测试内容**:
- 验证 Token-2022 Program 的账户派生
- 确认 AssociatedBondingCurve 和 AssociatedTokenAccount 使用正确的 token program

**关键验证点**:
```go
tokenProgram := solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
```

**预期结果**:
- AssociatedBondingCurve 和 AssociatedTokenAccount 与标准 Token Program 不同
- BondingCurve、CreatorVaultPda、GlobalVolumeAccumulator 保持不变

### 3. TestCalculatePumpV2Accounts_DifferentUsers

**目的**: 测试不同用户的账户计算

**测试内容**:
- 验证相同代币、不同用户的账户派生
- 确认共享账户（BondingCurve、CreatorVaultPda、GlobalVolumeAccumulator）相同
- 确认用户特定账户（AssociatedTokenAccount、UserVolumeAccumulator）不同

**预期结果**:
```
相同账户:
- BondingCurve
- AssociatedBondingCurve
- CreatorVaultPda
- GlobalVolumeAccumulator

不同账户:
- AssociatedTokenAccount (用户特定)
- UserVolumeAccumulator (用户特定)
```

### 4. TestCalculatePumpV2Accounts_DifferentMints

**目的**: 测试不同代币的账户计算

**测试内容**:
- 验证相同用户、不同代币的账户派生
- 确认代币特定账户不同
- 确认非代币相关账户相同

**预期结果**:
```
不同账户:
- BondingCurve (代币特定)
- AssociatedBondingCurve (代币特定)
- AssociatedTokenAccount (代币特定)

相同账户:
- CreatorVaultPda (创建者特定)
- GlobalVolumeAccumulator (全局)
- UserVolumeAccumulator (用户特定)
```

### 5. TestCalculatePumpV2Accounts_CompareWithV1

**目的**: 对比 Pump V1 和 V2 的账户计算差异

**测试内容**:
- 使用相同参数分别调用 V1 和 V2 函数
- 对比两个版本的账户派生结果
- 验证 V2 的向后兼容性

**关键发现**:
- 使用标准 Token Program 时，V1 和 V2 的 AssociatedBondingCurve 和 AssociatedTokenAccount 相同
- BondingCurve、CreatorVaultPda、GlobalVolumeAccumulator、UserVolumeAccumulator 在两个版本中完全相同

## 运行测试

### 运行所有 Pump V2 测试
```bash
cd /usr/local/server/copy-trade-bot
go test -v -run TestCalculatePumpV2Accounts ./test/
```

### 运行特定测试
```bash
# 测试 Token Program
go test -v -run TestCalculatePumpV2Accounts_TokenProgram ./test/

# 测试 Token-2022 Program
go test -v -run TestCalculatePumpV2Accounts_Token2022Program ./test/

# 测试不同用户
go test -v -run TestCalculatePumpV2Accounts_DifferentUsers ./test/

# 测试不同代币
go test -v -run TestCalculatePumpV2Accounts_DifferentMints ./test/

# 对比 V1 和 V2
go test -v -run TestCalculatePumpV2Accounts_CompareWithV1 ./test/
```

## 测试覆盖率

测试覆盖了以下场景:
- ✅ 标准 Token Program (SPL Token)
- ✅ Token-2022 Program
- ✅ 不同用户账户派生
- ✅ 不同代币账户派生
- ✅ V1 与 V2 兼容性
- ✅ PDA 派生正确性验证
- ✅ 零地址检查
- ✅ 账户唯一性验证

## 关键常量

```go
// Pump Program ID
PUMP_PROGRAM_ID = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"

// Token Programs
TokenProgramID = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
Token2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"

// Associated Token Account Program
SPLAssociatedTokenAccountProgramID = "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL"
```

## PDA 派生规则

### BondingCurve
```
seeds: ["bonding-curve", mint]
program: PUMP_PROGRAM_ID
```

### AssociatedBondingCurve (V2)
```
seeds: [bonding_curve, token_program, mint]
program: SPLAssociatedTokenAccountProgramID
```

### AssociatedTokenAccount (V2)
```
seeds: [user, token_program, mint]
program: SPLAssociatedTokenAccountProgramID
```

### CreatorVaultPda
```
seeds: ["creator-vault", creator]
program: PUMP_PROGRAM_ID
```

### GlobalVolumeAccumulator
```
seeds: ["global_volume_accumulator"]
program: PUMP_PROGRAM_ID
```

### UserVolumeAccumulator
```
seeds: ["user_volume_accumulator", user]
program: PUMP_PROGRAM_ID
```

## V1 vs V2 差异

| 账户 | V1 派生方法 | V2 派生方法 |
|------|------------|------------|
| BondingCurve | FindProgramAddress | FindProgramAddress (相同) |
| AssociatedBondingCurve | FindAssociatedTokenAddress | FindProgramAddress (ATA Program) |
| AssociatedTokenAccount | FindAssociatedTokenAddress | FindProgramAddress (ATA Program) |
| CreatorVaultPda | FindProgramAddress | FindProgramAddress (相同) |
| GlobalVolumeAccumulator | FindProgramAddress | FindProgramAddress (相同) |
| UserVolumeAccumulator | FindProgramAddress | FindProgramAddress (相同) |

**关键区别**: V2 支持 Token-2022 Program，通过在 ATA 派生中使用 `token_program` 参数实现。

## 测试结果示例

```
=== RUN   TestCalculatePumpV2Accounts_TokenProgram
    pump_v2_accounts_test.go:108: Calculated Pump V2 Accounts (Token Program): &{
        BondingCurve:4pQ2fZm555v3L5pTWBbP9mHU2TURwpryPZWEjv1pi1xY 
        AssociatedBondingCurve:41nVmVjvaVCrZhBYcPcAkr8A3GLPNvhzj9Z6fiJfcPKs 
        AssociatedTokenAccount:95PVSmdwkFi9ztKoMW8LgpFXcw2MorwRtQsCCUYsVnHF 
        CreatorVaultPda:AW1AaaoEhjxrQXkAzRNK5fLWbbjkuN1AATB1e2p2Xhfs 
        GlobalVolumeAccumulator:Hq2wp8uJ9jCPsYgNHex8RtqdvMPfVGoYwjvF1ATiwn2Y 
        UserVolumeAccumulator:BaE98B4ctejP1XFqXQrehoJuo9Es1Jr7jWssHMPQavCj
    }
--- PASS: TestCalculatePumpV2Accounts_TokenProgram (0.00s)
```

## 注意事项

1. **RPC 端点**: 测试使用 Helius RPC 端点，确保 API key 有效
2. **网络依赖**: 测试不依赖实际的链上数据，仅验证 PDA 派生逻辑
3. **确定性**: 所有测试都是确定性的，相同输入总是产生相同输出
4. **性能**: 所有测试在 0.01 秒内完成

## 维护建议

1. 当 Pump Program 更新时，验证 PDA 派生规则是否变化
2. 添加新的 token program 支持时，增加相应测试用例
3. 定期验证测试数据的有效性
4. 保持测试的独立性，避免测试间相互依赖
