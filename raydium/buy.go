package raydium

import (
	"bytes"
	"fmt"

	"copytrade/raydium/layouts"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// BuyV4Instruction 定义Raydium V4的买入指令结构
type SwapV4Instruction struct {
	Instruction      uint8  // 指令类型，对于买入操作通常是9
	AmountIn         uint64 // 输入代币数量
	MinimumAmountOut uint64 // 最小输出数量（考虑滑点）

}

// Data 将指令序列化为字节数组
func (inst *SwapV4Instruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBorshEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode buy instruction: %w", err)
	}
	return buf.Bytes(), nil
}

// SwapInstructionBuilder 实现Builder模式来构建Raydium V4买入指令
type SwapInstructionBuilder struct {
	// 交易参数
	baseAmountIn      uint64 //
	minQuoteAmountOut uint64 //

	// 核心账户
	tokenProgramId solana.PublicKey // Token程序账户
	poolId         solana.PublicKey // 池账户
	authority      solana.PublicKey // 池权限账户
	openOrders     solana.PublicKey // 开放订单账户
	targetOrders   solana.PublicKey // 目标订单账户
	baseVault      solana.PublicKey // 基础代币保险库
	quoteVault     solana.PublicKey // 报价代币保险库

	// 市场相关账户
	marketProgramId  solana.PublicKey // 市场程序ID
	marketId         solana.PublicKey // 市场ID
	marketBids       solana.PublicKey // 市场买单账户
	marketAsks       solana.PublicKey // 市场卖单账户
	marketEventQueue solana.PublicKey // 市场事件队列
	marketBaseVault  solana.PublicKey // 市场基础代币保险库
	marketQuoteVault solana.PublicKey // 市场报价代币保险库
	marketAuthority  solana.PublicKey // 市场权限账户

	// 用户相关账户
	userOwner             solana.PublicKey // 用户所有者账户
	userBaseTokenAccount  solana.PublicKey // 用户基础代币账户
	userQuoteTokenAccount solana.PublicKey // 用户报价代币账户

	// 其他账户
	withdrawQueue  solana.PublicKey // 提款队列
	lpMint         solana.PublicKey // LP代币铸币账户
	lpVault        solana.PublicKey // LP代币保险库
	eventAuthority solana.PublicKey // 事件权限账户
	programId      solana.PublicKey // Raydium V4程序账户

	// 协议费相关账户
	protocolFeeRecipient    solana.PublicKey // 协议费接收者
	protocolFeeTokenAccount solana.PublicKey // 协议费代币账户
}

// NewSwapInstructionBuilder 创建一个新的买入指令构建器
func NewSwapInstructionBuilder() *SwapInstructionBuilder {
	return &SwapInstructionBuilder{}
}

// SetPoolAccount 设置池账户
func (b *SwapInstructionBuilder) SetPoolAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.poolId = account
	return b
}

// SetUserAccount 设置用户账户
func (b *SwapInstructionBuilder) SetUserAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.userOwner = account
	return b
}

// SetTokenProgramAccount 设置Token程序账户
func (b *SwapInstructionBuilder) SetTokenProgramAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.tokenProgramId = account
	return b
}

// SetAuthorityAccount 设置池权限账户
func (b *SwapInstructionBuilder) SetAuthorityAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.authority = account
	return b
}

// SetOpenOrders 设置开放订单账户
func (b *SwapInstructionBuilder) SetOpenOrders(account solana.PublicKey) *SwapInstructionBuilder {
	b.openOrders = account
	return b
}

// SetTargetOrders 设置目标订单账户
func (b *SwapInstructionBuilder) SetTargetOrders(account solana.PublicKey) *SwapInstructionBuilder {
	b.targetOrders = account
	return b
}

// SetBaseVault 设置基础代币保险库
func (b *SwapInstructionBuilder) SetBaseVault(account solana.PublicKey) *SwapInstructionBuilder {
	b.baseVault = account
	return b
}

// SetQuoteVault 设置报价代币保险库
func (b *SwapInstructionBuilder) SetQuoteVault(account solana.PublicKey) *SwapInstructionBuilder {
	b.quoteVault = account
	return b
}

