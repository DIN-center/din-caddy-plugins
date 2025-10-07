package siwe

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	sessions "github.com/DIN-center/din-caddy-plugins/lib/auth/siwe/sessions"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	snft "github.com/DIN-center/din-sc/apps/din-go/lib/superfluidnft"
	"github.com/DIN-center/din-sc/apps/din-go/lib/superfluid"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/spruceid/siwe-go"
	"go.uber.org/zap"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// EIP5361 Auth Middleware Module
	_ caddy.Module                = (*SIWEAuthMiddleware)(nil)
	_ caddy.Provisioner           = (*SIWEAuthMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*SIWEAuthMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*SIWEAuthMiddleware)(nil)
	// TODO: validate provision step
	// _ caddy.Validator			= (*mod.DinMiddleware)(nil)
)

func handleError(err error, rw http.ResponseWriter, code int) {
	rw.WriteHeader(code)
	rw.Write([]byte(fmt.Sprintf(`{"error": "%v"}`, err.Error())))
	rw.Write([]byte("\n"))
}

type ChainInfo struct {
	Endpoint string `json:"endpoint"`
	CFAV1Address common.Address `json:"superfluid_forwarder_address"`
	client *superfluid.SupefluidForwarderClient
}

func (c *ChainInfo) Client(logger *zap.Logger) (*superfluid.SupefluidForwarderClient, error) {
	if c.client == nil {
		ec, err := ethclient.Dial(c.Endpoint)
		if err != nil {
			return nil, err
		}
		c.client, err = superfluid.NewSuperfluidForwarderClient(c.CFAV1Address, ec, logger)
		if err != nil {
			return nil, err
		}
	}
	return c.client, nil
}

type SIWEAuthMiddleware struct {
	Whitelist map[string]struct{}     `json:"whitelist"`
	Secret    string                  `json:"secret"`
	ProviderID *big.Int               `json:"provider_id"`
	NftEndpoint string                `json:"nft_endpoint"`
	NftAddresses map[common.Address]struct{}  `json:"nft_addresses"`
	ChainInfo map[string]*ChainInfo      `json:"chain_info`
	FlowrateTracker *sessions.Tracker
	RPSTracker *sessions.Tracker
	logger    *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (SIWEAuthMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din_auth",
		New: func() caddy.Module { return new(SIWEAuthMiddleware) },
	}
}

func (d *SIWEAuthMiddleware) Provision(context caddy.Context) error {
	d.logger = context.Logger(d)
	return nil
}

func flowRateSessionKey(sender, recipient, token common.Address) string {
	return fmt.Sprintf("%s:%s:%s", sender, recipient, token)
}

func calcAvailableRPS(flowrate, maxFlowRate, rpsLimit *big.Int) *big.Int {
	if maxFlowRate.Sign() == 0 {
		return big.NewInt(0)
	}

	// (flowrate * rpsLimit) / maxFlowRate
	scaled := new(big.Int).Mul(flowrate, rpsLimit)
	scaled.Div(scaled, maxFlowRate)

	// Return min(scaled, rpsLimit)
	if scaled.Cmp(rpsLimit) > 0 {
		return new(big.Int).Set(rpsLimit)
	}
	return scaled
}

