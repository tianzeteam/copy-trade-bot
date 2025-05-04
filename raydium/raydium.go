package raydium

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type Raydium struct {
	Connection     *rpc.Client
	UserPrivateKey *solana.PrivateKey
}

func NewRaydium(connection *rpc.Client) *Raydium {
	//var publicKey solana.PublicKey = solana.MustPublicKeyFromBase58("63vEsMh9SEuxJEQL9tVbbHzHtymXkyt5ArL1peVRYyic")
	var privateKey solana.PrivateKey = solana.MustPrivateKeyFromBase58("JWcsNtHoufA6oBE4cXjyh2gLLNfr2CCRDFXTCMMChuP355FSGKyZPkZXBgpyRAZCQCGYTJ4JwNGaCYSCE9gywiL") // 需要初始化
	r := &Raydium{
		Connection:     connection,
		UserPrivateKey: &privateKey,
	}

	return r
}
