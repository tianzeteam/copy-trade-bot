package bot

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"copytrade/core"
	"copytrade/logger"
	"copytrade/sdk"
	"copytrade/utils"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

type Bot struct {
	Config      *utils.Config
	Trader      *core.TraderImpl
	rpcClient   *rpc.Client
	nonceClient *client.Client
	sdkClient   *sdk.TradeSubscriptionClient

	feeMicroLamport uint64
	enableJito      bool
	delaySecond     int64
	//buyAmountLamport uint64           // amount of coins we buy for each coin (in lamports)
	pendingCoins     map[string]*Coin // coins which we will attempt to buy, but have yet to be purchased
	pendingCoinsLock sync.Mutex
	coinsToBuy       chan *Coin
	coinsToSell      chan string

	// skipATALookup skips looking up if the ATA exists. Useful for debugging & attempting to purchase coins we already have owned.
	// in prod, should always be set to `true` since we should never have ATA for new coins.
	SkipATALookup bool

	Blockhash     *solana.Hash
	balanceNeeded uint64

	// 卖出策略
	sellStrategy SellStrategy

	// 盈利目标监控器
	profitMonitor *ProfitTargetMonitor

	// OpenBook market states map, with baseMint as key
	marketStates     map[string]*MarketState
	marketStatesLock sync.RWMutex
}

func (b *Bot) status(msg interface{}) {
	logger.Info("Bot ", fmt.Sprintf("%v", msg))
}

func (b *Bot) statusy(msg interface{}) {
	logger.Info("Bot (Y) ", fmt.Sprintf("%v", msg))
}

func (b *Bot) statusg(msg interface{}) {
	logger.Info("Bot (G) ", fmt.Sprintf("%v", msg))
}

func (b *Bot) statusr(msg interface{}) {
	logger.Info("Bot (R) ", fmt.Sprintf("%v", msg))
}

type Coin struct {
	pickupTime             time.Time // used to make sure duration / timings are good
	privateKey             solana.PrivateKey
	publicKey              solana.PublicKey
	mintAddr               solana.PublicKey
	tokenBondingCurve      solana.PublicKey
	associatedBondingCurve solana.PublicKey
	eventAuthority         solana.PublicKey
	creator                solana.PublicKey
	creatorATA             solana.PublicKey
	creatorPurchased       bool
	creatorPurchaseSol     float64 // actual solana amount of buy, not lamports

	// our values related to the coin once we buy / decide to buy, and afterwards
	creatorSold           bool   // has creator sold?
	botPurchased          bool   // separate bool.
	phase                 string // pump ,complete raydium
	exitedBuyCoin         bool   // trigger to notify that we have finished all buy ops
	exitedSellCoin        bool   // trigger to notify that we have exited sell code routine
	exitedCreatorListener bool   // trigger to notify that we stopped listening to creator sell

	isSellingCoin bool // lets program know that we are already in the process of selling coin to avoid dup sell

	associatedTokenAccount  solana.PublicKey // user wallet's ata for this coin
	tokensHeld              *big.Int
	buySolToLamport         float64 // amount of coins we buy for each coin (in lamports)
	slippage                uint64
	buyTransactionSignature *solana.Signature
}

func (c *Coin) status(msg interface{}) {
	logger.Info(c.mintAddr.String(), " ", fmt.Sprintf("%v", msg))
}

// addition setting field
func (c *Coin) SetAssociatedTokenAccount(ata solana.PublicKey) {
	c.associatedTokenAccount = ata
}
func (c *Coin) SetTokensHeld(tokensHeld int64) {
	c.tokensHeld = big.NewInt(tokensHeld)
}

// NewBot creates a new bot struct that we use to buy & sell coins
func NewBot(config *utils.Config, sdkClient *sdk.TradeSubscriptionClient) (*Bot, error) {
	// create trader
	privateKey, err := solana.PrivateKeyFromBase58(config.PrivateKey)
	if err != nil {
		logger.Fatalf("Failed to parse private key: %v", err)
	}
	trader := core.NewTrader(config.RPCUrl, privateKey, config)

	rpcClient := rpc.New(config.RPCUrl)
	nonceClient := client.NewClient(config.RPCUrl)
	// 设置全局配置，供卖出策略使用
	SetConfig(config)

	b := &Bot{
		Config: config,

		Trader:      trader,
		rpcClient:   rpcClient,
		nonceClient: nonceClient,
		sdkClient:   sdkClient,

		// Convert BuyPriorityFee from SOL to lamports (1 SOL = 10^9 lamports)
		feeMicroLamport: uint64(config.BuyPriorityFee * 1e9),
		// Use Jito mode if enabled
		enableJito: config.Jito.Enable,
		// Use sell delay time
		delaySecond: int64(config.SellDelayTime),

		pendingCoins:     make(map[string]*Coin),
		pendingCoinsLock: sync.Mutex{},
		coinsToBuy:       make(chan *Coin),
		coinsToSell:      make(chan string),

		// 初始化卖出策略
		sellStrategy: NewMultiLevelSellStrategy(config.MultiSellLevels),

		// 初始化OpenBook市场状态映射
		marketStates:     make(map[string]*MarketState),
		marketStatesLock: sync.RWMutex{},
	}
	
	// 初始化盈利目标监控器（需要在Bot创建后初始化，因为需要传入bot实例）
	b.profitMonitor = NewProfitTargetMonitor(&config.ProfitTargetMonitor, b)
	
	b.fetchBlockhashLoop()
	return b, nil
}

// GetProfitMonitor 获取盈利目标监控器
func (b *Bot) GetProfitMonitor() *ProfitTargetMonitor {
	return b.profitMonitor
}

// StartProfitTargetMonitoring 启动盈利目标监控
func (b *Bot) StartProfitTargetMonitoring(mintAddress string, platform string, buyPrice float64, buyAmount uint64, creator solana.PublicKey, owner solana.PublicKey) {
	// 检查是否启用盈利目标监控
	if !b.Config.ProfitTargetMonitor.Enable {
		return
	}
	
	// 创建监控任务
	task := &ProfitTargetTask{
		MintAddress:        mintAddress,
		WalletAddress:      owner.String(),
		Platform:           platform,
		BuyPrice:           buyPrice,
		BuyAmount:          buyAmount,
		BuyTime:            time.Now(),
		TargetProfit:       b.Config.ProfitTargetMonitor.TargetProfitPercent,
		Timeout:            time.Duration(b.Config.ProfitTargetMonitor.Timeout) * time.Second,
		PartialSellPercent: b.Config.ProfitTargetMonitor.PartialSellPercent,
		FullExitOnTimeout:  b.Config.ProfitTargetMonitor.FullExitOnTimeout,
		CancelChan:         make(chan struct{}),
		Owner:              owner,
		Creator:            creator,
	}
	
	// 启动监控
	b.profitMonitor.StartMonitoring(task)
}

// NewCoin creates a new Coin instance with the provided parameters
func NewCoin(privateKey solana.PrivateKey, publicKey solana.PublicKey, mintAddr solana.PublicKey, buySolToLamport float64, slippage uint64) *Coin {
	return &Coin{

		privateKey:      privateKey,
		publicKey:       publicKey,
		mintAddr:        mintAddr,
		slippage:        slippage,
		tokensHeld:      big.NewInt(0),
		buySolToLamport: buySolToLamport,
	}
}
