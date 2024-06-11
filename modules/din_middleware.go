package modules

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	"github.com/pkg/errors"

	din_http "github.com/openrelayxyz/din-caddy-plugins/lib/http"
)

var (
	// Initializations of extended Caddy Module Interface Guards
	// https://caddyserver.com/docs/extending-caddy

	// Din Middleware Module
	_ caddy.Module                = (*DinMiddleware)(nil)
	_ caddy.Provisioner           = (*DinMiddleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*DinMiddleware)(nil)
	_ caddyfile.Unmarshaler       = (*DinMiddleware)(nil)
	// TODO: validate provision step
	// _ caddy.Validator			= (*mod.DinMiddleware)(nil)
)

type DinMiddleware struct {
	Services   map[string]*Service `json:"services"`
	stopChan   chan struct{}
	HttpClient *din_http.HTTPClient
}

type Service struct {
	Providers map[string]*provider `json:"providers"`
	Methods   []*string            `json:"methods"`

	LatestBlockNumberMethod string `json:"latest_block_number_method"`
	LatestBlockNumber       *int64 `json:"latest_block_number"`
	PriorityLocked          bool   `json:"priority_locked"`
}

// CaddyModule returns the Caddy module information.
func (DinMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.din",
		New: func() caddy.Module { return new(DinMiddleware) },
	}
}

// Provision() is called by Caddy to prepare the middleware for use.
// It is called only once, when the server is starting.
func (d *DinMiddleware) Provision(context caddy.Context) error {
	// initialize the http client on the middleware struct
	d.HttpClient = din_http.NewHTTPClient()

	// For each provider object in each service, we parse the URL and populate the upstream and path fields.
	for _, service := range d.Services {
		for _, provider := range service.Providers {
			url, err := url.Parse(provider.HttpUrl)
			if err != nil {
				return err
			}
			// provider.upstream = &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)}
			provider.upstream = &reverseproxy.Upstream{Dial: url.Host}
			provider.path = url.Path
		}
	}

	// We then start the latest block number polling for each provider in each network.
	// This is done in a goroutine that sets the latest block number in the service object,
	// and updates the provider's priorities accordingly.
	go d.UpdateProviderPriorities()

	return nil
}

// updateProviderPriorities is a goroutine that updates the provider priorities every 10 seconds.
func (d *DinMiddleware) UpdateProviderPriorities() {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			fmt.Println("ticker")
			for _, service := range d.Services {
				// only update the latest block number if the service doesn't have a locked priority hierarchy in place
				if service.PriorityLocked || service.LatestBlockNumberMethod == "" {
					continue
				}

				checkedProviders := make(map[string]int64)
				for _, provider := range service.Providers {
					// get the latest block number from the provider
					latestBlockNumber, err := d.GetLatestBlockNumber(provider, service.LatestBlockNumberMethod)
					if err != nil {
						fmt.Println("error getting latest block number", err)
						continue
					}
					// if the latest block number is greater than the current latest block number, update the service's latest block number
					if service.LatestBlockNumber == nil || *service.LatestBlockNumber < *latestBlockNumber {
						service.LatestBlockNumber = latestBlockNumber

						// set the priority of the provider to 0
						// service.Providers[provider.path].Priority = 0
						provider.Priority = 0

						// loop through all of the checked providers and set their priority to 1 unless they are the provider with the latest block number
						for path, blockNumber := range checkedProviders {
							if blockNumber != *service.LatestBlockNumber {
								service.Providers[path].Priority = 1
							}
						}
					}
					// add this provider to the checked providers map
					checkedProviders[provider.path] = *latestBlockNumber
				}
			}
		case <-d.stopChan:
			ticker.Stop()
			return
		}
	}
}

// ServeHTTP is the main handler for the middleware that is ran for every request.
// It checks if the service path is defined in the services map and sets the provider in the context.
func (d *DinMiddleware) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	servicePath := strings.TrimPrefix(r.URL.Path, "/")

	service, ok := d.Services[servicePath]
	if !ok {
		if servicePath == "" {
			rw.WriteHeader(200)
			rw.Write([]byte("{}"))
			return nil
		}
		rw.WriteHeader(404)
		rw.Write([]byte("Not Found\n"))
		return fmt.Errorf("service undefined")
	}
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	repl.Set(DinUpstreamsContextKey, service.Providers)
	return next.ServeHTTP(rw, r)
}

