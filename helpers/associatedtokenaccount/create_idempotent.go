package associatedtokenaccount

import (
	"errors"
	"fmt"

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	format "github.com/gagliardetto/solana-go/text/format"
	treeout "github.com/gagliardetto/treeout"
)

// CreateIdempotent 为指定钱包与代币 mint 创建关联代币账户（ATA）。
// 若账户已存在则静默跳过；仅当账户存在但归属者不同才报错。
//
// 跟单场景优先使用本指令而非 Create：机器人无法保证在下单前
// 一定完成 ATA 存在性查询，幂等语义可避免因账户已存在导致整笔交易失败。
type CreateIdempotent struct {
	Payer        solana.PublicKey `bin:"-" borsh_skip:"true"`
	Wallet       solana.PublicKey `bin:"-" borsh_skip:"true"`
	Mint         solana.PublicKey `bin:"-" borsh_skip:"true"`
	TokenProgram solana.PublicKey `bin:"-" borsh_skip:"true"`

	// [0] = [WRITE, SIGNER] Payer
	// ··········· 出资账户
	//
	// [1] = [WRITE] AssociatedTokenAccount
	// ··········· 待创建的 ATA 地址
	//
	// [2] = [] Wallet
	// ··········· ATA 的归属钱包
	//
	// [3] = [] TokenMint
	// ··········· 代币 mint
	//
	// [4] = [] SystemProgram
	// ··········· 系统程序
	//
	// [5] = [] TokenProgram
	// ··········· SPL Token 或 Token-2022 程序
	solana.AccountMetaSlice `bin:"-" borsh_skip:"true"`
}

// NewCreateIdempotentInstructionBuilder 创建 CreateIdempotent 指令构造器。
func NewCreateIdempotentInstructionBuilder() *CreateIdempotent {
	return &CreateIdempotent{
		TokenProgram: solana.TokenProgramID,
	}
}

// SetPayer 设置出资账户。
func (inst *CreateIdempotent) SetPayer(payer solana.PublicKey) *CreateIdempotent {
	inst.Payer = payer
	return inst
}

// SetWallet 设置 ATA 归属钱包。
func (inst *CreateIdempotent) SetWallet(wallet solana.PublicKey) *CreateIdempotent {
	inst.Wallet = wallet
	return inst
}

// SetMint 设置代币 mint。
func (inst *CreateIdempotent) SetMint(mint solana.PublicKey) *CreateIdempotent {
	inst.Mint = mint
	return inst
}

// SetTokenProgram 设置 token program（Token-2022 需显式指定）。
func (inst *CreateIdempotent) SetTokenProgram(tokenProgram solana.PublicKey) *CreateIdempotent {
	inst.TokenProgram = tokenProgram
	return inst
}

// SetAccounts 从已解析的账户列表回填字段（用于指令解码）。
func (inst *CreateIdempotent) SetAccounts(accounts []*solana.AccountMeta) error {
	inst.AccountMetaSlice = accounts
	if len(accounts) < 6 {
		return fmt.Errorf("insufficient accounts, CreateIdempotent requires at-least 6 accounts not %d", len(accounts))
	}
	inst.Payer = accounts[0].PublicKey
	inst.Wallet = accounts[2].PublicKey
	inst.Mint = accounts[3].PublicKey
	inst.TokenProgram = accounts[5].PublicKey
	return nil
}

// Build 使用标准 SPL Token Program 构造指令。
// 若已通过 SetTokenProgram 显式指定，则以指定值为准。
func (inst CreateIdempotent) Build() *Instruction {
	tokenProgram := inst.TokenProgram
	if tokenProgram.IsZero() {
		tokenProgram = solana.TokenProgramID
	}
	return inst.buildWithTokenProgram(tokenProgram)
}

// BuildV2 使用 Token-2022 Program 构造指令。
//
// Pump.fun V2 代币基于 Token-2022 发行，其 ATA 派生种子包含 token program，
// 因此必须用本方法而非 Build，否则会创建出错误地址的账户。
func (inst CreateIdempotent) BuildV2() *Instruction {
	return inst.buildWithTokenProgram(solana.Token2022ProgramID)
}

