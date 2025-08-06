package openidconnectproxy

import (
    // "context"
    "encoding/json"
    "net/http"
    "net/url"
    "sync"
    "time"

    "github.com/caddyserver/caddy/v2"
    "github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
    "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
    "github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type OpenIDConnectProxy struct {
    ClientID     string `json:"client_id"`
    ClientSecret string `json:"client_secret"`
    TokenURL     string `json:"token_url"`
    Scope        string `json:"scope"`
    Upstream     string `json:"upstream"`

    tokenMu      sync.RWMutex
    accessToken  string
    tokenExpiry  time.Time
}

func (a OpenIDConnectProxy) CaddyModule() caddy.ModuleInfo {
    return caddy.ModuleInfo{
        ID:  "http.handlers.openid_connect_proxy",
        New: func() caddy.Module { return new(OpenIDConnectProxy) },
    }
}

func (a *OpenIDConnectProxy) Provision(ctx caddy.Context) error {
    go a.refreshTokenLoop()
    return nil
}

func (a *OpenIDConnectProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
    token := a.getToken()
    r.Header.Set("Authorization", "Bearer "+token)
    return next.ServeHTTP(w, r)
}

func (a *OpenIDConnectProxy) getToken() string {
    a.tokenMu.RLock()
    defer a.tokenMu.RUnlock()
    return a.accessToken
}

func (a *OpenIDConnectProxy) refreshTokenLoop() {
    for {
        token, expiry, err := a.fetchToken()
        if err != nil {
            time.Sleep(30 * time.Second)
            continue
        }

        a.tokenMu.Lock()
        a.accessToken = token
        a.tokenExpiry = expiry
        a.tokenMu.Unlock()

        time.Sleep(time.Until(expiry.Add(-1 * time.Minute)))
    }
}

func (a *OpenIDConnectProxy) fetchToken() (string, time.Time, error) {
    data := url.Values{}
    data.Set("client_id", a.ClientID)
    data.Set("client_secret", a.ClientSecret)
    data.Set("grant_type", "client_credentials")
    data.Set("scope", a.Scope)

    resp, err := http.PostForm(a.TokenURL, data)
    if err != nil {
        return "", time.Time{}, err
    }
    defer resp.Body.Close()

    var parsed struct {
        AccessToken string `json:"access_token"`
        ExpiresIn   int    `json:"expires_in"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
        return "", time.Time{}, err
    }

    return parsed.AccessToken, time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second), nil
}

func (a *OpenIDConnectProxy) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
    for d.Next() {
        for d.NextBlock(0) {
            switch d.Val() {
            case "client_id":
                if !d.Args(&a.ClientID) {
                    return d.ArgErr()
                }
            case "client_secret":
                if !d.Args(&a.ClientSecret) {
                    return d.ArgErr()
                }
            case "token_url":
                if !d.Args(&a.TokenURL) {
                    return d.ArgErr()
                }
            case "scope":
                if !d.Args(&a.Scope) {
                    return d.ArgErr()
                }
            default:
                return d.Errf("unexpected argument: %s", d.Val())
            }
        }
    }
    return nil
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
    var m OpenIDConnectProxy
    err := m.UnmarshalCaddyfile(h.Dispenser)
    if err != nil {
        return nil, err
    }
    return &m, nil
}

func init() {
    caddy.RegisterModule(OpenIDConnectProxy{})
    httpcaddyfile.RegisterHandlerDirective("openid_connect_proxy", parseCaddyfile)
}

var (
    _ caddyhttp.MiddlewareHandler = (*OpenIDConnectProxy)(nil)
    _ caddyfile.Unmarshaler       = (*OpenIDConnectProxy)(nil)
)
