package siwe

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"strconv"

	"github.com/DIN-center/din-caddy-plugins/lib/auth"
	"github.com/DIN-center/din-caddy-plugins/lib/usage"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/ethereum/go-ethereum/crypto"
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

type SIWEAuthMiddleware struct {
	Whitelist   map[string]struct{} `json:"whitelist"`
	Secret      string              `json:"secret"`
	SessionUses *int              `json:"sessionUses"`
	UsageCfg    *usage.UsageConfig  `json:"usageCfg"`
	tracker     usage.TrackerManager
	logger      *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (SIWEAuthMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din_auth",
		New: func() caddy.Module { return new(SIWEAuthMiddleware) },
	}
}

func (d *SIWEAuthMiddleware) Provision(context caddy.Context) error {
	var err error
	d.logger = context.Logger(d)
	d.tracker, err = usage.ResolveTrackerManager(d.UsageCfg)
	return err
}

func (d *SIWEAuthMiddleware) createSession(rw http.ResponseWriter, r *http.Request) error {
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
	if _, ok := d.Whitelist[crypto.PubkeyToAddress(*publicKey).String()]; !ok {
		err := errors.New("unauthorized signer")
		handleError(err, rw, 401)
		return err
	}
	issued := time.Now()
	var key string
	if d.SessionUses != nil {
		d.tracker.Create(int64(*d.SessionUses), issued.Add(time.Hour))
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(issued),
		ExpiresAt: jwt.NewNumericDate(issued.Add(time.Hour)),
		ID: key,
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
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Access the JWT ID (jti)
		jti, ok := claims["jti"].(string)
		if ok {
			if tracker, ok := d.tracker.Get(jti); ok {
				switch err := tracker.Use(); err {
				case usage.ErrRequestLimit:
					handleError(err, rw, 429)
					return err
				default:
					d.logger.Warn("Error from usage tracker", zap.Error(err))
				}
			} else {
				handleError(usage.ErrNoToken, rw, 403)
				return err
			}
		}
	}
	return next.ServeHTTP(rw, r)

}

// UnmarshalCaddyfile sets up reverse proxy upstreamWrapper and method data on the serve based on the configuration of the Caddyfile
func (d *SIWEAuthMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	d.Whitelist = make(map[string]struct{})
	for dispenser.Next() {
		for dispenser.NextBlock(0) {
			switch dispenser.Val() {
			case "whitelist":
				for _, v := range dispenser.RemainingArgs() {
					d.Whitelist[v] = struct{}{}
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
			case "usage_tracking": 
				for dispenser.NextBlock(1) {
					switch dispenser.Val() {
					case "session_limit":
						var limitStr string
						if !dispenser.Args(&limitStr) {
							return dispenser.ArgErr()
						}
						limit, err := strconv.Atoi(limitStr)
						if err != nil {
							return err
						}
						d.SessionUses = &limit
					case "tracker":
						switch dispenser.Val() {
						case "redis":
							d.UsageCfg = &usage.UsageConfig{
								Type: "redis",
								Config: make(map[string]interface{}),
							}
							d.UsageCfg.Config["url"] = "redis://localhost:6379/0"
							d.UsageCfg.Config["batchSize"] = int64(1000)
							for dispenser.NextBlock(2) {
								switch dispenser.Val() {
								case "redis_url":
									var url string
									if !dispenser.Args(&url) {
										return dispenser.ArgErr()
									}
									d.UsageCfg.Config["url"] = url
								case "batch_size":
									var batchSizeStr string
									if !dispenser.Args(&batchSizeStr) {
										return dispenser.ArgErr()
									}
									batchSize, err := strconv.Atoi(batchSizeStr)
									if err != nil {
										return err
									}
									d.UsageCfg.Config["batchSize"] = int64(batchSize)
								}
							}

						case "mem":
							d.UsageCfg = &usage.UsageConfig{
								Type: "mem",
							}
						}
					}
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
