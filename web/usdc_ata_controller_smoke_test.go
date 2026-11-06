package web

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"copytrade/utils"

	"github.com/gagliardetto/solana-go"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"gopkg.in/yaml.v3"
)

func TestUSDCATAControllerCreatesIdempotentATA(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_, privateKeyBytes, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	privateKey := solana.PrivateKey(privateKeyBytes)
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode RPC request: %v", err)
		}

		response := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(request.ID)}
		switch request.Method {
		case "getAccountInfo":
			response["result"] = map[string]any{
				"context": map[string]uint64{"slot": 1},
				"value":   nil,
			}
		case "getLatestBlockhash":
			response["result"] = map[string]any{
				"context": map[string]uint64{"slot": 1},
				"value": map[string]any{
					"blockhash":            solana.Hash{}.String(),
					"lastValidBlockHeight": 100,
				},
			}
		case "sendTransaction":
			response["result"] = solana.Signature{}.String()
		default:
			t.Fatalf("unexpected RPC method: %s", request.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode RPC response: %v", err)
		}
	}))
	defer rpcServer.Close()

	configBytes, err := yaml.Marshal(utils.Config{PrivateKey: privateKey.String(), RPCUrl: rpcServer.URL})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, configBytes, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previousSecret := jwtSecret
	jwtSecret = []byte("usdc-ata-controller-smoke-test-secret")
	defer func() { jwtSecret = previousSecret }()

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Username: "smoke-test",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}).SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	router := gin.New()
	NewUSDCATAController(configPath).RegisterRoutesWithAuth(router)

	request := httptest.NewRequest(http.MethodPost, "/api/usdc-ata", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected HTTP status: got %d, body %s", response.Code, response.Body.String())
	}
	var body struct {
		Status    string `json:"status"`
		Signature string `json:"signature"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode controller response: %v", err)
	}
	if body.Status != "submitted" || body.Signature == "" {
		t.Fatalf("unexpected controller response: %+v", body)
	}
}
