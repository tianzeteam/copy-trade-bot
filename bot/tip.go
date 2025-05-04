package bot

import (
	"log"
	"sync"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	jitorpc "github.com/jito-labs/jito-go-rpc"
)

// JitoManager acts as the struct where we store important information for interacting
// with Jito. This includes keeping track of the leaders, determining tip amount based on percentile,
// and building the tip instruction.
type JitoManager struct {
	jitoClient *jitorpc.JitoJsonRpcClient
	rpcClient  *rpc.Client

	privateKey solana.PrivateKey

	// jitoValidators is a map of validator IDs that are running Jito.
	jitoValidators map[string]bool

	// slotLeader maps slot to validator ID.
	slotLeader map[uint64]string

	// voteAccounts maps nodeAccount to voteAccount
	voteAccounts map[string]string

	lock *sync.Mutex
}

func newJitoManager(rpcClient *rpc.Client) (*JitoManager, error) {
	jitoClient := jitorpc.NewJitoJsonRpcClient("https://amsterdam.mainnet.block-engine.jito.wtf/api/v1", "")

	return &JitoManager{
		rpcClient:  rpcClient,
		jitoClient: jitoClient,

		jitoValidators: make(map[string]bool),
		slotLeader:     make(map[uint64]string),
		voteAccounts:   make(map[string]string),

		lock: &sync.Mutex{},
	}, nil
}

func (j *JitoManager) generateTipInstruction(coin *Coin) (*system.Instruction, error) {

	// Get a random tip account
	randomTipAccount, err := j.jitoClient.GetRandomTipAccount()
	if err != nil {
		log.Fatalf("Failed to get random tip account: %v", err)
	}

	jitoTipAccount, err := solana.PublicKeyFromBase58(randomTipAccount.Address)
	if err != nil {
		log.Fatalf("Failed to parse Jito tip account public key: %v", err)
	}

	jitoTipAmount := uint64(10000) // lamports

	return system.NewTransferInstruction(
		jitoTipAmount,
		coin.publicKey,
		jitoTipAccount,
	).Build(), nil

}