// SetBaseTokenAccount 设置用户基础代币账户
func (b *SwapInstructionBuilder) SetBaseTokenAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.userBaseTokenAccount = account
	return b
}

// SetQuoteTokenAccount 设置用户报价代币账户
func (b *SwapInstructionBuilder) SetQuoteTokenAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.userQuoteTokenAccount = account
	return b
}

// SetUserBaseTokenAccountAccount 设置用户基础代币账户
func (b *SwapInstructionBuilder) SetUserBaseTokenAccountAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.userBaseTokenAccount = account
	return b
}

// SetUserQuoteTokenAccountAccount 设置用户报价代币账户
func (b *SwapInstructionBuilder) SetUserQuoteTokenAccountAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.userQuoteTokenAccount = account
	return b
}

// SetProtocolFeeRecipientAccount 设置协议费接收者账户
func (b *SwapInstructionBuilder) SetProtocolFeeRecipientAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.protocolFeeRecipient = account
	return b
}

// SetProtocolFeeRecipientTokenAccountAccount 设置协议费接收者代币账户
func (b *SwapInstructionBuilder) SetProtocolFeeRecipientTokenAccountAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.protocolFeeTokenAccount = account
	return b
}

// SetEventAuthorityAccount 设置事件权限账户
func (b *SwapInstructionBuilder) SetEventAuthorityAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.eventAuthority = account
	return b
}

// SetProgramAccount 设置程序账户
func (b *SwapInstructionBuilder) SetProgramAccount(account solana.PublicKey) *SwapInstructionBuilder {
	b.programId = account
	return b
}

// SetMarketId 设置市场ID
func (b *SwapInstructionBuilder) SetMarketId(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketId = account
	return b
}

// SetMarketProgramId 设置市场程序ID
func (b *SwapInstructionBuilder) SetMarketProgramId(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketProgramId = account
	return b
}

// SetMarketAuthority 设置市场权限账户
func (b *SwapInstructionBuilder) SetMarketAuthority(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketAuthority = account
	return b
}

// SetMarketBaseVault 设置市场基础代币保险库
func (b *SwapInstructionBuilder) SetMarketBaseVault(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketBaseVault = account
	return b
}

// SetMarketQuoteVault 设置市场报价代币保险库
func (b *SwapInstructionBuilder) SetMarketQuoteVault(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketQuoteVault = account
	return b
}

// SetMarketBids 设置市场买单账户
func (b *SwapInstructionBuilder) SetMarketBids(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketBids = account
	return b
}

// SetMarketAsks 设置市场卖单账户
func (b *SwapInstructionBuilder) SetMarketAsks(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketAsks = account
	return b
}

// SetMarketEventQueue 设置市场事件队列
func (b *SwapInstructionBuilder) SetMarketEventQueue(account solana.PublicKey) *SwapInstructionBuilder {
	b.marketEventQueue = account
	return b
}

// SetWithdrawQueue 设置提款队列账户
func (b *SwapInstructionBuilder) SetWithdrawQueue(account solana.PublicKey) *SwapInstructionBuilder {
	b.withdrawQueue = account
	return b
}

// SetLpVault 设置LP代币保险库
func (b *SwapInstructionBuilder) SetLpVault(account solana.PublicKey) *SwapInstructionBuilder {
	b.lpVault = account
	return b
}

// SetLpMint 设置LP代币铸币账户
func (b *SwapInstructionBuilder) SetLpMint(account solana.PublicKey) *SwapInstructionBuilder {
	b.lpMint = account
	return b
}

// SetBaseAmountIn 设置期望获得的基础代币数量
func (b *SwapInstructionBuilder) SetBaseAmountIn(amount uint64) *SwapInstructionBuilder {
	b.baseAmountIn = amount
	return b
}

// SetMaxQuoteAmountIn 设置最大输入的报价代币数量
func (b *SwapInstructionBuilder) SetMinQuoteAmountOut(amount uint64) *SwapInstructionBuilder {
	b.minQuoteAmountOut = amount
	return b
}

