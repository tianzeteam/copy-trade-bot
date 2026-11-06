package associatedtokenaccount

import (
	"errors"
	"fmt"

	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	format "github.com/gagliardetto/solana-go/text/format"
	treeout "github.com/gagliardetto/treeout"
)

// Create 为指定钱包与代币 mint 创建关联代币账户（ATA）。
// 若账户已存在则指令失败；需要幂等语义时请使用 CreateIdempotent。
type Create struct {
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

// NewCreateInstructionBuilder 创建 Create 指令构造器。
func NewCreateInstructionBuilder() *Create {
	return &Create{
		TokenProgram: solana.TokenProgramID,
	}
}

// SetPayer 设置出资账户。
func (inst *Create) SetPayer(payer solana.PublicKey) *Create {
	inst.Payer = payer
	return inst
}

// SetWallet 设置 ATA 归属钱包。
func (inst *Create) SetWallet(wallet solana.PublicKey) *Create {
	inst.Wallet = wallet
	return inst
}

// SetMint 设置代币 mint。
func (inst *Create) SetMint(mint solana.PublicKey) *Create {
	inst.Mint = mint
	return inst
}

// SetTokenProgram 设置 token program（Token-2022 需显式指定）。
func (inst *Create) SetTokenProgram(tokenProgram solana.PublicKey) *Create {
	inst.TokenProgram = tokenProgram
	return inst
}

// SetAccounts 从已解析的账户列表回填字段（用于指令解码）。
func (inst *Create) SetAccounts(accounts []*solana.AccountMeta) error {
	inst.AccountMetaSlice = accounts
	if len(accounts) < 6 {
		return fmt.Errorf("insufficient accounts, Create requires at-least 6 accounts not %d", len(accounts))
	}
	inst.Payer = accounts[0].PublicKey
	inst.Wallet = accounts[2].PublicKey
	inst.Mint = accounts[3].PublicKey
	inst.TokenProgram = accounts[5].PublicKey
	return nil
}

// Build 使用标准 SPL Token Program 构造指令。
// 若已通过 SetTokenProgram 显式指定，则以指定值为准。
func (inst Create) Build() *Instruction {
	tokenProgram := inst.TokenProgram
	if tokenProgram.IsZero() {
		tokenProgram = solana.TokenProgramID
	}
	return inst.buildWithTokenProgram(tokenProgram)
}

// BuildV2 强制使用 Token-2022 Program 构造指令，忽略 SetTokenProgram 的设置。
func (inst Create) BuildV2() *Instruction {
	return inst.buildWithTokenProgram(solana.Token2022ProgramID)
}

func (inst Create) buildWithTokenProgram(tokenProgram solana.PublicKey) *Instruction {
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
		TypeID: bin.TypeIDFromUint8(Instruction_Create),
	}}
}

// ValidateAndBuild 先校验再构造指令。
func (inst Create) ValidateAndBuild() (*Instruction, error) {
	if err := inst.Validate(); err != nil {
		return nil, err
	}
	return inst.Build(), nil
}

// Validate 校验必填字段。
func (inst *Create) Validate() error {
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
func (inst *Create) EncodeToTree(parent treeout.Branches) {
	parent.Child(format.Program(ProgramName, ProgramID)).
		ParentFunc(func(programBranch treeout.Branches) {
			programBranch.Child(format.Instruction("Create")).
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
func (inst Create) MarshalWithEncoder(encoder *bin.Encoder) error {
	return nil
}

// UnmarshalWithDecoder ATA 指令体为空。
func (inst *Create) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return nil
}

// NewCreateInstruction 构造标准 SPL Token 的 Create 指令。
func NewCreateInstruction(
	payer solana.PublicKey,
	walletAddress solana.PublicKey,
	splTokenMintAddress solana.PublicKey,
) *Create {
	return NewCreateInstructionBuilder().
		SetPayer(payer).
		SetWallet(walletAddress).
		SetMint(splTokenMintAddress)
}

// NewCreateInstructionWithTokenProgram 构造指定 token program 的 Create 指令。
func NewCreateInstructionWithTokenProgram(
	payer solana.PublicKey,
	walletAddress solana.PublicKey,
	splTokenMintAddress solana.PublicKey,
	tokenProgram solana.PublicKey,
) *Create {
	return NewCreateInstructionBuilder().
		SetPayer(payer).
		SetWallet(walletAddress).
		SetMint(splTokenMintAddress).
		SetTokenProgram(tokenProgram)
}

// GetPayerAccount 返回出资账户元信息。
func (inst *Create) GetPayerAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(0)
}

// GetAssociatedTokenAddressAccount 返回 ATA 账户元信息。
func (inst *Create) GetAssociatedTokenAddressAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(1)
}

// GetWalletAccount 返回钱包账户元信息。
func (inst *Create) GetWalletAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(2)
}

// GetMintAccount 返回 mint 账户元信息。
func (inst *Create) GetMintAccount() *solana.AccountMeta {
	return inst.AccountMetaSlice.Get(3)
}
