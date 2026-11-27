package noncecache

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/blocto/solana-go-sdk/client"
	"github.com/gagliardetto/solana-go"
)

type tradeNonceResult struct {
	nonce   solana.Hash
	release func()
	err     error
}

func tradeNonceHash(value byte) solana.Hash {
	var hash solana.Hash
	hash[0] = value
	return hash
}

func startTradeNonceAcquisition(nm *NonceManager, ctx context.Context) <-chan tradeNonceResult {
	return startTradeNonceAcquisitionWithPriority(nm, ctx, false)
}

func startPriorityTradeNonceAcquisition(nm *NonceManager, ctx context.Context) <-chan tradeNonceResult {
	return startTradeNonceAcquisitionWithPriority(nm, ctx, true)
}

func startTradeNonceAcquisitionWithPriority(nm *NonceManager, ctx context.Context, priority bool) <-chan tradeNonceResult {
	resultCh := make(chan tradeNonceResult, 1)
	go func() {
		var nonce solana.Hash
		var release func()
		var err error
		if priority {
			nonce, release, err = nm.AcquirePriorityTradeNonce(ctx)
		} else {
			nonce, release, err = nm.AcquireTradeNonce(ctx)
		}
		resultCh <- tradeNonceResult{nonce: nonce, release: release, err: err}
	}()
	return resultCh
}

func requireNoTradeNonceResult(t *testing.T, resultCh <-chan tradeNonceResult) {
	t.Helper()
	select {
	case result := <-resultCh:
		if result.release != nil {
			result.release()
		}
		t.Fatalf("nonce 应被串行阻塞，却提前返回: nonce=%s err=%v", result.nonce, result.err)
	case <-time.After(25 * time.Millisecond):
	}
}

func requireTradeNonceResult(t *testing.T, resultCh <-chan tradeNonceResult, expected solana.Hash) tradeNonceResult {
	t.Helper()
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("获取 nonce 失败: %v", result.err)
		}
		if result.nonce != expected {
			t.Fatalf("nonce 不匹配: got=%s want=%s", result.nonce, expected)
		}
		if result.release == nil {
			t.Fatal("release 不能为空")
		}
		return result
	case <-time.After(time.Second):
		t.Fatal("等待 nonce 更新超时")
		return tradeNonceResult{}
	}
}