// Build 构建最终的Raydium V4买入指令
func (b *SwapInstructionBuilder) Build() solana.Instruction {
	// 创建指令数据
	inst := &SwapV4Instruction{
		Instruction:      9, // 买入指令类型
		AmountIn:         b.baseAmountIn,
		MinimumAmountOut: b.minQuoteAmountOut,
	}

	// 序列化指令数据
	data, err := inst.Data()
	if err != nil {
		panic(fmt.Errorf("failed to encode buy instruction: %w", err))
	}

	// 创建账户元数据列表
	keys := solana.AccountMetaSlice{}

	// 添加必要的账户元数据
	// 注意：账户顺序必须与Raydium V4程序预期的顺序一致
	keys.Append(solana.Meta(b.tokenProgramId))       // Token Program
	keys.Append(solana.Meta(b.poolId).WRITE())       // AMM Pool
	keys.Append(solana.Meta(b.authority))            // authority
	keys.Append(solana.Meta(b.openOrders).WRITE())   // Open Orders
	keys.Append(solana.Meta(b.targetOrders).WRITE()) // Target Orders
	keys.Append(solana.Meta(b.baseVault).WRITE())    // Pool Base Token Account
	keys.Append(solana.Meta(b.quoteVault).WRITE())   // Pool Quote Token Account

	// 添加市场相关账户
	keys.Append(solana.Meta(b.marketProgramId))          // Market Program ID
	keys.Append(solana.Meta(b.marketId).WRITE())         // Market ID
	keys.Append(solana.Meta(b.marketBids).WRITE())       // Market Bids
	keys.Append(solana.Meta(b.marketAsks).WRITE())       // Market Asks
	keys.Append(solana.Meta(b.marketEventQueue).WRITE()) // Market Event Queue
	keys.Append(solana.Meta(b.marketBaseVault).WRITE())  // Market Base Vault
	keys.Append(solana.Meta(b.marketQuoteVault).WRITE()) // Market Quote Vault
	keys.Append(solana.Meta(b.marketAuthority))          // Market Authority

	// 添加用户代币账户
	keys.Append(solana.Meta(b.userBaseTokenAccount).WRITE())  // User Base Token Account
	keys.Append(solana.Meta(b.userQuoteTokenAccount).WRITE()) // User Quote Token Account

	// 添加其他必要账户
	//keys.Append(solana.Meta(b.withdrawQueue).WRITE()) // Withdraw Queue
	//keys.Append(solana.Meta(b.lpMint))                // LP Mint
	//keys.Append(solana.Meta(b.lpVault).WRITE())       // LP Vault
	keys.Append(solana.Meta(b.userOwner).SIGNER()) // Event Authority

	// 创建并返回指令
	return solana.NewInstruction(
		b.programId,
		keys,
		data,
	)
}

// FromPoolInfo 从池信息创建买入指令构建器
func NewSwapInstructionBuilderFromPoolInfo(
	poolInfo *layouts.ApiPoolInfoV4,
	userAccount solana.PublicKey,
	userBaseTokenAccount solana.PublicKey,
	userQuoteTokenAccount solana.PublicKey,
	baseAmountIn uint64,
	minQuoteAmountOut uint64,
	programAccount solana.PublicKey,
) *SwapInstructionBuilder {
	return NewSwapInstructionBuilder().
		SetProgramAccount(programAccount).
		SetPoolAccount(poolInfo.ID).
		SetUserAccount(userAccount).
		SetBaseTokenAccount(userBaseTokenAccount).
		SetQuoteTokenAccount(userQuoteTokenAccount).
		SetBaseVault(poolInfo.BaseVault).
		SetQuoteVault(poolInfo.QuoteVault).
		SetOpenOrders(poolInfo.OpenOrders).
		SetTargetOrders(poolInfo.TargetOrders).
		SetMarketId(poolInfo.MarketId).
		SetMarketProgramId(poolInfo.MarketProgramId).
		SetMarketBaseVault(poolInfo.MarketBaseVault).
		SetMarketQuoteVault(poolInfo.MarketQuoteVault).
		SetMarketBids(poolInfo.MarketBids).
		SetMarketAsks(poolInfo.MarketAsks).
		SetMarketEventQueue(poolInfo.MarketEventQueue).
		SetMarketAuthority(poolInfo.MarketAuthority).
		SetBaseAmountIn(baseAmountIn).
		SetMinQuoteAmountOut(minQuoteAmountOut)
}
