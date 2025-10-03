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

func (c *ChainInfo) Client(logger *zap.Logger) (*superfluid.SupefluidForwarderClient, err) {
	if c.client == nil {
		ec, err := ethclient.Dial(d.NftEndpoint)
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
	SessionTracker *sessions.Tracker
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

func (d *SIWEAuthMiddleware) createSession(rw http.ResponseWriter, r *http.Request) error {
	isNftRequest := false
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

			cfav1_client, err := d.ChainInfo[chainid].Client()
			if err != nil {
				return err
			}

			flowrate, err := cfav1_client.GetFlowrate(metadata.TokenAddress, owner, metadata.RecipientAddress)
			if err != nil {
				return err
			}

			rps := int(metadata.RequestsPerSecondLimit)
			if rpsStr := resources[0].Query().Get("rps"); rpsStr != "" {
				if rpsInt, err := strconv.Atoi(rpsStr); err != nil {
					d.logger.Debug("Parse error on rps string", zap.String("rps", rpsStr), zap.String("error", err.Error()))
				} else {
					rps = rpsInt
				}
			}

			d.RPSTracker(r.Context(), metadata.URL().String())

			

			// TODO: We need separate session trackers, one that tracks RPS against NFTs, and one that tracks flow rate against streams

				// TokenId               *big.Int
				// NFTAddress            common.Address
				// ChainID               *big.Int
				// ServiceId             *big.Int
				// RequestsPerSecondLimit uint32
				// Expiration            uint64
				// TokenAddress          common.Address
				// CostPerRequest        *big.Int
				// RecipientAddress      common.Address

		}
		// Extension: If the NFT has a non-zero token set, use the CFAV1 script to check the flowrate between
		// the token's owner and the specified recipient. Get the combined flowrate of existing sessions from
		// the sessiontracker, and if that plus the requirement for this request is less than the flowrate from
		// the chain, the token can be issued and this session should be tracked in the session manager.
		//
		// For interacting with the session manger, the "customer" field should be an aggregate of the token,
		// sender, and recipient addresses.

	} else {
		if _, ok := d.Whitelist[strings.ToLower(crypto.PubkeyToAddress(*publicKey).String())]; !ok {
			err := errors.New("unauthorized signer")
			handleError(err, rw, 401)
			return err
		}
	}
	issued := time.Now()
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
					var chainid string
					if !disp.AllArgs(&chainid, &ci.Endpoint, &ci.CFAV1Address) {
						return disp.ArgErr()
					}
					d.Chains[chainid] = ci
				}

			// TODO: Once we have other session tracker configurations, those will go here.

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
