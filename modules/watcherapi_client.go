package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

// CheckParams represents the parameters for a check request
type CheckParams struct {
	CheckID    string   `query:"check_id" validate:"required,oneof=blockNumberConsistency blockNonStateConsistency"`
	Network    string   `query:"network" validate:"required"`
	ProviderID []string `query:"provider_id"`
	Region     []string `query:"region" validate:"dive,oneof=NorthAmerica Latam Europe MiddleEastAfrica AsiaPacific"`
	Interval   string   `query:"interval" validate:"interval_format"`
}

// CheckResponse represents the response for a check request
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
	Provider           string  `json:"provider"`
	ProviderLocation   string  `json:"provider_location"`
	EndpointURL        string  `json:"endpoint_url"`
	BlockNumberFrom    int     `json:"block_number_from"`
	BlockNumberTo      int     `json:"block_number_to"`
	LastCheckTimestamp string  `json:"lastest_check_ts"`
	CheckSummary       Summary `json:"check_summary"`
	ResponseStatus     Status  `json:"response_status"`
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
	Provider          string       `json:"provider"`
	ProviderLocation  string       `json:"provider_location"`
	EndpointURL       string       `json:"endpoint_url"`
	LastPingTimestamp string       `json:"lastest_ping_ts"`
	MaxRPS            float64      `json:"max_rps"`
	Latency           LatencyStats `json:"latency"`
	ResponseStatus    Status       `json:"response_status"`
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

type WatcherAPIClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

const CHECK_RESOURCE_PATH = "/v1/monitoring/check"
const LATENCY_RESOURCE_PATH = "/v1/monitoring/latency"

var validate *validator.Validate

// Initialize the validator when the package is loaded
func init() {
	validate = validator.New()

	// Register custom validation for interval format
	validate.RegisterValidation("interval_format", func(fl validator.FieldLevel) bool {
		interval := fl.Field().String()
		if interval == "" {
			return true // empty values are allowed
		}
		// Match {number}[min|h|d|w]
		re := regexp.MustCompile(`^\d+(min|h|d|w)$`)
		return re.MatchString(interval)
	})
}

func ValidateQueryParams(params interface{}) error {
	err := validate.Struct(params)
	if err != nil {
		// Format validation errors into a human-readable format
		var validationErrors []string
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors = append(validationErrors, fmt.Sprintf("Field '%s': %s", err.Field(), err.Tag()))
		}
		return fmt.Errorf("validation failed: %s", strings.Join(validationErrors, ", "))
	}
	return nil
}

// Implement the Validate method for CheckParams
func (params *CheckParams) Validate() error {
	return ValidateQueryParams(params)
}

// Implement the Validate method for LatencyQueryParams
func (params *LatencyQueryParams) Validate() error {
	return ValidateQueryParams(params)
}

// NewClient creates a new instance of the Watcher API client.
func NewClient(baseURL, apiKey string) *WatcherAPIClient {
	return &WatcherAPIClient{
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{},
	}
}

// buildQuery builds a query string from the given parameters.
func buildQuery(params interface{}) Result[string] {
	values := url.Values{}

	// Reflect on the struct fields
	v := reflect.ValueOf(params)
	t := reflect.TypeOf(params)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		queryKey := fieldType.Tag.Get("query")

		// Skip empty fields
		if queryKey == "" || (field.Kind() == reflect.String && field.String() == "") {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			values.Add(queryKey, field.String())
		case reflect.Slice:
			slice := field.Interface().([]string)
			if len(slice) > 0 {
				values.Add(queryKey, strings.Join(slice, ","))
			}
		case reflect.Int:
			if field.Int() > 0 {
				values.Add(queryKey, strconv.Itoa(int(field.Int())))
			}
		}
	}

	return Ok(values.Encode())
}

func (c *WatcherAPIClient) doRequest(method, endpoint string, params interface{}) Result[[]byte] {
	// Build query string
	queryResult := buildQuery(params)
	if queryResult.IsErr() {
		return Err[[]byte](fmt.Errorf("failed to build query: %w", queryResult.UnwrapErr()))
	}
	queryString := queryResult.Unwrap()
	if queryString != "" {
		endpoint = fmt.Sprintf("%s?%s", endpoint, queryString)
	}

	// Create full URL
	url := fmt.Sprintf("%s%s", c.BaseURL, endpoint)
	fmt.Println("url", url)
	// Create HTTP request
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return Err[[]byte](fmt.Errorf("failed to create request: %w", err))
	}

	// Add headers
	req.Header.Set("X-API-KEY", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	// Make HTTP request
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return Err[[]byte](fmt.Errorf("request failed: %w", err))
	}
	defer resp.Body.Close()

	// Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		var errResp = &ErrorResponse{}
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return Err[[]byte](fmt.Errorf("unexpected status code: %d", resp.StatusCode))
		}
		return Err[[]byte](fmt.Errorf("status code: %d, message: %s", resp.StatusCode, errResp.Message))
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Err[[]byte](fmt.Errorf("failed to read response: %w", err))
	}

	return Ok(respBody)
}

func (c *WatcherAPIClient) GetCheck(params CheckParams) Result[CheckResponse] {
	// Validate parameters
	if err := params.Validate(); err != nil {
		return Err[CheckResponse](err)
	}

	// Make the API call
	respBody := c.doRequest("GET", CHECK_RESOURCE_PATH, params)
	if respBody.IsErr() {
		return Err[CheckResponse](respBody.UnwrapErr())
	}

	var response CheckResponse
	if err := json.Unmarshal(respBody.Unwrap(), &response); err != nil {
		return Err[CheckResponse](fmt.Errorf("failed to parse response: %w", err))
	}

	return Ok(response)
}

func (c *WatcherAPIClient) GetLatency(params LatencyQueryParams) Result[LatencyResponse] {
	// Validate parameters
	if err := params.Validate(); err != nil {
		return Err[LatencyResponse](err)
	}

	// Make the API call
	respBody := c.doRequest("GET", LATENCY_RESOURCE_PATH, params)
	if respBody.IsErr() {
		return Err[LatencyResponse](respBody.UnwrapErr())
	}

	var response LatencyResponse
	if err := json.Unmarshal(respBody.Unwrap(), &response); err != nil {
		return Err[LatencyResponse](fmt.Errorf("failed to parse response: %w", err))
	}

	return Ok(response)
}
