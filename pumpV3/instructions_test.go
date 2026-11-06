package pumpV3

import (
	"bytes"
	"testing"

	"github.com/gagliardetto/solana-go"
)

func testTradeAccounts() TradeAccounts {
	key := func(v byte) solana.PublicKey {
		var out solana.PublicKey
		out[0] = v
		return out
	}
	return TradeAccounts{
		Global:                             key(1),
		BaseMint:                           key(2),
		QuoteMint:                          WSOLMint,
		BaseTokenProgram:                   solana.TokenProgramID,
		QuoteTokenProgram:                  solana.TokenProgramID,
		AssociatedTokenProgram:             solana.SPLAssociatedTokenAccountProgramID,
		FeeRecipient:                       key(3),
		AssociatedQuoteFeeRecipient:        key(4),
		BuybackFeeRecipient:                key(5),
		AssociatedQuoteBuybackFeeRecipient: key(6),
		BondingCurve:                       key(7),
		AssociatedBaseBondingCurve:         key(8),
		AssociatedQuoteBondingCurve:        key(9),
		User:                               key(10),
		AssociatedBaseUser:                 key(11),
		AssociatedQuoteUser:                key(12),
		CreatorVault:                       key(13),
		AssociatedCreatorVault:             key(14),
		SharingConfig:                      key(15),
		GlobalVolumeAccumulator:            key(16),
		UserVolumeAccumulator:              key(17),
		AssociatedUserVolumeAccumulator:    key(18),
		FeeConfig:                          key(19),
		FeeProgram:                         key(20),
		SystemProgram:                      solana.SystemProgramID,
		EventAuthority:                     key(22),
		Program:                            key(23),
	}
}

func TestNewBuyExactQuoteInV2Instruction(t *testing.T) {
	acc := testTradeAccounts()
	ix, err := NewBuyExactQuoteInV2Instruction(123, 456, acc)
	if err != nil {
		t.Fatalf("NewBuyExactQuoteInV2Instruction() error = %v", err)
	}
	if ix.ProgramID() != ProgramID {
		t.Fatalf("program id = %s, want %s", ix.ProgramID(), ProgramID)
	}
	if got := len(ix.Accounts()); got != 27 {
		t.Fatalf("accounts = %d, want 27", got)
	}
	data, err := ix.Data()
	if err != nil {
		t.Fatalf("Data() error = %v", err)
	}
	if len(data) != 24 {
		t.Fatalf("data length = %d, want 24", len(data))
	}
	if !bytes.Equal(data[:8], InstructionBuyExactQuoteInV2[:]) {
		t.Fatalf("discriminator = %x, want %x", data[:8], InstructionBuyExactQuoteInV2)
	}
}

func TestBuyInstructionsRequireGlobalVolumeAccumulator(t *testing.T) {
	acc := testTradeAccounts()
	acc.GlobalVolumeAccumulator = solana.PublicKey{}
	if _, err := NewBuyV2Instruction(1, 1, acc); err == nil {
		t.Fatal("NewBuyV2Instruction() accepted a zero global volume accumulator")
	}
	if _, err := NewBuyExactQuoteInV2Instruction(1, 1, acc); err == nil {
		t.Fatal("NewBuyExactQuoteInV2Instruction() accepted a zero global volume accumulator")
	}
}