func (d *SIWEAuthMiddleware) createSession(rw http.ResponseWriter, r *http.Request) error {
	isNftRequest := false
	issued := time.Now()
	sessionId := uuid.NewString()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleError(err, rw, 500)
		return err
	}
	var sm signedMessage
	if err := json.Unmarshal(body, &sm); err != nil {
		return err
	}
	message, err := siwe.ParseMessage(sm.Message)
	if err != nil {
		handleError(err, rw, 400)
		return err
	}
	publicKey, err := message.VerifyEIP191(sm.Signature.String())
	if err != nil {
		return err
	}
	if statement := message.GetStatement(); statement != nil && *statement == "DIN NFT Authentication"{
		isNftRequest = true
	}
	if isNftRequest {
		resources := message.GetResources()
		if len(resources) < 1 {
			err := errors.New("no NFT specified")
			handleError(err, rw, 400)
			return err
		}
		if resources[0].Scheme != "din" {
			err := errors.New("unknown resource protocol")
			handleError(err, rw, 400)
			return err
		}
		nftId, ok := new(big.Int).SetString(resources[0].User.Username(), 10)
		if !ok {
			err := errors.New("expected numeric NFT ID")
			handleError(err, rw, 400)
			return err
		}
		address := common.HexToAddress(resources[0].Host)
		if _, ok := d.NftAddresses[address]; !ok {
			err := errors.New("nft address not recognized")
			handleError(err, rw, 400)
			return err
		}
		client, err := ethclient.Dial(d.NftEndpoint)

		if err != nil {
			return err
		}
		oc, err := snft.NewSuperfluidNFTClient(common.HexToAddress(resources[0].Host), client, d.logger)
		if err != nil {
			err := errors.New("could not construct contract endpoint")
			handleError(err, rw, 500)
			return err
		}
		metadata, err := oc.GetTokenMetadata(nftId)
		if err != nil {
			if err != nil {
				handleError(err, rw, 500)
				return err
			}
		}
		if metadata.ServiceId.Cmp(d.ProviderID) != 0 {
			err := errors.New("nft not authorized by this provider")
			handleError(err, rw, 401)
			return err
		}
		owner, err := oc.OwnerOf(nftId)
		if err != nil {
			handleError(err, rw, 500)
			return err
		}
		if owner != common.Address(crypto.PubkeyToAddress(*publicKey)) {
			err := errors.New("signer does not own specified")
			handleError(err, rw, 401)
			return err
		}
		if metadata.TokenAddress != (common.Address{}) {
			chainid := fmt.Sprintf("%#x", metadata.ChainID)

			cfav1_client, err := d.ChainInfo[chainid].Client(d.logger)
			if err != nil {
				return err
			}

			totalFlow, err := cfav1_client.GetFlowrate(metadata.TokenAddress, owner, metadata.StreamRecipient)
			if err != nil {
				return err
			}
			
			frKey := flowRateSessionKey(owner, metadata.StreamRecipient, metadata.TokenAddress)
			allocatedFlow, err := d.FlowrateTracker.GetCollectionQuantity(r.Context(), frKey)
			if err != nil {
				return err
			}

			availableFlow := new(big.Int).Sub(totalFlow, allocatedFlow)

			rpsLimit := new(big.Int).SetInt64(int64(metadata.RequestsPerSecondLimit))

			allocatedRPS, err := d.RPSTracker.GetCollectionQuantity(r.Context(), metadata.URL().String())
			if err != nil {
				return err
			}

			availableRPS := new(big.Int).Sub(rpsLimit, allocatedRPS)
			

			var sessionRPS *big.Int
			if rpsStr := resources[0].Query().Get("rps"); rpsStr != "" {
				if rps, ok := new(big.Int).SetString(rpsStr, 10); ok {
					sessionRPS = rps
				}
			}
			if sessionRPS == nil {
				// TODO (longer term): rather than erroring out here, determine the maximum available flow given the RPS limit on the token, the available flow, and the token's stream max
				err := errors.New("session requests must specify a number of rps")
				handleError(err, rw, 401)
				return err
			}


			// requiredFlow := (sesionRPS * metadata.StreamMax) / rpsLimit
			requiredFlow := new(big.Int).Div(new(big.Int).Mul(sessionRPS, metadata.StreamMax, ), rpsLimit)

			if sessionRPS.Cmp(availableRPS) > 0 {
				d.logger.Info("requested more RPS than available", zap.Any("session", sessionRPS), zap.Any("avail", availableRPS))
				err := errors.New("requested more RPS than token has available")
				handleError(err, rw, 401)
				return err
			}
			
			if requiredFlow.Cmp(availableFlow) > 0 {
				d.logger.Info("request needs more flow than available", zap.Any("session", requiredFlow), zap.Any("avail", availableFlow))
				err := errors.New("request needs more flow than token has available")
				handleError(err, rw, 401)
				return err
			}
			// TODO: Consider giving some leeway (eg. 30 seconds or a minute) before a session expires during which
			// replacement sessions can be issued, so both can co-exist for a brief period.
			exp := issued.Add(time.Hour)

			if err := d.FlowrateTracker.AddSession(r.Context(), sessionId, frKey, requiredFlow, exp); err != nil {
				return err
			}
			if err := d.RPSTracker.AddSession(r.Context(), sessionId, metadata.URL().String(), sessionRPS, exp); err != nil {
				return err
			}
		}
	} else {
		if _, ok := d.Whitelist[strings.ToLower(crypto.PubkeyToAddress(*publicKey).String())]; !ok {
			err := errors.New("unauthorized signer")
			handleError(err, rw, 401)
			return err
		}
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(issued),
		ExpiresAt: jwt.NewNumericDate(issued.Add(time.Hour)),
	})
	tokenString, err := token.SignedString([]byte(d.Secret))
	if err != nil {
		d.logger.Warn("Signing error", zap.String("error", err.Error()), zap.String("key", d.Secret))
		handleError(err, rw, 500)
		return err
	}
	d.logger.Debug("token issued")

	exp := auth.UnixTime(issued.Add(time.Hour))
	data, err := json.Marshal(auth.AuthToken{
		Headers: map[string]string{
			"x-api-key": tokenString,
		},
		Expiration: &exp,
	})
	if err != nil {
		handleError(err, rw, 500)
		return err
	}
	rw.WriteHeader(200)
	rw.Write(data)
	rw.Write([]byte("\n"))
	return nil
}

