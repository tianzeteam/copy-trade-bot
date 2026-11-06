package associatedtokenaccount

import (
	"testing"

	solana "github.com/gagliardetto/solana-go"
)

var (
	testWallet = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	testMint   = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
)

func TestCreateIdempotentBuildEncoding(t *testing.T) {
	inst := NewCreateIdempotentInstructionBuilder().
		SetPayer(testWallet).
		SetWallet(testWallet).
		SetMint(testMint).
		Build()

	data, err := inst.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if len(data) != 1 || data[0] != Instruction_CreateIdempotent {
		t.Fatalf("expected data [%d], got %v", Instruction_CreateIdempotent, data)
	}

	accounts := inst.Accounts()
	if len(accounts) != 6 {
		t.Fatalf("expected 6 accounts, got %d", len(accounts))
	}
	if !accounts[0].IsSigner || !accounts[0].IsWritable {
		t.Fatal("payer must be signer and writable")
	}
	if !accounts[1].IsWritable || accounts[1].IsSigner {
		t.Fatal("ata must be writable and non-signer")
	}
	if !accounts[5].PublicKey.Equals(solana.TokenProgramID) {
		t.Fatalf("expected SPL Token program, got %s", accounts[5].PublicKey)
	}
}

func TestCreateIdempotentBuildV2UsesToken2022(t *testing.T) {
	builder := NewCreateIdempotentInstructionBuilder().
		SetPayer(testWallet).
		SetWallet(testWallet).
		SetMint(testMint)

	v1 := builder.Build()
	v2 := builder.BuildV2()

	v1Accounts := v1.Accounts()
	v2Accounts := v2.Accounts()

	if !v1Accounts[5].PublicKey.Equals(solana.TokenProgramID) {
		t.Fatalf("Build() must use SPL Token program, got %s", v1Accounts[5].PublicKey)
	}
	if !v2Accounts[5].PublicKey.Equals(solana.Token2022ProgramID) {
		t.Fatalf("BuildV2() must use Token-2022 program, got %s", v2Accounts[5].PublicKey)
	}
	// ATA 派生种子包含 token program，两种 program 必须得到不同地址。
	if v1Accounts[1].PublicKey.Equals(v2Accounts[1].PublicKey) {
		t.Fatal("Token-2022 ATA must differ from SPL Token ATA")
	}

	expected, _, err := FindAssociatedTokenAddressWithProgram(testWallet, testMint, solana.Token2022ProgramID)
	if err != nil {
		t.Fatalf("FindAssociatedTokenAddressWithProgram error: %v", err)
	}
	if !v2Accounts[1].PublicKey.Equals(expected) {
		t.Fatalf("BuildV2 ata = %s, want %s", v2Accounts[1].PublicKey, expected)
	}
}

func TestCreateBuildEncoding(t *testing.T) {
	inst := NewCreateInstruction(testWallet, testWallet, testMint).Build()

	data, err := inst.Data()
	if err != nil {
		t.Fatalf("Data() error: %v", err)
	}
	if len(data) != 1 || data[0] != Instruction_Create {
		t.Fatalf("expected data [%d], got %v", Instruction_Create, data)
	}
	if len(inst.Accounts()) != 6 {
		t.Fatalf("expected 6 accounts, got %d", len(inst.Accounts()))
	}
}

func TestValidateRejectsMissingFields(t *testing.T) {
	if err := (NewCreateIdempotentInstructionBuilder().SetWallet(testWallet).SetMint(testMint)).Validate(); err == nil {
		t.Fatal("expected error when payer missing")
	}
	if err := (NewCreateIdempotentInstructionBuilder().SetPayer(testWallet).SetMint(testMint)).Validate(); err == nil {
		t.Fatal("expected error when wallet missing")
	}
	if err := (NewCreateIdempotentInstructionBuilder().SetPayer(testWallet).SetWallet(testWallet)).Validate(); err == nil {
		t.Fatal("expected error when mint missing")
	}
}

func TestProgramIDMatchesUpstream(t *testing.T) {
	want := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	if !ProgramID.Equals(want) {
		t.Fatalf("ProgramID = %s, want %s", ProgramID, want)
	}
	if !solana.SPLAssociatedTokenAccountProgramID.Equals(want) {
		t.Fatalf("solana.SPLAssociatedTokenAccountProgramID = %s, want %s", solana.SPLAssociatedTokenAccountProgramID, want)
	}
}
