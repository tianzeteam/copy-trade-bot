// Package associatedtokenaccount 提供 SPL Associated Token Account 程序的指令构造能力。
//
// 存在的理由：go.mod 锁定的 solana-go v1.12.0 只实现了 Create 指令，
// 缺少 CreateIdempotent，也没有 Token-2022 版本的构造方法（BuildV2）。
// 本项目在 Pump.fun V2（Token-2022）等场景下必须创建幂等 ATA，
// 因此在仓库内自带一份与上游 v1.24.0 行为一致的实现，避免依赖
// 未提交的本地 vendor 补丁。
//
// 包名与上游保持一致，调用方仅需替换 import 路径：
//
//	associatedtokenaccount "copytrade/helpers/associatedtokenaccount"
package associatedtokenaccount

import (
	"bytes"
	"fmt"

	spew "github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"
	solana "github.com/gagliardetto/solana-go"
	text "github.com/gagliardetto/solana-go/text"
	treeout "github.com/gagliardetto/treeout"
)

// ProgramID 是 SPL Associated Token Account 程序地址。
var ProgramID solana.PublicKey = solana.SPLAssociatedTokenAccountProgramID

// SetProgramID 覆盖默认程序地址（例如测试网自定义部署）。
func SetProgramID(pubkey solana.PublicKey) {
	ProgramID = pubkey
	solana.RegisterInstructionDecoder(ProgramID, registryDecodeInstruction)
}

// ProgramName 用于日志与指令树渲染。
const ProgramName = "AssociatedTokenAccount"

func init() {
	solana.RegisterInstructionDecoder(ProgramID, registryDecodeInstruction)
}

const (
	// Instruction_Create 创建 ATA，若账户已存在则报错。
	Instruction_Create uint8 = iota

	// Instruction_CreateIdempotent 创建 ATA，若账户已存在则静默跳过。
	Instruction_CreateIdempotent
)

// InstructionIDToName 返回指令 ID 对应的名称。
func InstructionIDToName(id uint8) string {
	switch id {
	case Instruction_Create:
		return "Create"
	case Instruction_CreateIdempotent:
		return "CreateIdempotent"
	default:
		return ""
	}
}

// Instruction 是 ATA 程序指令的变体容器。
type Instruction struct {
	bin.BaseVariant
}

// EncodeToTree 渲染指令树，便于调试输出。
func (inst *Instruction) EncodeToTree(parent treeout.Branches) {
	if enToTree, ok := inst.Impl.(text.EncodableToTree); ok {
		enToTree.EncodeToTree(parent)
	} else {
		parent.Child(spew.Sdump(inst))
	}
}

// InstructionImplDef 使用单字节 u8 作为指令判别符（ATA 程序非 Anchor 程序）。
var InstructionImplDef = bin.NewVariantDefinition(
	bin.Uint8TypeIDEncoding,
	[]bin.VariantType{
		{Name: "Create", Type: (*Create)(nil)},
		{Name: "CreateIdempotent", Type: (*CreateIdempotent)(nil)},
	},
)

// ProgramID 返回指令所属程序。
func (inst *Instruction) ProgramID() solana.PublicKey {
	return ProgramID
}

// Accounts 返回指令账户列表。
func (inst *Instruction) Accounts() (out []*solana.AccountMeta) {
	return inst.Impl.(solana.AccountsGettable).GetAccounts()
}

// Data 返回指令数据，即单字节指令判别符。
func (inst *Instruction) Data() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := bin.NewBinEncoder(buf).Encode(inst); err != nil {
		return nil, fmt.Errorf("unable to encode instruction: %w", err)
	}
	return buf.Bytes(), nil
}

// TextEncode 支持 solana-go 的文本编码接口。
func (inst *Instruction) TextEncode(encoder *text.Encoder, option *text.Option) error {
	return encoder.Encode(inst.Impl, option)
}

// UnmarshalWithDecoder 反序列化指令。
func (inst *Instruction) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	return inst.BaseVariant.UnmarshalBinaryVariant(decoder, InstructionImplDef)
}

// MarshalWithEncoder 序列化指令：先写单字节判别符，再写指令体（ATA 指令体为空）。
func (inst Instruction) MarshalWithEncoder(encoder *bin.Encoder) error {
	if err := encoder.WriteUint8(inst.TypeID[0]); err != nil {
		return fmt.Errorf("unable to write variant type: %w", err)
	}
	return encoder.Encode(inst.Impl)
}

func registryDecodeInstruction(accounts []*solana.AccountMeta, data []byte) (interface{}, error) {
	inst, err := DecodeInstruction(accounts, data)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

// DecodeInstruction 从账户列表与指令数据解码出 ATA 指令。
func DecodeInstruction(accounts []*solana.AccountMeta, data []byte) (*Instruction, error) {
	inst := new(Instruction)
	if err := bin.NewBinDecoder(data).Decode(inst); err != nil {
		return nil, fmt.Errorf("unable to decode instruction: %w", err)
	}
	if v, ok := inst.Impl.(solana.AccountsSettable); ok {
		if err := v.SetAccounts(accounts); err != nil {
			return nil, fmt.Errorf("unable to set accounts for instruction: %w", err)
		}
	}
	return inst, nil
}

// FindAssociatedTokenAddressWithProgram 按指定 token program 派生 ATA 地址。
//
// seeds = [wallet, tokenProgram, mint]，program = Associated Token Account 程序。
// 标准 SPL Token 与 Token-2022 派生出的地址不同，因此 Pump.fun V2
// （Token-2022）必须使用本函数而非 solana.FindAssociatedTokenAddress。
func FindAssociatedTokenAddressWithProgram(
	wallet solana.PublicKey,
	splTokenMintAddress solana.PublicKey,
	tokenProgram solana.PublicKey,
) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{
			wallet.Bytes(),
			tokenProgram.Bytes(),
			splTokenMintAddress.Bytes(),
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
}
