package din

import (
	"net/http"
	"net/url"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

type DinSelect struct {
	selector reverseproxy.Selector
}

// CaddyModule returns the Caddy module information.
func (DinSelect) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.dinupstreams",
		New: func() caddy.Module { return new(DinSelect) },
	}
}

func (d *DinSelect) Provision(context caddy.Context) error {
	selector := &reverseproxy.HeaderHashSelection{Field: "Din-Session-Id"}
	selector.Provision(context)
	d.selector = selector
	return nil
}
func (d *DinSelect) Select(pool reverseproxy.UpstreamPool, r *http.Request, rw http.ResponseWriter) *reverseproxy.Upstream {
	repl := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	var mus []*metaUpstream
	if v, ok := repl.Get("din.internal.upstreams"); ok {
		mus = v.([]*metaUpstream)
	}

	res := d.selector.Select(pool, r, rw)
	for _, mu := range mus {
		if res == mu.upstream {
			r.URL.RawPath = mu.path
			r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
			for k, v := range mu.Headers {
				r.Header.Add(k, v)
			}
			break
		}
	}
	return res
}

func (d *DinSelect) UnmarshalCaddyfile(dispenser *caddyfile.Dispenser) error {
	return nil
}
