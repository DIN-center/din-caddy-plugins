package din

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

type DinUpstreams struct{}

// CaddyModule returns the Caddy module information.
func (DinUpstreams) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.upstreams.dinupstreams",
		New: func() caddy.Module { return new(DinUpstreams) },
	}
}

func (d *DinUpstreams) GetUpstreams(r *http.Request) ([]*reverseproxy.Upstream, error) {
	var mus []*metaUpstream
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if v, ok := repl.Get("din.internal.upstreams"); ok {
		mus = v.([]*metaUpstream)
	}
	res := make([]*reverseproxy.Upstream, 0, len(mus))
	for priority := 0; priority < len(mus); priority++ {
		for _, u := range mus {
			if u.Priority == priority && u.upstream.Available() {
				res = append(res, u.upstream)
			}
		}
		if len(res) > 0 {
			break
		}
	}
	if len(res) == 0 {
		// Didn't find any based on priority, available, pass along all upstreams
		for _, u := range mus {
			res = append(res, u.upstream)
		}
	}
	return res, nil
}

type metaUpstream struct {
	HttpUrl  string `json:"http.url"`
	path     string
	Headers  map[string]string
	upstream *reverseproxy.Upstream
	Priority int
}

func (d *DinUpstreams) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
