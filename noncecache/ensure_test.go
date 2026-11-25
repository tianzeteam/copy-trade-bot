package noncecache

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
)

type nonceTestRPC struct {
	mu          sync.Mutex
	url         string
	wallet      solana.PublicKey
	walletKey   string
	missing     map[string]bool
	sendCount   int
	failAccount bool
}

func (s *nonceTestRPC) accountInfo(pubkey solana.PublicKey) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.missing[pubkey.String()] {
		return map[string]any{"context": map[string]any{"slot": 1}, "value": nil}
	}

	var data []byte
	if pubkey.Equals(s.wallet) {
		data = []byte{}
	} else {
		data = make([]byte, NONCE_ACCOUNT_LENGTH)
		binary.LittleEndian.PutUint32(data[4:8], nonceStateInitialized)
		copy(data[8:40], s.wallet[:])
		copy(data[40:72], make([]byte, 32))
		binary.LittleEndian.PutUint64(data[72:80], 5000)
	}
	return map[string]any{
		"context": map[string]any{"slot": 1},
		"value": map[string]any{
			"lamports":   1_000_000_000,
			"data":       []any{base64.StdEncoding.EncodeToString(data), "base64"},
			"owner":      solana.SystemProgramID.String(),
			"executable": false,
			"rentEpoch":  0,
		},
	}
}

func (s *nonceTestRPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Method == "getAccountInfo" && s.failAccount {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "rpc unavailable"}})
		return
	}

	var result any
	switch req.Method {
	case "getAccountInfo":
		var pubkey string
		_ = json.Unmarshal(req.Params[0], &pubkey)
		info := s.accountInfo(solana.MustPublicKeyFromBase58(pubkey))
		if len(req.Params) > 1 {
			var opts struct {
				DataSlice *struct {
					Offset int `json:"offset"`
					Length int `json:"length"`
				} `json:"dataSlice"`
			}
			_ = json.Unmarshal(req.Params[1], &opts)
			if opts.DataSlice != nil && info["value"] != nil {
				value := info["value"].(map[string]any)
				encoded := value["data"].([]any)[0].(string)
				raw, _ := base64.StdEncoding.DecodeString(encoded)
				start := opts.DataSlice.Offset
				end := start + opts.DataSlice.Length
				if start >= 0 && end <= len(raw) {
					value["data"] = []any{base64.StdEncoding.EncodeToString(raw[start:end]), "base64"}
				}
			}
		}
		result = info
	case "getMinimumBalanceForRentExemption":
		result = 1_461_600
	case "getBalance":
		result = map[string]any{"context": map[string]any{"slot": 1}, "value": 1_000_000_000}
	case "getRecentBlockhash":
		result = map[string]any{
			"context": map[string]any{"slot": 1},
			"value":   map[string]any{"blockhash": base58.Encode(make([]byte, 32)), "lastValidBlockHeight": 1},
		}
	case "sendTransaction":
		s.mu.Lock()
		s.sendCount++
		s.mu.Unlock()
		result = base58.Encode(make([]byte, 64))
	default:
		result = nil
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func newNonceTestManager(t *testing.T, configured solana.PublicKey) (*NonceManager, *nonceTestRPC) {
	t.Helper()
	wallet, err := solana.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	rpcServer := &nonceTestRPC{
		wallet:    wallet.PublicKey(),
		walletKey: wallet.String(),
		missing:   map[string]bool{},
	}
	server := httptest.NewServer(rpcServer)
	rpcServer.url = server.URL
	t.Cleanup(server.Close)

	nm := &NonceManager{
		rpcClient:          rpc.New(server.URL),
		nonceClient:        client.NewClient(server.URL),
		nonceAccountPubkey: &configured,
		stopChan:           make(chan struct{}),
	}
	return nm, rpcServer
}

func TestParseNonceAccountPubkey(t *testing.T) {
	if got := parseNonceAccountPubkey(""); !got.IsZero() {
		t.Fatal("空配置应为零地址")
	}
	if got := parseNonceAccountPubkey("not-base58"); !got.IsZero() {
		t.Fatal("非法配置应为零地址")
	}
	pk := solana.NewWallet().PublicKey()
	if got := parseNonceAccountPubkey(pk.String()); !got.Equals(pk) {
		t.Fatal("合法地址应被保留")
	}
}

func TestEnsureNonceAccountReusesValidConfigured(t *testing.T) {
	configured := solana.NewWallet().PublicKey()
	nm, rpcServer := newNonceTestManager(t, configured)

	created, err := nm.EnsureNonceAccount(context.Background(), rpcServer.walletKey, true)
	if err != nil {
		t.Fatalf("复用失败: %v", err)
	}
	if created {
		t.Fatal("合法 nonce 不应重新创建")
	}
	if nm.GetNonceAccountPubkey() == nil || !nm.GetNonceAccountPubkey().Equals(configured) {
		t.Fatal("复用后地址应保持不变")
	}
	if rpcServer.sendCount != 0 {
		t.Fatalf("复用不应发送交易，实际 %d", rpcServer.sendCount)
	}
}

func TestEnsureNonceAccountRejectsInvalidWhenCreationDisabled(t *testing.T) {
	configured := solana.NewWallet().PublicKey()
	nm, rpcServer := newNonceTestManager(t, configured)
	rpcServer.missing[configured.String()] = true

	created, err := nm.EnsureNonceAccount(context.Background(), rpcServer.walletKey, false)
	if err == nil || created {
		t.Fatalf("非法 nonce 且禁止创建时应失败: created=%v err=%v", created, err)
	}
	if rpcServer.sendCount != 0 {
		t.Fatal("不应发送交易")
	}
}

func TestEnsureNonceAccountCreatesWhenConfiguredInvalid(t *testing.T) {
	configured := solana.NewWallet().PublicKey()
	nm, rpcServer := newNonceTestManager(t, configured)
	rpcServer.missing[configured.String()] = true

	created, err := nm.EnsureNonceAccount(context.Background(), rpcServer.walletKey, true)
	if err != nil {
		t.Fatalf("自动创建失败: %v", err)
	}
	if !created {
		t.Fatal("非法 nonce 应触发创建")
	}
	if rpcServer.sendCount != 2 {
		t.Fatalf("应发送 create + initialize 两笔交易，实际 %d", rpcServer.sendCount)
	}
	if nm.GetNonceAccountPubkey().Equals(configured) {
		t.Fatal("新 nonce 地址不应等于失效配置")
	}
}

func TestEnsureNonceAccountDoesNotCreateOnRPCFailure(t *testing.T) {
	configured := solana.NewWallet().PublicKey()
	nm, rpcServer := newNonceTestManager(t, configured)
	rpcServer.failAccount = true

	created, err := nm.EnsureNonceAccount(context.Background(), rpcServer.walletKey, true)
	if created || !errors.Is(err, ErrNonceValidationUnavailable) {
		t.Fatalf("RPC 失败时不应创建新 nonce: created=%v err=%v", created, err)
	}
	if rpcServer.sendCount != 0 {
		t.Fatal("RPC 失败时不应发送交易")
	}
}
