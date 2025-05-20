package core

import (
	"github.com/gagliardetto/solana-go"
)

// 系统账户常量
var (
	RAYDIUM_V4_PROGRAM_ID                = solana.MustPublicKeyFromBase58("675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8")
	PUMP_PROGRAM_ID                      = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	AMM_PROGRAM_ID                       = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
	OPENBOOK_MARKET_PROGRAM_ID           = solana.MustPublicKeyFromBase58("srmqPvymJeFKQ4zGQed1GFppgkRHL9kaELCbyksJtPX")
	EVENT_AUTHORITY                      = solana.MustPublicKeyFromBase58("Ce6TQqeHC9p8KetsN6JsjHK7UTZk7nasjjnr7XxXp9F1")
	GLOBAL_ACCOUNT                       = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")
	FEE_RECIPIENT                        = solana.MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
	AMM_EVENT_AUTHORITY                  = solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR")
	PROTOCOL_FEE_RECIPIENT               = solana.MustPublicKeyFromBase58("FWsW1xNtWscwNmKv6wVsU1iTzRN6wmmk3MjxRP5tT7hz")
	PROTOCOL_FEE_RECIPIENT_TOKEN_ACCOUNT = solana.MustPublicKeyFromBase58("A9AKE7gPf3xn3rU3WhVJYpAz494izRzPeb1Fn8KdtAeu")
	ACCOUNT_SPACE                        = uint64(165)
	WSOL                                 = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")

	GLOBAL_CONFIG_ACCOUNT = solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw") // 请替换为正确的地址
	//EVENT_AUTHORITY_ACCOUNT = solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR") // 请替换为正确的地址

	RAYDIUM_AUTHORITY_v4 = "5Q544fKrFoe6tsEbD7S8EmxGTJYAKtTVhAW5Q5pge4j1"
)
