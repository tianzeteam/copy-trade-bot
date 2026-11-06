package calculator

import (
	"context"
	"testing"

	"copytrade/global"
	pumpV3 "copytrade/pumpV3"

	"github.com/gagliardetto/solana-go"
)

// seedFeeConfigForTest 预置费率缓存，使账户派生完全离线：
// buyback recipient 名单取自这里注入的 Global。
func seedFeeConfigForTest(t *testing.T) solana.PublicKey {
	t.Helper()
	buyback := pumpV3.GlobalAccount // 任意合法公钥即可，名单来自注入的 Global 模拟
	var bb [8]solana.PublicKey
	bb[0] = buyback
	global.SetPumpFeeConfig(&pumpV3.FeeConfig{}, &pumpV3.Global{BuybackFeeRecipients: bb})
	return buyback
}

// TestCalculatePumpV3Accounts_GoldenOnChain 使用真实链上交易数据（Token-2022 币）
// 校验统一交易接口账户派生。黄金值来自该笔交易实际使用的账户地址。
func TestCalculatePumpV3Accounts_GoldenOnChain(t *testing.T) {
	accountsCalc := NewAccountsCalculator("http://localhost:1") // 纯派生，不发起 RPC 请求

	creator := solana.MustPublicKeyFromBase58("HmgSia9qke4yYRRYQaXFs6dj4Nt7BSu4UNbaWymsTeS6")
	mint := solana.MustPublicKeyFromBase58("HRwsEGCErPEDDGFZAbXdezhmimjLi2Ge18GodUUwpump")
	user := solana.MustPublicKeyFromBase58("cwhihmydRGfmG9BX5i5Ame26NBtzYZbTX1BgoV2X43v")
	feeRecipient := pumpV3.GlobalAccount
	buybackRecipient := seedFeeConfigForTest(t)

	acc, err := accountsCalc.CalculatePumpV3Accounts(
		context.Background(), creator, mint, user,
		solana.Token2022ProgramID, pumpV3.WSOLMint, feeRecipient,
	)
	if err != nil {
		t.Fatalf("CalculatePumpV3Accounts 失败: %v", err)
	}

	// 链上交易实际使用的账户（与原 V2 测试同一笔交易）。
	golden := []struct {
		name string
		got  solana.PublicKey
		want solana.PublicKey
	}{
		{"AssociatedBaseUser", acc.AssociatedBaseUser, solana.MustPublicKeyFromBase58("6id4Fkm5hbyHsEALxqHo4x7qnfbsRVPt3goj4g3yTSnu")},
		{"CreatorVault", acc.CreatorVault, solana.MustPublicKeyFromBase58("BhnSW3H1t7Bh2mVf77L5KFhUzYxn3eKYmQqd2D9qAd67")},
		{"GlobalVolumeAccumulator", acc.GlobalVolumeAccumulator, solana.MustPublicKeyFromBase58("Hq2wp8uJ9jCPsYgNHex8RtqdvMPfVGoYwjvF1ATiwn2Y")},
		{"UserVolumeAccumulator", acc.UserVolumeAccumulator, solana.MustPublicKeyFromBase58("3r7LTaSH8QkJppauDuxGANmxXh6FgdDCXZ5Jwb3ZKgDF")},
	}
	for _, g := range golden {
		if g.got != g.want {
			t.Errorf("%s = %s, 期望 %s", g.name, g.got, g.want)
		}
	}

	// 其余派生账户必须与 pumpV3 的 PDA / ATA 规则一致。
	wantBondingCurve, err := pumpV3.BondingCurvePDA(mint)
	if err != nil {
		t.Fatal(err)
	}
	if acc.BondingCurve != wantBondingCurve {
		t.Errorf("BondingCurve = %s, 期望 %s", acc.BondingCurve, wantBondingCurve)
	}
	wantBaseBCAta, err := pumpV3.AssociatedTokenAccount(wantBondingCurve, mint, solana.Token2022ProgramID)
	if err != nil {
		t.Fatal(err)
	}
	if acc.AssociatedBaseBondingCurve != wantBaseBCAta {
		t.Errorf("AssociatedBaseBondingCurve = %s, 期望 %s", acc.AssociatedBaseBondingCurve, wantBaseBCAta)
	}
	wantSharing, err := pumpV3.SharingConfigPDA(mint)
	if err != nil {
		t.Fatal(err)
	}
	if acc.SharingConfig != wantSharing {
		t.Errorf("SharingConfig = %s, 期望 %s", acc.SharingConfig, wantSharing)
	}
	wantFeeConfig, err := pumpV3.FeeConfigPDA()
	if err != nil {
		t.Fatal(err)
	}
	if acc.FeeConfigAccount != wantFeeConfig {
		t.Errorf("FeeConfigAccount = %s, 期望 %s", acc.FeeConfigAccount, wantFeeConfig)
	}

	// 协议固定地址。
	if acc.Program != pumpV3.ProgramID {
		t.Errorf("Program = %s, 期望 %s", acc.Program, pumpV3.ProgramID)
	}
	if acc.EventAuthority != pumpV3.EventAuthority {
		t.Errorf("EventAuthority = %s, 期望 %s", acc.EventAuthority, pumpV3.EventAuthority)
	}
	if acc.FeeProgram != pumpV3.FeeProgramID {
		t.Errorf("FeeProgram = %s, 期望 %s", acc.FeeProgram, pumpV3.FeeProgramID)
	}
	if acc.QuoteTokenProgram != solana.TokenProgramID {
		t.Errorf("QuoteTokenProgram = %s, 期望 SPL Token", acc.QuoteTokenProgram)
	}
	if acc.FeeRecipient != feeRecipient {
		t.Errorf("FeeRecipient = %s, 期望 %s", acc.FeeRecipient, feeRecipient)
	}
	if acc.BuybackFeeRecipient != buybackRecipient {
		t.Errorf("BuybackFeeRecipient = %s, 期望来自链上 Global 名单 %s", acc.BuybackFeeRecipient, buybackRecipient)
	}
}