func waitForTradeNonceUpdateChannel(t *testing.T, nm *NonceManager) <-chan struct{} {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		nm.mutex.RLock()
		updateCh := nm.nonceUpdateCh
		nm.mutex.RUnlock()
		if updateCh != nil {
			return updateCh
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("等待 nonce 更新通道初始化超时")
	return nil
}

func waitForTradeNonceWaiterCount(t *testing.T, nm *NonceManager, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	var waiting int
	for time.Now().Before(deadline) {
		nm.mutex.RLock()
		waiting = len(nm.tradeNonceWaiters)
		nm.mutex.RUnlock()
		if waiting == expected {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("等待 nonce 门禁队列长度超时: got=%d want=%d", waiting, expected)
}

func TestAcquireTradeNonceFirstUsesCurrentHash(t *testing.T) {
	expected := tradeNonceHash(1)
	nm := &NonceManager{globalBlockhash: &expected}

	nonce, release, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	if nonce != expected {
		t.Fatalf("首次 nonce 不匹配: got=%s want=%s", nonce, expected)
	}
	if release == nil {
		t.Fatal("release 不能为空")
	}

	release()
	release()

	nm.mutex.RLock()
	defer nm.mutex.RUnlock()
	if nm.tradeGate == nil {
		t.Fatal("nil tradeGate 应被懒初始化")
	}
	if !nm.hasTradeNonce || nm.lastTradeNonce != expected {
		t.Fatal("release 不应清除已使用 nonce 状态")
	}
}

func TestAcquireTradeNonceSerializesConcurrentTrades(t *testing.T) {
	firstHash := tradeNonceHash(1)
	secondHash := tradeNonceHash(2)
	nm := &NonceManager{globalBlockhash: &firstHash}

	firstNonce, firstRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	if firstNonce != firstHash {
		t.Fatalf("首次 nonce 不匹配: got=%s want=%s", firstNonce, firstHash)
	}
	defer firstRelease()

	nm.UpdateNonce(secondHash)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := startTradeNonceAcquisition(nm, ctx)
	requireNoTradeNonceResult(t, resultCh)

	firstRelease()
	second := requireTradeNonceResult(t, resultCh, secondHash)
	second.release()
}

func TestAcquirePriorityTradeNoncePreemptsNormalWaiters(t *testing.T) {
	firstHash := tradeNonceHash(1)
	secondHash := tradeNonceHash(2)
	thirdHash := tradeNonceHash(3)
	nm := &NonceManager{globalBlockhash: &firstHash}

	_, firstRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	defer firstRelease()
	nm.UpdateNonce(secondHash)

	normalCtx, normalCancel := context.WithCancel(context.Background())
	defer normalCancel()
	normalResultCh := startTradeNonceAcquisition(nm, normalCtx)
	waitForTradeNonceWaiterCount(t, nm, 1)

	priorityCtx, priorityCancel := context.WithCancel(context.Background())
	defer priorityCancel()
	priorityResultCh := startPriorityTradeNonceAcquisition(nm, priorityCtx)
	waitForTradeNonceWaiterCount(t, nm, 2)

	firstRelease()
	priorityResult := requireTradeNonceResult(t, priorityResultCh, secondHash)
	defer priorityResult.release()
	requireNoTradeNonceResult(t, normalResultCh)

	nm.UpdateNonce(thirdHash)
	priorityResult.release()
	normalResult := requireTradeNonceResult(t, normalResultCh, thirdHash)
	normalResult.release()
}

func TestAcquireTradeNonceFIFOWithinPriority(t *testing.T) {
	tests := []struct {
		name     string
		priority bool
	}{
		{name: "normal"},
		{name: "priority", priority: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			firstHash := tradeNonceHash(1)
			secondHash := tradeNonceHash(2)
			thirdHash := tradeNonceHash(3)
			nm := &NonceManager{globalBlockhash: &firstHash}

			_, holderRelease, err := nm.AcquireTradeNonce(context.Background())
			if err != nil {
				t.Fatalf("首次获取 nonce 失败: %v", err)
			}
			defer holderRelease()
			nm.UpdateNonce(secondHash)

			start := startTradeNonceAcquisition
			if test.priority {
				start = startPriorityTradeNonceAcquisition
			}

			firstCtx, firstCancel := context.WithCancel(context.Background())
			defer firstCancel()
			firstResultCh := start(nm, firstCtx)
			waitForTradeNonceWaiterCount(t, nm, 1)

			secondCtx, secondCancel := context.WithCancel(context.Background())
			defer secondCancel()
			secondResultCh := start(nm, secondCtx)
			waitForTradeNonceWaiterCount(t, nm, 2)

			holderRelease()
			firstResult := requireTradeNonceResult(t, firstResultCh, secondHash)
			defer firstResult.release()
			requireNoTradeNonceResult(t, secondResultCh)

			nm.UpdateNonce(thirdHash)
			firstResult.release()
			secondResult := requireTradeNonceResult(t, secondResultCh, thirdHash)
			secondResult.release()
		})
	}
}

func TestAcquirePriorityTradeNonceCanceledDoesNotBlockNormalWaiter(t *testing.T) {
	firstHash := tradeNonceHash(1)
	secondHash := tradeNonceHash(2)
	nm := &NonceManager{globalBlockhash: &firstHash}

	_, holderRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	defer holderRelease()
	nm.UpdateNonce(secondHash)

	normalCtx, normalCancel := context.WithCancel(context.Background())
	defer normalCancel()
	normalResultCh := startTradeNonceAcquisition(nm, normalCtx)
	waitForTradeNonceWaiterCount(t, nm, 1)

	priorityCtx, priorityCancel := context.WithCancel(context.Background())
	defer priorityCancel()
	priorityResultCh := startPriorityTradeNonceAcquisition(nm, priorityCtx)
	waitForTradeNonceWaiterCount(t, nm, 2)
	priorityCancel()

	select {
	case result := <-priorityResultCh:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("应返回 context.Canceled，实际: %v", result.err)
		}
		if result.release != nil {
			t.Fatal("获取失败时不应返回 release")
		}
	case <-time.After(time.Second):
		t.Fatal("高优先级等待者取消未返回")
	}
	waitForTradeNonceWaiterCount(t, nm, 1)

	holderRelease()
	normalResult := requireTradeNonceResult(t, normalResultCh, secondHash)
	normalResult.release()
}

func TestAcquireTradeNonceWaitsForUpdateNonce(t *testing.T) {
	firstHash := tradeNonceHash(1)
	secondHash := tradeNonceHash(2)
	nm := &NonceManager{globalBlockhash: &firstHash}

	_, firstRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	firstRelease()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := startTradeNonceAcquisition(nm, ctx)
	requireNoTradeNonceResult(t, resultCh)

	nm.UpdateNonce(secondHash)
	second := requireTradeNonceResult(t, resultCh, secondHash)
	second.release()
}

func TestAcquireTradeNonceIgnoresDuplicateHash(t *testing.T) {
	firstHash := tradeNonceHash(1)
	secondHash := tradeNonceHash(2)
	nm := &NonceManager{globalBlockhash: &firstHash}

	_, firstRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	firstRelease()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := startTradeNonceAcquisition(nm, ctx)
	updateCh := waitForTradeNonceUpdateChannel(t, nm)

	nm.mutex.RLock()
	versionBefore := nm.nonceVersion
	nm.mutex.RUnlock()
	nm.UpdateNonce(firstHash)
	nm.mutex.RLock()
	versionAfter := nm.nonceVersion
	nm.mutex.RUnlock()
	if versionAfter != versionBefore {
		t.Fatalf("重复 hash 不应递增版本: before=%d after=%d", versionBefore, versionAfter)
	}
	select {
	case <-updateCh:
		t.Fatal("重复 hash 不应通知等待者")
	default:
	}
	requireNoTradeNonceResult(t, resultCh)

	nm.UpdateNonce(secondHash)
	second := requireTradeNonceResult(t, resultCh, secondHash)
	second.release()
}

func TestAcquireTradeNonceReleaseAllowsNextChangedNonce(t *testing.T) {
	firstHash := tradeNonceHash(1)
	secondHash := tradeNonceHash(2)
	nm := &NonceManager{globalBlockhash: &firstHash}

	_, firstRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	firstRelease()
	nm.UpdateNonce(secondHash)

	nonce, secondRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("release 后获取 nonce 失败: %v", err)
	}
	if nonce != secondHash {
		t.Fatalf("release 后 nonce 不匹配: got=%s want=%s", nonce, secondHash)
	}
	secondRelease()
}

