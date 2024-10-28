package siwe

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spruceid/siwe-go"
	"go.uber.org/zap"
)

type ISIWESignerClient interface {
	GenPrivKey(sc *SigningConfig) error
	Sign(msg string, sc *SigningConfig) ([]byte, error)
	CreateNewSIWEAuth(authUrl string, sessionCount int) *SIWEClientAuth
}

type SIWESignerClient struct{}

type SigningConfig struct {
	PrivateKey []byte
	privateKey *ecdsa.PrivateKey
	SignerURL  string
	Address    string
}

func NewSIWESignerClient() *SIWESignerClient {
	return &SIWESignerClient{}
}

func (s *SIWESignerClient) Sign(msg string, sc *SigningConfig) ([]byte, error) {
	if sc.privateKey == nil && len(sc.PrivateKey) > 0 {
		s.GenPrivKey(sc)
	}
	if sc.privateKey != nil {
		// Sign Locally
		return signMessage(msg, sc.privateKey)
	}
	// Call signer
	return nil, errors.New("private key must be set in signing config")
}

func (s *SIWESignerClient) GenPrivKey(sc *SigningConfig) error {
	if sc.privateKey == nil {
		privateKey, err := crypto.ToECDSA(sc.PrivateKey)
		sc.privateKey = privateKey
		if err != nil {
			return fmt.Errorf("error converting private key: %w", err)
		}
	}
	if sc.Address == "" {
		sc.Address = crypto.PubkeyToAddress(sc.privateKey.PublicKey).String()
	}
	return nil
}

func (s *SIWESignerClient) CreateNewSIWEAuth(authUrl string, sessionCount int) *SIWEClientAuth {
	return &SIWEClientAuth{
		ProviderURL:  authUrl,
		SessionCount: sessionCount,
	}
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

type SIWEClientAuth struct {
	ProviderURL   string
	SessionTokens []auth.AuthToken
	SessionCount  int
	Signer        *SigningConfig
	err           error
	quitCh        chan struct{}
	client        *http.Client
	domain        string
	logger        *zap.Logger
}

func NewSIWEClient(url string, sessionCount int, signer *SigningConfig) *SIWEClientAuth {
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
	return &SIWEClientAuth{
		ProviderURL:  url,
		Signer:       signer,
		SessionCount: sessionCount,
		client:       client,
	}
}

// Start a series of sessions with the provider. The AuthClient should automatically
// establish new sessions as they near expiration
func (c *SIWEClientAuth) Start(logger *zap.Logger) error {
	c.logger = logger
	if c.client == nil {
		c.client = &http.Client{Transport: &http.Transport{
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
	}
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
		c.logger.Debug("Making session tokens", zap.Int("i", i), zap.Int("of", c.SessionCount))
		var err error
		c.SessionTokens[i], err = c.GetToken(nil)
		if err != nil {
			c.logger.Info("Error establishing session. Will retry in 15 seconds", zap.Int("i", i), zap.String("error", err.Error()))
			now := auth.UnixTime(time.Now())
			c.SessionTokens[i].Expiration = &now
			c.Renew(i, 15*time.Second)
			continue
		}
		if c.SessionTokens[i].Expiration != nil {
			c.Renew(i, time.Until(time.Time(*c.SessionTokens[i].Expiration)))
		}
	}
	return nil
}

func (c *SIWEClientAuth) Renew(i int, d time.Duration) {
	go func() {
		t := time.NewTimer(d - (time.Second * 5))
		select {
		case <-t.C:
			var err error
			c.logger.Debug("Attempting renewal", zap.Int("i", i))
			c.SessionTokens[i], err = c.GetToken(nil)
			if err != nil {
				c.logger.Warn("Error getting token, will try again in 1 minute\n", zap.Int("i", i))
				c.Renew(i, time.Minute) // Attempt renewal in 1 minute
				return
			}
			c.logger.Debug("Renewal successful", zap.Int("i", i))
			if c.SessionTokens[i].Expiration != nil {
				c.Renew(i, time.Until(time.Time(*c.SessionTokens[i].Expiration)))
			}
		case <-c.quitCh:
			c.err = auth.ErrSessionClosed
			return
		}
	}()
}

// Error will return an error if the AuthClient is unhealthy, or nil if it should be able to sign a valid request
func (c *SIWEClientAuth) Error() error {
	if c.err != nil {
		return c.err
	}
	for _, token := range c.SessionTokens {
		if err := token.Peek(); err == nil {
			// If any token is non-nil, this client is okay
			return nil
		}
	}
	return auth.ErrNoTokensAvailable
}

type signedMessage struct {
	Message   string        `json:"msg"`
	Signature hexutil.Bytes `json:"sig"`
}

// GetToken should take a map of parameters for a token, and return a map of Header -> Value for a session
func (c *SIWEClientAuth) GetToken(map[string]interface{}) (auth.AuthToken, error) {
	options := make(map[string]interface{})
	options["expirationTime"] = time.Now().Add(time.Minute)

	siweSignerClient := NewSIWESignerClient()
	err := siweSignerClient.GenPrivKey(c.Signer)
	if err != nil {
		return auth.AuthToken{}, err
	}

	msg, err := siwe.InitMessage(c.domain, c.Signer.Address, c.ProviderURL, siwe.GenerateNonce(), options)
	if err != nil {
		return auth.AuthToken{}, err
	}
	sig, err := siweSignerClient.Sign(msg.String(), c.Signer)
	if err != nil {
		return auth.AuthToken{}, err
	}
	data, err := json.Marshal(signedMessage{
		Message:   msg.String(),
		Signature: sig,
	})
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
	if err := json.Unmarshal(body, &tok); err != nil {
		return tok, err
	}
	if tok.Error != "" {
		err = errors.New(tok.Error)
	}

	return tok, err
}

// Sign should add headers to the client request such that it would be accepted by the server
func (c *SIWEClientAuth) Sign(r *http.Request) error {
	at := c.selectAuthToken(r)
	counter := 0
	for err := at.Use(); err != nil; err = at.Use() {
		if counter == 10 {
			return err
		}
		counter++
		at = c.selectAuthToken(r)
	}
	for k, v := range at.Headers {
		r.Header.Set(k, v)
	}
	return nil
}

func hashStringToIndex(s string, listSize int) int {
	hasher := fnv.New32a()        // Initialize a new 32-bit FNV-1a hash
	hasher.Write([]byte(s))       // Hash the string
	hash := hasher.Sum32()        // Get the hash as a 32-bit unsigned integer
	index := int(hash) % listSize // Use modulo to ensure the index is within the bounds of the list
	return index
}

func (c *SIWEClientAuth) selectAuthToken(r *http.Request) auth.AuthToken {
	if sessionId := r.Header.Get("Din-Session-Id"); sessionId != "" {
		return c.SessionTokens[hashStringToIndex(sessionId, c.SessionCount)]
	}
	return c.SessionTokens[0]
}

// Stop should end any Goroutines associated with this client. Once an AuthClient is stopped it cannot be started again
func (c *SIWEClientAuth) Stop() {
	close(c.quitCh)
}
