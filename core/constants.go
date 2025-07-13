package core

import (
	"github.com/gagliardetto/solana-go"
)

// 系统账户常量
var (
	OKX_ROUTER_V2_PROGRAM_ID             = solana.MustPublicKeyFromBase58("6m2CDdhRgxpH4WjvdzxAYbGxwdGUz5MziiL5jek2kBma")
	RAYDIUM_LAUNCHPAD_PROGRAM_ID         = solana.MustPublicKeyFromBase58("LanMV9sAd7wArD4vJFi2qDdfnVhFxYSUg6eADduJ3uj")
	PHOTON_ROUTER_PROGRAM_ID             = solana.MustPublicKeyFromBase58("BSfD6SHZigAfDWSjzD5Q41jw8LmKwtmjskPH9XW1mrRW")
	JUPITER_V6_PROGRAM_ID                = solana.MustPublicKeyFromBase58("JUP6LkbZbjS1jKKwapdHNy74zcZ3tLUZoi5QNyVTaV4")
	RAYDIUM_V4_PROGRAM_ID                = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
	RAYDIUM_AMM_ROUTEING                 = solana.MustPublicKeyFromBase58("routeUGWgWzqBWFcrCfv8tritsqukccJPu3q5GPP3xS")
	PUMP_PROGRAM_ID                      = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	AMM_PROGRAM_ID                       = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
	AMM_GLOBAL_CONFIG_ACCOUNT            = solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw")
	OPENBOOK_MARKET_PROGRAM_ID           = solana.MustPublicKeyFromBase58("srmqPvymJeFKQ4zGQed1GFppgkRHL9kaELCbyksJtPX")
	EVENT_AUTHORITY                      = solana.MustPublicKeyFromBase58("Ce6TQqeHC9p8KetsN6JsjHK7UTZk7nasjjnr7XxXp9F1")
	GLOBAL_ACCOUNT                       = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")
	FEE_RECIPIENT                        = solana.MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
	AMM_EVENT_AUTHORITY                  = solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR")
	PROTOCOL_FEE_RECIPIENT               = solana.MustPublicKeyFromBase58("JCRGumoE9Qi5BBgULTgdgTLjSgkCMSbF62ZZfGs84JeU")
	PROTOCOL_FEE_RECIPIENT_TOKEN_ACCOUNT = solana.MustPublicKeyFromBase58("DWpvfqzGWuVy9jVSKSShdM2733nrEsnnhsUStYbkj6Nn")
	ACCOUNT_SPACE                        = uint64(165)
	WSOL                                 = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

	GLOBAL_CONFIG_ACCOUNT = solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw") // 请替换为正确的地址
	RAYDIUM_AUTHORITY_v4  = "5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1"
)