func TestAcquireTradeNonceCanceledWhileWaitingForGate(t *testing.T) {
	hash := tradeNonceHash(1)
	nm := &NonceManager{globalBlockhash: &hash}
	_, release, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, canceledRelease, err := nm.AcquireTradeNonce(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回 context.Canceled，实际: %v", err)
	}
	if canceledRelease != nil {
		t.Fatal("获取失败时不应返回 release")
	}
}

func TestAcquireTradeNonceCanceledWhileWaitingForUpdate(t *testing.T) {
	hash := tradeNonceHash(1)
	nm := &NonceManager{globalBlockhash: &hash}
	_, firstRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	firstRelease()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultCh := startTradeNonceAcquisition(nm, ctx)
	waitForTradeNonceUpdateChannel(t, nm)
	cancel()

	select {
	case result := <-resultCh:
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("应返回 context.Canceled，实际: %v", result.err)
		}
		if result.release != nil {
			t.Fatal("获取失败时不应返回 release")
		}
	case <-time.After(time.Second):
		t.Fatal("context 取消未唤醒 nonce 等待者")
	}
}

type tradeNonceAccountRPC struct {
	mu    sync.Mutex
	nonce solana.Hash
	calls int
}

func (s *tradeNonceAccountRPC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Method != "getAccountInfo" {
		http.Error(w, "unexpected method", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	nonce := s.nonce
	s.calls++
	s.mu.Unlock()

	data := append([]byte(nil), nonce[:]...)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      request.ID,
		"result": map[string]any{
			"context": map[string]any{"slot": 1},
			"value": map[string]any{
				"lamports":   1_000_000_000,
				"data":       []any{base64.StdEncoding.EncodeToString(data), "base64"},
				"owner":      solana.SystemProgramID.String(),
				"executable": false,
				"rentEpoch":  0,
			},
		},
	})
}

func TestAcquireTradeNonceRefreshesAfterTimeout(t *testing.T) {
	firstHash := tradeNonceHash(1)
	refreshedHash := tradeNonceHash(2)
	rpcServer := &tradeNonceAccountRPC{nonce: refreshedHash}
	server := httptest.NewServer(rpcServer)
	t.Cleanup(server.Close)

	nm := &NonceManager{
		globalBlockhash:     &firstHash,
		nonceAccountAddress: solana.NewWallet().PublicKey().String(),
		nonceClient:         client.NewClient(server.URL),
	}
	_, firstRelease, err := nm.AcquireTradeNonce(context.Background())
	if err != nil {
		t.Fatalf("首次获取 nonce 失败: %v", err)
	}
	firstRelease()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	nonce, release, err := nm.AcquireTradeNonce(ctx)
	if err != nil {
		t.Fatalf("超时刷新 nonce 失败: %v", err)
	}
	if nonce != refreshedHash {
		t.Fatalf("刷新后 nonce 不匹配: got=%s want=%s", nonce, refreshedHash)
	}
	release()

	rpcServer.mu.Lock()
	calls := rpcServer.calls
	rpcServer.mu.Unlock()
	if calls == 0 {
		t.Fatal("超时后应通过 RPC 刷新 nonce")
	}
}
