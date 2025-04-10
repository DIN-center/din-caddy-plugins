package modules

import (
	"encoding/json"
	"net/http"
	din_http "github.com/DIN-center/din-caddy-plugins/lib/http"
	"github.com/caddyserver/caddy/v2"
)

type ProviderFilter interface {
    FilterProviders(*http.Request, map[string]*provider) map[string]*provider
}

type methodFilter struct {
	FilteredMethods map[string]struct{}
}


func (mf *methodFilter) FilterProviders(r *http.Request, p map[string]*provider) map[string]*provider {
	if len(mf.FilteredMethods) == 0 {
		return p
	}
	result := make(map[string]*provider)
	
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)

	var bodyData []byte
	var request din_http.JSONRPCRequest

	if v, ok := repl.Get(RequestBodyKey); ok {
		bodyData = v.([]byte)
	}

	// Unmarshal the byte array into the struct
	err := json.Unmarshal(bodyData, &request)
	if err != nil {
		// Body isn't JSON, send it to any provider and good luck
		return p
	}
	if _, ok := mf.FilteredMethods[request.Method]; !ok {
		// Method isn't an important one, send to any provider
		return p
	}
	for k, provider := range p {
		if _, ok := provider.Methods[request.Method]; ok {
			result[k] = provider
		}
	}
	if len(result) == 0 {
		// Nothing supports this method. Send it to any provider and good luck
		return p
	}
	return result
}