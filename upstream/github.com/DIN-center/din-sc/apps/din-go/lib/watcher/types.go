package watcher

import "fmt"

// CheckQueryParams represents the parameters for a check request
type CheckQueryParams struct {
	CheckID    string   `query:"check_id" validate:"required,oneof=blockNumberConsistency blockNonStateConsistency"`
	Network    string   `query:"network" validate:"required"`
	ProviderID []string `query:"provider_id"`
	Region     []string `query:"region" validate:"dive,oneof=NorthAmerica Latam Europe MiddleEastAfrica AsiaPacific"`
	Interval   string   `query:"interval" validate:"interval_format"`
}

// Defines the structure of the Global Check API response.
type CheckResponse struct {
	WatcherID      string              `json:"watcher_id"`
	Origin         string              `json:"origin"`
	Network        string              `json:"network"`
	CycleStart     string              `json:"cycle_start"`
	CycleEnd       string              `json:"cycle_end"`
	CycleDuration  int                 `json:"cycle_duration"`
	TotalProviders int                 `json:"total_providers"`
	Providers      []CheckProviderData `json:"providers"`
}

type CheckProviderData struct {
	Provider             string  `json:"provider"`
	ProviderLocation     string  `json:"provider_location"`
	EndpointURL          string  `json:"endpoint_url"`
	BlockNumberFrom      int     `json:"block_number_from"`
	BlockNumberTo        int     `json:"block_number_to"`
	LatestCheckTimestamp string  `json:"latest_check_ts"`
	CheckSummary         Summary `json:"check_summary"`
	ResponseStatus       Status  `json:"response_status"`
}

type Summary struct {
	Total          int     `json:"total"`
	Pass           int     `json:"pass"`
	Fail           int     `json:"fail"`
	PassPercentage float64 `json:"pass_percentage"`
	FailPercentage float64 `json:"fail_percentage"`
}

type Status struct {
	Total             int     `json:"total"`
	Success           int     `json:"success"`
	Error             int     `json:"error"`
	Timeout           int     `json:"timeout"`
	SuccessPercentage float64 `json:"success_percentage"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

// LatencyQueryParams represents the parameters for a latency request
type LatencyQueryParams struct {
	Network    string   `query:"network" validate:"required"`
	ProviderID []string `query:"provider_id"` // Comma-separated
	Region     []string `query:"region" validate:"dive,oneof=NorthAmerica Latam Europe MiddleEastAfrica AsiaPacific"`
	Interval   string   `query:"interval" validate:"interval_format"` // e.g., "5min", "1h"
}

// Defines the structure of the Global Latency API response.
type LatencyResponse struct {
	WatcherID      string                `json:"watcher_id"`
	Origin         string                `json:"origin"`
	Network        string                `json:"network"`
	CycleStart     string                `json:"cycle_start"`
	CycleEnd       string                `json:"cycle_end"`
	CycleDuration  int                   `json:"cycle_duration"`
	TotalProviders int                   `json:"total_providers"`
	Providers      []LatencyProviderData `json:"providers"`
}

type LatencyProviderData struct {
	Provider             string       `json:"provider"`
	ProviderLocation     string       `json:"provider_location"`
	EndpointURL          string       `json:"endpoint_url"`
	LastRequestTimestamp string       `json:"last_request_ts"`
	MaxRPS               float64      `json:"max_rps"`
	Latency              LatencyStats `json:"latency"`
	ResponseStatus       Status       `json:"response_status"`
}

// LatencyStats represents latency statistics for a provider.
type LatencyStats struct {
	Min float64 `json:"min"`
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
	P50 float64 `json:"p50"`
	P75 float64 `json:"p75"`
	P90 float64 `json:"p90"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// Result represents either a success value T or an error
// This is a more convenient way to handle errors in requests and avoid boilerplate code
type Result[T any] struct {
	Ok  *T
	Err *error
}

// Ok creates a new successful Result
func Ok[T any](value T) Result[T] {
	return Result[T]{Ok: &value}
}

// Err creates a new error Result
func Err[T any](err error) Result[T] {
	return Result[T]{Err: &err}
}

// IsOk returns true if the Result contains a success value
func (r Result[T]) IsOk() bool {
	return r.Err == nil
}

// IsErr returns true if the Result contains an error
func (r Result[T]) IsErr() bool {
	return r.Err != nil
}

// Unwrap returns the contained Ok value or panics if it's an Err
func (r Result[T]) Unwrap() T {
	if r.Ok == nil {
		panic(fmt.Sprintf("%v", *r.Err))
	}
	return *r.Ok
}

// UnwrapErr returns the contained Err value or panics if it's an Ok
func (r Result[T]) UnwrapErr() error {
	if r.Err == nil {
		panic("called unwrapErr on Ok value")
	}
	return *r.Err
}

// Implement the Validate method for CheckParams
func (params *CheckQueryParams) Validate() error {
	return validateQueryParams(params)
}

// Implement the Validate method for LatencyQueryParams
func (params *LatencyQueryParams) Validate() error {
	return validateQueryParams(params)
}
