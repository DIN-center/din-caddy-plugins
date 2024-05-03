package modules

import (
	reflect "reflect"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
)

func TestMiddlewareCaddyModule(t *testing.T) {
	dinMiddleware := new(DinMiddleware)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestMiddlewareCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.handlers.din",
				New: func() caddy.Module { return new(DinMiddleware) },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := dinMiddleware.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}

func TestURLToMetaUpstream(t *testing.T) {
	tests := []struct {
		name   string
		urlstr string
		outPut *metaUpstream
		hasErr bool
	}{
		{
			name:   "passing localhost",
			urlstr: "http://localhost:8080",
			outPut: &metaUpstream{
				HttpUrl:  "http://localhost:8080",
				path:     "",
				Headers:  make(map[string]string),
				upstream: &reverseproxy.Upstream{Dial: "localhost:8080"},
				Priority: 0,
			},
			hasErr: false,
		},
		{
			name:   "passing fullurl with key",
			urlstr: "https://eth.rpc.test.cloud:443/key",
			outPut: &metaUpstream{
				HttpUrl:  "https://eth.rpc.test.cloud:443/key",
				path:     "/key",
				Headers:  make(map[string]string),
				upstream: &reverseproxy.Upstream{Dial: "eth.rpc.test.cloud:443"},
				Priority: 0,
			},
			hasErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaUpstream, err := urlToMetaUpstream(tt.urlstr)
			if err != nil && !tt.hasErr {
				t.Errorf("urlToMetaUpstream() = %v, want %v", err, tt.hasErr)
			}
			if !reflect.DeepEqual(metaUpstream, tt.outPut) {
				t.Errorf("urlToMetaUpstream() = %v, want %v", metaUpstream, tt.outPut)
			}
		})
	}
}