// TestCalculatePumpV3Accounts_Scoping 校验账户的派生维度：
// 曲线类账户与用户无关，用户类账户与 mint 无关。
func TestCalculatePumpV3Accounts_Scoping(t *testing.T) {
	accountsCalc := NewAccountsCalculator("http://localhost:1")
	ctx := context.Background()

	creator := solana.MustPublicKeyFromBase58("F5X7fkeLhB1wELU1cnfgR4g4nDpLdA3J2QgAkfVQrxRy")
	mint1 := solana.MustPublicKeyFromBase58("GkCfs9n8jLvuviiToewryCFa2rHQGKQv9FdnhTUEJtCH")
	mint2 := solana.MustPublicKeyFromBase58("6XaLsZGcBN9nfHzsxGX5KL25bYbFn5n8NiCLSzRAbonk")
	user1 := solana.MustPublicKeyFromBase58("FkyWzLirQxLRrNqGd1RtWNWDxymh9TftJSP8d8xeTtpB")
	user2 := solana.MustPublicKeyFromBase58("AwiFZDp7ee2QPbwUbGcpDSQ8hWz4a2qsNfqa13noY1EL")
	feeRecipient := pumpV3.GlobalAccount
	seedFeeConfigForTest(t)

	derive := func(mint, user solana.PublicKey) *struct{ BondingCurve, AssociatedBaseUser, UserVolumeAccumulator solana.PublicKey } {
		acc, err := accountsCalc.CalculatePumpV3Accounts(ctx, creator, mint, user, solana.TokenProgramID, pumpV3.WSOLMint, feeRecipient)
		if err != nil {
			t.Fatalf("CalculatePumpV3Accounts 失败: %v", err)
		}
		return &struct{ BondingCurve, AssociatedBaseUser, UserVolumeAccumulator solana.PublicKey }{
			acc.BondingCurve, acc.AssociatedBaseUser, acc.UserVolumeAccumulator,
		}
	}

	a1 := derive(mint1, user1)
	a2 := derive(mint1, user2)
	b1 := derive(mint2, user1)

	if a1.BondingCurve != a2.BondingCurve {
		t.Errorf("BondingCurve 不应随用户变化: %s vs %s", a1.BondingCurve, a2.BondingCurve)
	}
	if a1.AssociatedBaseUser == a2.AssociatedBaseUser {
		t.Errorf("AssociatedBaseUser 应随用户变化")
	}
	if a1.UserVolumeAccumulator == a2.UserVolumeAccumulator {
		t.Errorf("UserVolumeAccumulator 应随用户变化")
	}
	if a1.BondingCurve == b1.BondingCurve {
		t.Errorf("BondingCurve 应随 mint 变化")
	}
	if a1.UserVolumeAccumulator != b1.UserVolumeAccumulator {
		t.Errorf("UserVolumeAccumulator 不应随 mint 变化: %s vs %s", a1.UserVolumeAccumulator, b1.UserVolumeAccumulator)
	}
}
