package siwe

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"
	// "github.com/openrelayxyz/din-caddy-plugins/auth"
)

// Headers    map[string]string `json:"headers`
// Expiration *UnixTime `json:"exp,omitempty"`
// Uses       *int64    `json:"uses,omitempty"`

func TestClientBasic(t *testing.T) {
	counter := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth" {
			t.Errorf("Expected to request '/auth', got: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"headers": {"x-api-key": "%v"}}`, counter)))
		counter++
	}))
	defer server.Close()
	key, _ := crypto.GenerateKey()
	keyBytes := crypto.FromECDSA(key)
	signer := &SigningConfig{
		PrivateKey: keyBytes,
	}
	signer.GenPrivKey()
	fmt.Printf("Key: %#x\nAddress: %v", keyBytes, signer.Address)
	client := NewSIWEClient(server.URL+"/auth", 16, signer)
	if err := client.Start(zap.NewNop()); err != nil {
		t.Errorf(err.Error())
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add("Din-Session-Id", "foo")
	at := client.selectAuthToken(req)
	if at.Headers == nil {
		t.Errorf("Auth token should have non-nil headers")
	}
	if err := client.Sign(req); err != nil {
		t.Errorf("error signing request: %v", err.Error())
	}
	if req.Header.Get("x-api-key") == "" {
		t.Errorf("Expected x-api-key header to be set")
	}
}
