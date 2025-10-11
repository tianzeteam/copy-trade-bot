package types

import (
	//"copytrade/calculator"
	"copytrade/raydium/layouts"
	"time"

	"github.com/gagliardetto/solana-go"
)

// TradeMode 定义交易模式
type TradeMode string

const (
	Normal     TradeMode = "normal"
	Jito       TradeMode = "jito"
	NextBlock  TradeMode = "nextblock"
	Temportal  TradeMode = "temportal"
	BloXroute  TradeMode = "bloXroute"
	Zeroslot   TradeMode = "zeroslot"
	Blockrazor TradeMode = "blockrazor"
	Astralane  TradeMode = "astralane"
	FlashBlock TradeMode = "flashblock"
)

// ValidateMode 验证交易模式是否有效
func (m TradeMode) IsValid() bool {
	switch m {
	case Normal, Jito, NextBlock, Temportal, BloXroute, Zeroslot, Blockrazor, Astralane, FlashBlock:
		return true
	default:
		return false
	}
}

// String 实现 Stringer 接口
func (m TradeMode) String() string {
	return string(m)
}

// Raydium V4 存储AMM交易事件信息
type AmmRaydiumV4TradeInfo struct {
	UserAddress  solana.PublicKey // 用户地址
	SolAmount    uint64           // Sol数量
	TokenAmount  uint64           // Token数量
	IsBuy        bool             // 是否买入
	MintAddress  solana.PublicKey // Mint地址
	AmmPool      solana.PublicKey // AMM池地址
	TimeStamp    int64            // 时间戳
	OpenOrders   solana.PublicKey
	TargetOrders solana.PublicKey
	BaseVault    solana.PublicKey
	QuoteVault   solana.PublicKey
	MarketId     solana.PublicKey

	Version      uint32
	CurrentPrice float64   // 当前价格
	TradeTime    string    // 交易时间
	ReceivedAt   time.Time // 接收时间
	TxHash       string    // 交易哈希
	Slot         uint64    // 区块槽位
	Decimal      uint32
	SolReserve   uint64 // SOL储备量
}

// MarketState represents an OpenBook market state with additional metadata
type MarketState struct {
	// Market account public key
	MarketID solana.PublicKey
	// Market state data
	Layout layouts.MarketStateLayoutV3
	// Base token mint address
	BaseMint solana.PublicKey
	// Quote token mint address
	QuoteMint solana.PublicKey
	// Base token decimal places
	MarketEventQueue solana.PublicKey
	// Base vault address
	BaseVault solana.PublicKey
	// Quote vault address
	QuoteVault solana.PublicKey
	// Bids account address
	Bids solana.PublicKey
	// Asks account address
	Asks solana.PublicKey
	// Base lot size
	BaseLotSize uint64
	// Quote lot size
	QuoteLotSize uint64
	// Fee rate in basis points
	FeeRateBps uint64
}

// PumpAccounts 存储所有需要的账户地址
type PumpAccounts struct {
	BondingCurve            solana.PublicKey
	AssociatedBondingCurve  solana.PublicKey
	AssociatedTokenAccount  solana.PublicKey
	CreatorVaultPda         solana.PublicKey // 代币创建者金库PDA地址
	GlobalVolumeAccumulator solana.PublicKey // 全局成交量累加器地址
	UserVolumeAccumulator   solana.PublicKey // 用户成交量累加器地址
}

// AmmAccounts 存储AMM相关的账户地址
type AmmAccounts struct {
	Pool                      solana.PublicKey // AMM池地址
	Mint                      solana.PublicKey // 基础代币地址
	BaseMint                  solana.PublicKey // 基础代币地址
	QuoteMint                 solana.PublicKey // 报价代币地址
	PoolBaseTokenAccount      solana.PublicKey // 池中基础代币账户
	PoolQuoteTokenAccount     solana.PublicKey // 池中报价代币账户
	UserBaseTokenAccount      solana.PublicKey // 用户的基础代币账户
	UserQuoteTokenAccount     solana.PublicKey // 用户的报价代币账户
	WsolTokenAccount          solana.PublicKey // WSOL账户
	CoinCreator               solana.PublicKey // 代币创建者地址
	CoinCreatorVaultAuthority solana.PublicKey // 代币创建者金库授权地址
	CoinCreatorVaultAta       solana.PublicKey // 代币创建者金库ATA地址
	GlobalVolumeAccumulator   solana.PublicKey // 全局成交量累加器地址
	UserVolumeAccumulator     solana.PublicKey // 用户成交量累加器地址
}

