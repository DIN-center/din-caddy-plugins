package modules

import (
	reflect "reflect"
	"testing"

	"github.com/caddyserver/caddy/v2"
)

func TestSelectCaddyModule(t *testing.T) {
	dinSelect := new(DinSelect)

	tests := []struct {
		name   string
		output caddy.ModuleInfo
	}{
		{
			name: "TestSelectCaddyModuleInit",
			output: caddy.ModuleInfo{
				ID:  "http.reverse_proxy.selection_policies.dinupstreams",
				New: func() caddy.Module { return new(DinSelect) },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modInfo := dinSelect.CaddyModule()
			if modInfo.ID != tt.output.ID {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.ID, tt.output.ID)
			}
			if reflect.TypeOf(modInfo.New()) != reflect.TypeOf(tt.output.New()) {
				t.Errorf("CaddyModule() = %v, want %v", modInfo.New(), tt.output.New())
			}
		})
	}
}
