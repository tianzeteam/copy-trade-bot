package core

import "github.com/gagliardetto/solana-go"

// 系统账户常量
var (
	PUMP_PROGRAM_ID                      = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")
	AMM_PROGRAM_ID                       = solana.MustPublicKeyFromBase58("pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA")
	EVENT_AUTHORITY                      = solana.MustPublicKeyFromBase58("Ce6TQqeHC9p8KetsN6JsjHK7UTZk7nasjjnr7XxXp9F1")
	GLOBAL_ACCOUNT                       = solana.MustPublicKeyFromBase58("4wTV1YmiEkRvAtNtsSGPtUrqRYQMe5SKy2uB4Jjaxnjf")
	FEE_RECIPIENT                        = solana.MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
	AMM_EVENT_AUTHORITY                  = solana.MustPublicKeyFromBase58("GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR")
	GLOBAL_CONFIG                        = solana.MustPublicKeyFromBase58("ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw")
	PROTOCOL_FEE_RECIPIENT               = solana.MustPublicKeyFromBase58("62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV")
	PROTOCOL_FEE_RECIPIENT_TOKEN_ACCOUNT = solana.MustPublicKeyFromBase58("94qWNrtmfn42h3ZjUZwWvK1MEo9uVmmrBPd2hpNjYDjb")
	ACCOUNT_SPACE                        = uint64(165)
	WSOL                                 = solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
)
