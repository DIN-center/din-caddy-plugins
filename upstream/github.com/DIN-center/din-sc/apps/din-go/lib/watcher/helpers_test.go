package watcher

import (
	"testing"
)

type testParams struct {
	Name     string   `validate:"required" query:"name"`
	Age      int      `validate:"gte=0" query:"age"`
	Interval string   `validate:"interval_format" query:"interval"`
	Tags     []string `query:"tags"`
}

func TestValidateQueryParams(t *testing.T) {
	tests := []struct {
		name    string
		params  interface{}
		wantErr bool
	}{
		{
			name: "valid params",
			params: testParams{
				Name:     "test",
				Age:      25,
				Interval: "30min",
				Tags:     []string{"tag1", "tag2"},
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			params: testParams{
				Age:      25,
				Interval: "30min",
			},
			wantErr: true,
		},
		{
			name: "invalid interval format",
			params: testParams{
				Name:     "test",
				Age:      25,
				Interval: "30x", // invalid suffix
			},
			wantErr: true,
		},
		{
			name: "negative age",
			params: testParams{
				Name:     "test",
				Age:      -1,
				Interval: "30min",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQueryParams(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateQueryParams() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildQuery(t *testing.T) {
	tests := []struct {
		name   string
		params interface{}
		want   string
	}{
		{
			name: "all fields populated",
			params: testParams{
				Name:     "test",
				Age:      25,
				Interval: "30min",
				Tags:     []string{"tag1", "tag2"},
			},
			want: "age=25&interval=30min&name=test&tags=tag1%2Ctag2",
		},
		{
			name: "empty optional fields",
			params: testParams{
				Name: "test",
			},
			want: "name=test",
		},
		{
			name: "with empty slice",
			params: testParams{
				Name: "test",
				Tags: []string{},
			},
			want: "name=test",
		},
		{
			name: "zero age field",
			params: testParams{
				Name: "test",
				Age:  0,
			},
			want: "name=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildQuery(tt.params)
			if got != tt.want {
				t.Errorf("buildQuery() = %v, want %v", got, tt.want)
			}
		})
	}
}