func (d *SIWEAuthMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	switch r.URL.Path {
	case "/auth":
		return d.createSession(rw, r)
	case "/":
		// Used for proxy health checks
		return next.ServeHTTP(rw, r)
	default:
	}
	tokenString := r.Header.Get("x-api-key")
	if tokenString == "" {
		err := errors.New("no auth token")
		handleError(err, rw, 401)
		return err
	}
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(d.Secret), nil
	})
	if err != nil {
		handleError(err, rw, 403)
		return err
	}
	if !token.Valid {
		err := fmt.Errorf("invalid token")
		handleError(err, rw, 403)
		return err
	}
	return next.ServeHTTP(rw, r)

}

// UnmarshalCaddyfile sets up reverse proxy upstreamWrapper and method data on the serve based on the configuration of the Caddyfile
func (d *SIWEAuthMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	d.Whitelist = make(map[string]struct{})
	d.NftAddresses = make(map[common.Address]struct{})
	d.FlowrateTracker = sessions.NewMemoryTracker()
	d.RPSTracker = sessions.NewMemoryTracker()
	for dispenser.Next() {
		for dispenser.NextBlock(0) {
			switch dispenser.Val() {
			case "whitelist":
				for _, v := range dispenser.RemainingArgs() {
					d.Whitelist[strings.ToLower(v)] = struct{}{}
				}
			case "secret":
				dispenser.NextBlock(0)
				d.Secret = dispenser.Val()
			case "secret_file":
				var secretFilePath string
				if !dispenser.Args(&secretFilePath) {
					return dispenser.ArgErr()
				}
				secret, err := ioutil.ReadFile(secretFilePath)
				if err != nil {
					return dispenser.Errf("failed to read secret file: %v", err)
				}
				d.Secret = string(secret)
			case "provider_id":
				dispenser.NextBlock(0)
				d.ProviderID, _ = new(big.Int).SetString(dispenser.Val(), 10)
			case "nft_endpoint":
				dispenser.NextBlock(0)
				d.NftEndpoint = dispenser.Val()
			case "nft_addresses":
				for _, v := range dispenser.RemainingArgs() {
					d.NftAddresses[common.HexToAddress(v)] = struct{}{}
				}
			case "chain_info":
				for dispenser.NextBlock(1) {
					var ci ChainInfo
					var chainid, address string
					if !dispenser.AllArgs(&chainid, &ci.Endpoint, &address) {
						return dispenser.ArgErr()
					}
					ci.CFAV1Address = common.HexToAddress(address)
					d.ChainInfo[chainid] = &ci
				}

			default:
				return dispenser.Errf("unknown subdirective: %s", dispenser.Val())
			}
		}
	}
	// If the secret is still empty, generate a random one
	if d.Secret == "" {
		secret, err := generateRandomSecret()
		if err != nil {
			return dispenser.Errf("failed to generate random secret: %v", err)
		}
		d.Secret = secret
	}

	return nil
}

func (d *SIWEAuthMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}

// generateRandomSecret generates a random 32-byte secret and returns it as a base64-encoded string.
func generateRandomSecret() (string, error) {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(secret), nil
}
