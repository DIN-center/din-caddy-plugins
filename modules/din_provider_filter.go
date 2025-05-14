package modules

import "github.com/caddyserver/caddy/v2"

// ProviderFilter offers an interface for determining which providers can handle
// a particular request. This is similar to what happens in the upstreams module,
// but lets us attach filtering logic to specific networks.
type ProviderFilter interface {
	FilterProviders(string, map[string]*provider) map[string]*provider
}

// methodFilter implements the ProviderFilter. If a network needs to route specific
// methods to a subset of providers, they can indicate the method's importance here,
// and the methodFilter will compare against the providers' method sets to decide where
// to route.
type methodFilter struct {
	FilteredMethods map[string]struct{}
}

func (mf *methodFilter) FilterProviders(repl *caddy.Replacer, p map[string]*provider) map[string]*provider {
	// If there are no filtered methods, or the method is empty, return all providers
	if len(mf.FilteredMethods) == 0 {
		return p
	}

	method, err := getRequestMethod(repl)
	if err != nil {
		return p
	}
	// If the request method is empty, return all providers
	if method == "" {
		return p
	}

	// Create a new map to store the filtered providers
	result := make(map[string]*provider)

	// If the method is not in the filtered methods, return all providers
	if _, ok := mf.FilteredMethods[method]; !ok {
		return p
	}

	// Iterate over the providers and add the ones that support the filtered method to the result
	for k, provider := range p {
		if _, ok := provider.Methods[method]; ok {
			// This provider supports the specified filtered method, and is eligible to serve the request
			result[k] = provider
		}
	}

	// If there are no providers that support the filtered method, return all providers
	if len(result) == 0 {
		return p
	}

	// Return the filtered providers
	return result
}