type RaydiumV4Accounts struct {
	Pool                  solana.PublicKey
	ID                    solana.PublicKey
	BaseMint              solana.PublicKey
	QuoteMint             solana.PublicKey
	UserBaseTokenAccount  solana.PublicKey
	UserQuoteTokenAccount solana.PublicKey
	LpMint                solana.PublicKey
	BaseDecimals          uint64
	QuoteDecimals         uint64
	LpDecimals            uint64
	Version               uint64
	ProgramId             solana.PublicKey
	Authority             solana.PublicKey
	OpenOrders            solana.PublicKey
	TargetOrders          solana.PublicKey
	BaseVault             solana.PublicKey
	QuoteVault            solana.PublicKey
	//WithdrawQueue      solana.PublicKey
	LpVault       solana.PublicKey
	WithdrawQueue solana.PublicKey

	MarketVersion      uint64
	MarketProgramId    solana.PublicKey
	MarketId           solana.PublicKey
	MarketAuthority    solana.PublicKey
	MarketBaseVault    solana.PublicKey
	MarketQuoteVault   solana.PublicKey
	MarketBids         solana.PublicKey
	MarketAsks         solana.PublicKey
	MarketEventQueue   solana.PublicKey
	LookupTableAccount solana.PublicKey
}

// RaydiumLaunchpadAccounts stores all the necessary accounts for interacting with the Raydium Launchpad.
type RaydiumLaunchpadAccounts struct {
	Payer                 solana.PublicKey
	Authority             solana.PublicKey
	GlobalConfig          solana.PublicKey // NOTE: This needs to be fetched or provided
	PlatformConfig        solana.PublicKey // NOTE: This needs to be fetched or provided
	PoolState             solana.PublicKey
	UserBaseTokenAccount  solana.PublicKey
	UserQuoteTokenAccount solana.PublicKey
	BaseVault             solana.PublicKey
	QuoteVault            solana.PublicKey
	BaseTokenMint         solana.PublicKey
	QuoteTokenMint        solana.PublicKey
	BaseTokenProgram      solana.PublicKey
	QuoteTokenProgram     solana.PublicKey
	EventAuthority        solana.PublicKey
	Program               solana.PublicKey
	ClaimPlatformFee      solana.PublicKey
	ClaimCreatorFee       solana.PublicKey
}

// TradeAccounts 定义交易所需的账户
type TradeAccounts struct {
	User                      solana.PublicKey
	Mint                      solana.PublicKey
	BaseMint                  solana.PublicKey
	QuoteMint                 solana.PublicKey
	Pool                      solana.PublicKey
	UserBaseTokenAccount      solana.PublicKey
	UserQuoteTokenAccount     solana.PublicKey
	PoolBaseTokenAccount      solana.PublicKey
	PoolQuoteTokenAccount     solana.PublicKey
	BondingCurve              solana.PublicKey
	AssociatedBondingCurve    solana.PublicKey
	AssociatedTokenAccount    solana.PublicKey
	CreatorVault              solana.PublicKey
	CoinCreatorVaultAuthority solana.PublicKey

	RaydiumV4Accounts       *RaydiumV4Accounts
	LaunchpadAccounts       *RaydiumLaunchpadAccounts
	RaydiumCPMMAccounts     *RaydiumCPMMAccounts
	MeteoraDbcAccounts      *MeteoraDbcAccounts
	GlobalVolumeAccumulator solana.PublicKey // 全局成交量累加器地址
	UserVolumeAccumulator   solana.PublicKey // 用户成交量累加器地址
	NonceAccount            solana.PublicKey // nonce账户
}

// RaydiumCPMMAccounts stores all the necessary accounts for interacting with the Raydium CPMM.
type RaydiumCPMMAccounts struct {
	Payer              solana.PublicKey
	Authority          solana.PublicKey
	AmmConfig          solana.PublicKey
	PoolState          solana.PublicKey
	InputTokenAccount  solana.PublicKey
	OutputTokenAccount solana.PublicKey
	InputVault         solana.PublicKey
	OutputVault        solana.PublicKey
	ObservationState   solana.PublicKey
	InputTokenMint     solana.PublicKey
	OutputTokenMint    solana.PublicKey
	InputTokenProgram  solana.PublicKey
	OutputTokenProgram solana.PublicKey
	Program            solana.PublicKey
}