// UnmarshalCaddyfile sets up reverse proxy provider and method data on the serve based on the configuration of the Caddyfile
func (d *DinMiddleware) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	var latestBlockNumberMethod string
	// if d.Methods == nil {
	// 	d.Methods = make(map[string][]*string)
	// }
	if d.Services == nil {
		d.Services = make(map[string]*Service)
	}
	for dispenser.Next() { // Skip the directive name
		switch dispenser.Val() {
		case "services":
			for n1 := dispenser.Nesting(); dispenser.NextBlock(n1); {
				serviceName := dispenser.Val()
				d.Services[serviceName] = &Service{}
				for nesting := dispenser.Nesting(); dispenser.NextBlock(nesting); {
					switch dispenser.Val() {
					case "methods":
						d.Services[serviceName].Methods = make([]*string, dispenser.CountRemainingArgs())
						for i := 0; i < dispenser.CountRemainingArgs(); i++ {
							d.Services[serviceName].Methods[i] = new(string)
						}
						if !dispenser.Args(d.Services[serviceName].Methods...) {
							return dispenser.Errf("invalid 'methods' argument for service %s", serviceName)
						}
					case "providers":
						for dispenser.NextBlock(nesting + 1) {
							providerObj, err := urlToProvider(dispenser.Val())
							if err != nil {
								return err
							}
							for dispenser.NextBlock(nesting + 2) {
								switch dispenser.Val() {
								case "headers":
									for dispenser.NextBlock(nesting + 3) {
										k := dispenser.Val()
										var v string
										if dispenser.Args(&v) {
											providerObj.Headers[k] = v
										} else {
											return dispenser.Errf("header should have key and value")
										}
									}
								case "priority":
									dispenser.NextBlock(nesting + 2)
									providerObj.Priority, err = strconv.Atoi(dispenser.Val())
									if err != nil {
										return err
									}
								}
							}
							if d.Services[serviceName].Providers == nil {
								d.Services[serviceName].Providers = make(map[string]*provider)
							}

							d.Services[serviceName].Providers[providerObj.path] = providerObj
						}
						if len(d.Services[serviceName].Providers) == 0 {
							return dispenser.Errf("expected at least one provider for service %s", serviceName)
						}
					case "latest_block_number_method":
						dispenser.Next()
						latestBlockNumberMethod = dispenser.Val()
						if d.Services[serviceName] == nil {
							return dispenser.Errf("service %s not found", serviceName)
						}
						d.Services[serviceName].LatestBlockNumberMethod = latestBlockNumberMethod
					default:
						return dispenser.Errf("unrecognized option: %s", dispenser.Val())
					}
				}
			}
		}
	}
	return nil
}

// urlToProvider parses the URL and returns an provider object
func urlToProvider(urlStr string) (*provider, error) {
	url, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return &provider{
		HttpUrl: urlStr,
		path:    url.Path,
		Headers: make(map[string]string),
		// upstream: &reverseproxy.Upstream{Dial: fmt.Sprintf("%v://%v", url.Scheme, url.Host)},
		upstream: &reverseproxy.Upstream{Dial: url.Host},
	}, nil
}

func (d *DinMiddleware) GetLatestBlockNumber(provider *provider, latestBlockNumberMethod string) (*int64, error) {
	payload := []byte(`{"jsonrpc":"2.0","method":"` + latestBlockNumberMethod + `","params":[],"id":1}`)

	// Send the POST request
	resp, err := d.HttpClient.Post(provider.HttpUrl, provider.Headers, []byte(payload))
	if err != nil {
		return nil, errors.Wrap(err, "Error sending POST request")
	}

	// response struct
	var result struct {
		Jsonrpc string `json:"jsonrpc"`
		Id      int    `json:"id"`
		Result  string `json:"result"`
	}

	// Unmarshal the response
	err = json.Unmarshal(resp, &result)
	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling response")
	}

	// Convert the hexadecimal string to an int64
	blockNumber, err := strconv.ParseInt(result.Result[2:], 16, 64)
	if err != nil {
		return nil, errors.Wrap(err, "Error converting block number")
	}

	return aws.Int64(blockNumber), nil
}

func (d *DinMiddleware) ParseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	err := d.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}

	return d, nil
}
