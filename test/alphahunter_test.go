package test

import (
	"log"
	"testing"

	"github.com/gagliardetto/solana-go"
	_ "github.com/joho/godotenv/autoload"
)

var (
	//shouldProxy                       = strings.Contains(os.Getenv("PROXY_URL"), "http")
	privateKey solana.PrivateKey = solana.MustPrivateKeyFromBase58("JWcsNtHoufA6oBE4cXjyh2gLLNfr2CCRDFXTCMMChuP355FSGKyZPkZXBgpyRAZCQCGYTJ4JwNGaCYSCE9gywiL") // 需要初始化
	myWallet   solana.PublicKey  = solana.MustPublicKeyFromBase58("63vEsMh9SEuxJEQL9tVbbHzHtymXkyt5ArL1peVRYyic")
	//feeMicroLamport uint64            = 200000
	buySolToLamport float64 = 0.0001 // 乘以 1e9 将 Sol 转换为 lamports
	//jitoEnable      bool              = true
)

func setupSuite(tb testing.TB) func(tb testing.TB) {
	log.Println("setup suite")

	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
	}
}

func TestTokenAmount(t *testing.T) {
	/* // Using the new config-based approach
	config := &utils.Config{
		PrivateKey:     privateKey.String(),
		RPCUrl:         "https://mainnet.helius-rpc.com/?api-key=e3e379af-d66c-4978-b75d-8f29b3e2c141",
		BuyPriorityFee: 0.0002, // 200000 microlamports
		SellDelayTime:  0,
		Jito: utils.ModeConfig{
			Enable: true,
		},
	}

	tradeBot, err := bot.NewBot(config)
	if err != nil {
		log.Fatal(err)
	}

	var mintAddr solana.PublicKey = solana.MustPublicKeyFromBase58("4Xj4yytpSB9GjvgSgycyseAaoxnfgq78QeLR1z6apump")
	coin := bot.NewCoin(privateKey, myWallet, mintAddr, buySolToLamport, uint64(500000))
	tradeBot.trader.Buy(context.Background(), types.OrderParams{
		TradeParams: types.TradeParams{
			Accounts: types.Accounts{
				Mint: mintAddr,
			},
			Amount: 500000,
		},
	}) */
}
