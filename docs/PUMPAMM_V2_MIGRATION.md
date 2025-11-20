# PumpAMM V2 迁移文档

## 概述

成功将 `PumpAmmBuy` 和 `AmmSell` 方法从旧的 `pumpamm` 模块迁移到新的 `pumpammV2` 模块。

## 主要变更

### 1. PumpAmmBuy 方法优化

**位置**: `core/trader.go` 第 435-479 行 (ATA 创建), 第 503-536 行 (买入指令)

**变更内容**:
- **ATA 创建逻辑优化**: 参考 `Buy` 方法，支持 Token-2022 (V2) 和标准 Token (V1)
  - 使用 `ataBuilder` 变量和条件判断
  - V2 使用 `BuildV2()` 创建 Token-2022 ATA
  - V1 使用 `Build()` 创建标准 Token ATA
- **买入指令优化**: 从 Builder 模式切换到直接函数调用
  - 使用 `pumpammV2.NewBuyInstruction()` 替代 `pumpamm.NewBuyInstructionBuilder()`
  - 添加完整的错误处理

**ATA 创建 - 之前**:
```go
if t.Config.SkipATACheck {
    createATAIx := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
        SetPayer(botWallet.PublicKey()).
        SetWallet(botWallet.PublicKey()).
        SetMint(params.TradeParams.Accounts.Mint).
        Build()  // 只支持标准 Token
    instructions = append(instructions, createATAIx)
}
```

**ATA 创建 - 之后**:
```go
if t.Config.SkipATACheck {
    ataBuilder := associatedtokenaccount.NewCreateIdempotentInstructionBuilder().
        SetPayer(botWallet.PublicKey()).
        SetWallet(botWallet.PublicKey()).
        SetMint(params.TradeParams.Accounts.Mint)

    var createATAIx *associatedtokenaccount.Instruction
    if params.TradeParams.Accounts.Version == constants.PumpVersionV2 {
        // V2: 使用 BuildV2() 创建 Token-2022 ATA
        createATAIx = ataBuilder.BuildV2()
    } else {
        // V1: 使用标准 Build() 创建 Token ATA
        createATAIx = ataBuilder.Build()
    }
    instructions = append(instructions, createATAIx)
}
```

**买入指令 - 之前**:
```go
buyInstruction := pumpamm.NewBuyInstructionBuilder().
    SetPoolAccount(...).
    SetUserAccount(...).
    // ... 更多 Set 方法
    SetBaseAmountOut(params.TradeParams.Amount).
    SetMaxQuoteAmountIn(params.TradeParams.Cost)

instructions = append(instructions, buyInstruction.Build())
```

**买入指令 - 之后**:
```go
buyInstruction, err := pumpammV2.NewBuyInstruction(
    params.TradeParams.Amount,                    // baseAmountOut
    params.TradeParams.Cost,                      // maxQuoteAmountIn
    params.TradeParams.Accounts.Pool,             // pool
    params.TradeParams.Accounts.User,             // user
    GLOBAL_CONFIG_ACCOUNT,                        // globalConfig
    // ... 所有账户参数
    pumpammV2.ProgramID,                          // program
    // ... 其他参数
)
if err != nil {
    logger.Errorf("创建买入指令失败: %v", err)
    return "", fmt.Errorf("创建买入指令失败: %w", err)
}

instructions = append(instructions, buyInstruction)
```

### 2. AmmSell 方法优化

**位置**: `core/trader.go` 第 1910-1941 行

**变更内容**:
- 从 Builder 模式切换到直接函数调用
- 使用 `pumpammV2.NewSellInstruction()` 替代 `pumpamm.NewSellInstructionBuilder()`
- 移除 `ValidateAndBuild()` 调用，直接使用返回的指令
- 添加错误处理

**之前**:
```go
sellInstruction := pumpamm.NewSellInstructionBuilder().
    SetGlobalConfigAccount(...).
    SetProtocolFeeRecipientAccount(...).
    // ... 更多 Set 方法
    SetBaseAmountIn(params.TradeParams.Amount).
    SetMinQuoteAmountOut(params.TradeParams.Cost)

ix, err := sellInstruction.ValidateAndBuild()
if err != nil {
    logger.Errorf("AmmSell构建指令失败: %v", err)
    return "", fmt.Errorf("AmmSell构建指令失败: %w", err)
}

instructions = append(instructions, ix)
```