func (inst CreateIdempotent) buildWithTokenProgram(tokenProgram solana.PublicKey) *Instruction {
	associatedTokenAddress, _, _ := FindAssociatedTokenAddressWithProgram(
		inst.Wallet,
		inst.Mint,
		tokenProgram,
	)

	keys := []*solana.AccountMeta{
		{
			PublicKey:  inst.Payer,
			IsSigner:   true,
			IsWritable: true,
		},
		{
			PublicKey:  associatedTokenAddress,
			IsSigner:   false,
			IsWritable: true,
		},
		{
			PublicKey:  inst.Wallet,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  inst.Mint,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  solana.SystemProgramID,
			IsSigner:   false,
			IsWritable: false,
		},
		{
			PublicKey:  tokenProgram,
			IsSigner:   false,
			IsWritable: false,
		},
	}

	inst.AccountMetaSlice = keys

	return &Instruction{BaseVariant: bin.BaseVariant{
		Impl:   inst,
		TypeID: bin.TypeIDFromUint8(Instruction_CreateIdempotent),
	}}
}

// ValidateAndBuild 先校验再构造指令。
func (inst CreateIdempotent) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// Validate 校验必填字段。
func (inst *CreateIdempotent) Validate() error {
	if inst.Payer.IsZero() {
		return errors.New("payer not set")
	}
	if inst.Wallet.IsZero() {
		return errors.New("wallet not set")
	}
	if inst.Mint.IsZero() {
		return errors.New("mint not set")
	}
	tokenProgram := inst.TokenProgram
	if tokenProgram.IsZero() {
		tokenProgram = solana.TokenProgramID
	}
	if _, _, err := FindAssociatedTokenAddressWithProgram(inst.Wallet, inst.Mint, tokenProgram); err != nil {
		return fmt.Errorf("error while FindAssociatedTokenAddress: %w", err)
	}
	return nil
}

// EncodeToTree 渲染指令树。
func (inst *CreateIdempotent) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program(ProgramName, ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("CreateIdempotent")).
				ParentFunc(func(instructionBranch treeout.Branches) {
					instructionBranch.Child("Params[len=0]").ParentFunc(func(paramsBranch treeout.Branches) {})

					instructionBranch.Child("Accounts[len=6]").ParentFunc(func(accountsBranch treeout.Branches) {
						accountsBranch.Child(format.Meta("                 payer", inst.AccountMetaSlice.Get(0)))
						accountsBranch.Child(format.Meta("associatedTokenAddress", inst.AccountMetaSlice.Get(1)))
						accountsBranch.Child(format.Meta("                wallet", inst.AccountMetaSlice.Get(2)))
						accountsBranch.Child(format.Meta("             tokenMint", inst.AccountMetaSlice.Get(3)))
						accountsBranch.Child(format.Meta("         systemProgram", inst.AccountMetaSlice.Get(4)))
						accountsBranch.Child(format.Meta("          tokenProgram", inst.AccountMetaSlice.Get(5)))
					})
				})
		})
}

// MarshalWithEncoder ATA 指令体为空，判别符由 Instruction 写入。
func (inst CreateIdempotent) MarshalWithEncoder(encoder *bin.Encoder) error {
	return nil
}

// UnmarshalWithDecoder ATA 指令体为空。
func (inst *CreateIdempotent) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return nil
}

// NewCreateIdempotentInstruction 构造标准 SPL Token 的 CreateIdempotent 指令。
func NewCreateIdempotentInstruction(
	payer solana.PublicKey,
	walletAddress solana.PublicKey,
	splTokenMintAddress solana.PublicKey,
) *CreateIdempotent {
	return NewCreateIdempotentInstructionWithTokenProgram(
		payer,
		walletAddress,
		splTokenMintAddress,
		solana.TokenProgramID,
	)
}

// NewCreateIdempotentInstructionWithTokenProgram 构造指定 token program 的 CreateIdempotent 指令。
func NewCreateIdempotentInstructionWithTokenProgram(
	payer solana.PublicKey,
	walletAddress solana.PublicKey,
	splTokenMintAddress solana.PublicKey,
	tokenProgram solana.PublicKey,
) *CreateIdempotent {
	return NewCreateIdempotentInstructionBuilder().
		SetPayer(payer).
		SetWallet(walletAddress).
		SetMint(splTokenMintAddress).
		SetTokenProgram(tokenProgram)
}

// GetPayerAccount 返回出资账户元信息。
func (inst *CreateIdempotent) GetPayerAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

// GetAssociatedTokenAddressAccount 返回 ATA 账户元信息。
func (inst *CreateIdempotent) GetAssociatedTokenAddressAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

// GetWalletAccount 返回钱包账户元信息。
func (inst *CreateIdempotent) GetWalletAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

// GetMintAccount 返回 mint 账户元信息。
func (inst *CreateIdempotent) GetMintAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}