// MeteoraDbcAccounts stores all the necessary accounts for interacting with Meteora DBC.
type MeteoraDbcAccounts struct {
	Payer                solana.PublicKey
	Config               solana.PublicKey
	PoolAuthority        solana.PublicKey
	Pool                 solana.PublicKey
	InputTokenAccount    solana.PublicKey
	OutputTokenAccount   solana.PublicKey
	InputVault           solana.PublicKey
	OutputVault          solana.PublicKey
	InputTokenMint       solana.PublicKey
	OutputTokenMint      solana.PublicKey
	InputTokenProgram    solana.PublicKey
	OutputTokenProgram   solana.PublicKey
	Authority            solana.PublicKey
	ReferralTokenAccount solana.PublicKey
	Program              solana.PublicKey
}

// FeeConfig 定义费用相关配置
type FeeConfig struct {
	ComputeUnits uint32
	Slippage     float64 // 滑点
	PriorityFee  uint64  // 优先级费用（lamports）
}

// TradeParams 定义所有交易模式共用的基础参数
type TradeParams struct {
	Accounts     TradeAccounts
	MaxSolAmount uint64
	Amount       uint64
	Cost         uint64 // 买入时是最大花费，卖出时是最小获得
	Blockhash    solana.Hash
	FeeConfig    FeeConfig
}

// ZeroslotParams 定义 Zeroslot 模式特有的参数
type ZeroslotParams struct {
	Enable    bool
	BundleUrl string
	RpcUrl    string
	ApiKeys   []string
	TipAmount uint64
}

// NextBlockParams 定义 NextBlock 模式特有的参数
type NextBlockParams struct {
	BundleUrl string
	Enable    bool
	ApiKey    string
	TipAmount uint64
}

// JitoParams 定义 Jito 模式特有的参数
type JitoParams struct {
	Enable    bool
	BundleUrl string
	TipAmount uint64
}

// TemportalParams 定义 Temportal 模式特有的参数
type TemporalParams struct {
	Enable    bool
	BundleUrl string
	ApiKey    string
	TipAmount uint64
}

// BloXrouteParams 定义 BloXroute 模式特有的参数
type BloXrouteParams struct {
	Enable    bool
	RpcUrl    string
	ApiKeys   []string
	TipAmount uint64
}

// BlockrazorParams 定义 Blockrazor 模式特有的参数
type BlockrazorParams struct {
	Enable         bool
	RpcUrl         string // 交易提交端点
	HealthEndpoint string // 健康检查端点
	ApiKey         string // API 密钥
	Mode           string // 交易模式 (fast, normal, etc.)
	TipAmount      uint64 // 小费金额
}

// AnsParams 定义 ANS (Astralane) 模式特有的参数
type AstralaneParams struct {
	Enable    bool
	RpcUrl    string
	ApiKey    string
	TipAmount uint64
}

// FlashBlockParams 定义 FlashBlock 模式特有的参数
type FlashBlockParams struct {
	Enable    bool
	RpcUrl    string
	ApiKey    string
	TipAmount uint64
}

// OrderParams 定义完整的交易参数（适用于买入和卖出）
type OrderParams struct {
	TradeParams                        // 基础参数
	NextBlockParams  *NextBlockParams  // NextBlock 特有参数
	JitoParams       *JitoParams       // Jito 特有参数
	TemporalParams   *TemporalParams   // Temportal 特有参数
	BloXrouteParams  *BloXrouteParams  // BloXroute 特有参数
	ZeroslotParams   *ZeroslotParams   // Zeroslot 特有参数
	BlockrazorParams *BlockrazorParams // Blockrazor 特有参数
	AstralaneParams  *AstralaneParams  // ANS (Astralane) 特有参数
	FlashBlockParams *FlashBlockParams // FlashBlock 特有参数
}

// TradeMessage 交易消息结构
type TradeMessage struct {
	Action   string `json:"action"`   // 操作类型：buy, sell
	Mint     string `json:"mint"`     // 代币地址
	Platform string `json:"platform"` // 平台：pump, pumpamm, raydiumV4, cpmm, launchpad
}

// ClosePositionMessage 平仓消息结构
type ClosePositionMessage struct {
	MintAddress  string  `json:"mint_address"`  // 代币地址
	CurrentPrice float64 `json:"current_price"` // 当前价格
	Platform     string  `json:"platform"`      // 平台：pump, pumpamm, raydiumV4, cpmm, launchpad
	MessageType  string  `json:"message_type"`  // 消息类型：delay_sell, stop_loss, take_profit
	Reason       string  `json:"reason"`        // 平仓原因
	CopyAddress  string  `json:"copy_address"`  // 跟单钱包地址
}