**之后**:
```go
sellInstruction, err := pumpammV2.NewSellInstruction(
    params.TradeParams.Amount,                    // baseAmountIn
    params.TradeParams.Cost,                      // minQuoteAmountOut
    params.TradeParams.Accounts.Pool,             // pool
    params.TradeParams.Accounts.User,             // user
    GLOBAL_CONFIG_ACCOUNT,                        // globalConfig
    // ... 所有账户参数
    pumpammV2.ProgramID,                          // program
    // ... 其他参数
)
if err != nil {
    logger.Errorf("AmmSell构建指令失败: %v", err)
    return "", fmt.Errorf("AmmSell构建指令失败: %w", err)
}

instructions = append(instructions, sellInstruction)
```

## 技术细节

### pumpammV2 模块特点

1. **直接函数调用**: 使用 `NewBuyInstruction()` 和 `NewSellInstruction()` 直接创建指令
2. **参数顺序**: 参数按固定顺序传递，无需 Builder 模式
3. **错误处理**: 函数返回 `(Instruction, error)`，需要显式处理错误
4. **Program ID**: 使用 `pumpammV2.ProgramID` (pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA)

### 账户参数映射

| 参数名称 | 说明 |
|---------|------|
| baseAmountOut/baseAmountIn | 基础代币数量 |
| maxQuoteAmountIn/minQuoteAmountOut | 报价代币数量 |
| pool | 池账户 |
| user | 用户账户 |
| globalConfig | 全局配置账户 |
| baseMint | 基础代币 Mint |
| quoteMint | 报价代币 Mint (WSOL) |
| userBaseTokenAccount | 用户基础代币账户 |
| userQuoteTokenAccount | 用户报价代币账户 |
| poolBaseTokenAccount | 池基础代币账户 |
| poolQuoteTokenAccount | 池报价代币账户 |
| protocolFeeRecipient | 协议费用接收者 |
| protocolFeeRecipientTokenAccount | 协议费用接收者代币账户 |
| baseTokenProgram | 基础代币程序 (TokenProgramID) |
| quoteTokenProgram | 报价代币程序 (TokenProgramID) |
| systemProgram | 系统程序 |
| associatedTokenProgram | 关联代币程序 |
| eventAuthority | 事件权限 |
| program | 程序 ID (pumpammV2.ProgramID) |
| coinCreatorVaultAta | 创建者金库 ATA |
| coinCreatorVaultAuthority | 创建者金库权限 |
| globalVolumeAccumulator | 全局交易量累加器 (仅买入) |
| userVolumeAccumulator | 用户交易量累加器 (仅买入) |
| feeConfig | 费用配置 |
| feeProgram | 费用程序 |

## 优势

1. **Token-2022 支持**: 完整支持 Pump V2 的 Token-2022 标准
2. **更简洁**: 直接函数调用比 Builder 模式更简洁
3. **类型安全**: 编译时检查参数类型和数量
4. **更新的 IDL**: pumpammV2 使用最新的程序 IDL
5. **更好的错误处理**: 显式的错误返回和处理
6. **性能优化**: 减少中间对象创建
7. **代码一致性**: ATA 创建逻辑与 Buy 方法保持一致

## 兼容性

- ✅ 编译通过
- ✅ 保持相同的功能
- ✅ 使用相同的 Program ID
- ✅ 账户结构保持一致

## 测试建议

1. 测试买入交易流程
2. 测试卖出交易流程
3. 验证交易签名和确认
4. 检查费用计算是否正确
5. 确认账户创建和关闭逻辑

## 注意事项

1. pumpammV2 模块已在项目中导入 (`copytrade/pumpammV2`)
2. 旧的 `pumpamm` 模块仍然存在，但不再使用
3. Program ID 保持不变: `pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA`
4. 所有常量 (GLOBAL_CONFIG_ACCOUNT, PROTOCOL_FEE_RECIPIENT 等) 保持不变

## 相关文件

- `core/trader.go` - 主要修改文件
- `pumpammV2/instructions.go` - 新指令定义
- `pumpammV2/program-id.go` - Program ID 定义
- `core/constants.go` - 常量定义

## 更新日期

2025-01-19
