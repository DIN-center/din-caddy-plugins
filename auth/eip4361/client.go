package eip4361

import (
	"bytes"
	"errors"
	"encoding/json"
	"io/ioutil"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"github.com/openrelayxyz/din-caddy-plugins/auth"
	"crypto/ecdsa"
	"time"
	"github.com/spruceid/siwe-go"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"hash/fnv"
)

type SigningConfig struct {
	PrivateKey   []byte
	privateKey   *ecdsa.PrivateKey
	SignerURL    string
	Address      string
}

func (sc *SigningConfig) Sign(msg string) ([]byte, error) {
	if sc.privateKey == nil && len(sc.PrivateKey) > 0 {
		sc.genPrivKey()
	}
	if sc.privateKey != nil {
		// Sign Locally
		return signMessage(msg, sc.privateKey)
	}
	// Call signer
	return nil, errors.New("private key must be set in signing config")
}

func (sc *SigningConfig) genPrivKey() error {
	privateKey, err := crypto.ToECDSA(sc.PrivateKey)
	sc.privateKey = privateKey
	sc.Address = crypto.PubkeyToAddress(sc.privateKey.PublicKey).String()
	return err
}

func signHash(data []byte) common.Hash {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256Hash([]byte(msg))
}

func signMessage(message string, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	sign := signHash([]byte(message))
	signature, err := crypto.Sign(sign.Bytes(), privateKey)

	if err != nil {
		return nil, err
	}

	signature[64] += 27
	return signature, nil
}

type EIP4361ClientAuth struct {
	ProviderURL  string
	SessionTokens []auth.AuthToken
	SessionCount int
	Signer       *SigningConfig
	err  error
	quitCh chan struct{}
	client *http.Client
	domain string
}

func NewEIP4361Client(url string, sessionCount int, signer *SigningConfig) *EIP4361ClientAuth {
	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConnsPerHost:   16,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}}
	return &EIP4361ClientAuth{
		ProviderURL: url,
		Signer: signer,
		SessionCount: sessionCount,
		client: client,
	}
}


// Start a series of sessions with the provider. The AuthClient should automatically 
// establish new sessions as they near expiration
func (c *EIP4361ClientAuth) Start() error {
	url, err := url.Parse(c.ProviderURL)
	if err != nil {
		return err
	}
	c.domain = url.Hostname()
	if c.SessionCount == 0 {
		return errors.New("session count must be > 0")
	}
	c.SessionTokens = make([]auth.AuthToken, c.SessionCount)
	for i := 0; i < c.SessionCount; i++ {
		var err error
		c.SessionTokens[i], err = c.GetToken(nil)
		if err != nil {
			return err
		}
		if c.SessionTokens[i].Expiration != nil {
			c.Renew(i, time.Until(time.Time(*c.SessionTokens[i].Expiration)))
		}
	}
	return nil
}

func (c *EIP4361ClientAuth) Renew(i int, d time.Duration) func() {
	return func() {
		t := time.NewTimer(d - (time.Second * 5))
		select {
		case <-t.C:
			var err error
			c.SessionTokens[i], err = c.GetToken(nil)
			if err != nil {
				c.err = err
				return
			}
			if c.SessionTokens[i].Expiration != nil {
				c.Renew(i, time.Until(time.Time(*c.SessionTokens[i].Expiration)))
			}
		case <-c.quitCh:
			c.err = auth.ErrSessionClosed
			return
		}
	}
}

// Error will return an error if the AuthClient is unhealthy, or nil if it should be able to sign a valid request
func (c *EIP4361ClientAuth) Error() error {
	return c.err
}

type signedMessage struct {
	Message string `json:"msg"`
	Signature hexutil.Bytes `json:"sig"`
}

// GetToken should take a map of parameters for a token, and return a map of Header -> Value for a session
func (c *EIP4361ClientAuth) GetToken(map[string]interface{}) (auth.AuthToken, error) {
	options := make(map[string]interface{})
	options["expirationTime"] = time.Now().Add(time.Minute)
	msg, err := siwe.InitMessage(c.domain, c.Signer.Address, c.ProviderURL, siwe.GenerateNonce(), options)
	if err != nil {
		return auth.AuthToken{}, err
	}
	sig, err := c.Signer.Sign(msg.String())
	if err != nil {
		return auth.AuthToken{}, err
	}
	data, err := json.Marshal(signedMessage{
		Message: msg.String(),
		Signature: sig,
	}); 
	if err != nil {
		return auth.AuthToken{}, err
	}

	r, err := c.client.Post(c.ProviderURL, "text/plain", bytes.NewReader(data))
	if err != nil {
		return auth.AuthToken{}, err
	}

	var tok auth.AuthToken
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return tok, err
	}
	err = json.Unmarshal(body, &tok)

	return tok, err
}
// Sign should add headers to the client request such that it would be accepted by the server
func (c *EIP4361ClientAuth) Sign(r *http.Request) error {
	at := c.selectAuthToken(r)
	counter := 0
	for err := at.Use(); err != nil; err = at.Use() {
		if counter == 10 {
			return err
		}
		at = c.selectAuthToken(r)
	}
	for k, v := range at.Headers {
		r.Header.Set(k, v)
	}
	return nil
}

func hashStringToIndex(s string, listSize int) int {
    hasher := fnv.New32a() // Initialize a new 32-bit FNV-1a hash
    hasher.Write([]byte(s)) // Hash the string
    hash := hasher.Sum32() // Get the hash as a 32-bit unsigned integer
    index := int(hash) % listSize // Use modulo to ensure the index is within the bounds of the list
    return index
}

func (c *EIP4361ClientAuth) selectAuthToken(r *http.Request) auth.AuthToken {
	if sessionId := r.Header.Get("Din-Session-Id"); sessionId != "" {
		return c.SessionTokens[hashStringToIndex(sessionId, c.SessionCount)]
	}
	return c.SessionTokens[0]
}

// Stop should end any Goroutines associated with this client. Once an AuthClient is stopped it cannot be started again
func (c *EIP4361ClientAuth) Stop() {
	close(c.quitCh)
}

