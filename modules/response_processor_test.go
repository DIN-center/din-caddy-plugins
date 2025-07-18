package modules

import (
	"errors"
	"testing"

	netlib "github.com/DIN-center/din-caddy-plugins/lib/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// MockNetworkHandler for testing
type MockNetworkHandler struct {
	mock.Mock
	netlib.NetworkHandler
}

func (m *MockNetworkHandler) ParseResponse(body []byte, statusCode int) error {
	args := m.Called(body, statusCode)
	return args.Error(0)
}

func (m *MockNetworkHandler) IsRetryableError(err error, statusCode int) bool {
	args := m.Called(err, statusCode)
	return args.Bool(0)
}

func TestNewResponseProcessor(t *testing.T) {
	mockHandler := new(MockNetworkHandler)
	logger := zap.NewNop()

	rp := NewResponseProcessor(mockHandler, logger)

	assert.NotNil(t, rp)
	assert.Equal(t, mockHandler, rp.handler)
	assert.Equal(t, logger, rp.logger)
}

func TestProcessResponse(t *testing.T) {
	tests := []struct {
		name          string
		body          []byte
		statusCode    int
		mockReturn    error
		expectedError error
	}{
		{
			name:          "successful response processing",
			body:          []byte(`{"result": "success"}`),
			statusCode:    200,
			mockReturn:    nil,
			expectedError: nil,
		},
		{
			name:          "error in response processing",
			body:          []byte(`{"error": "invalid"}`),
			statusCode:    200,
			mockReturn:    errors.New("parsing error"),
			expectedError: errors.New("parsing error"),
		},
		{
			name:          "empty body",
			body:          []byte{},
			statusCode:    204,
			mockReturn:    nil,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := new(MockNetworkHandler)
			logger := zap.NewNop()
			rp := NewResponseProcessor(mockHandler, logger)

			mockHandler.On("ParseResponse", tt.body, tt.statusCode).Return(tt.mockReturn)

			err := rp.ProcessResponse(tt.body, tt.statusCode)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			mockHandler.AssertExpectations(t)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		statusCode     int
		mockReturn     bool
		expectedResult bool
	}{
		{
			name:           "retryable timeout error",
			err:            errors.New("timeout"),
			statusCode:     0,
			mockReturn:     true,
			expectedResult: true,
		},
		{
			name:           "non-retryable validation error",
			err:            errors.New("validation failed"),
			statusCode:     400,
			mockReturn:     false,
			expectedResult: false,
		},
		{
			name:           "retryable rate limit error",
			err:            errors.New("rate limit exceeded"),
			statusCode:     429,
			mockReturn:     true,
			expectedResult: true,
		},
		{
			name:           "nil error",
			err:            nil,
			statusCode:     200,
			mockReturn:     false,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := new(MockNetworkHandler)
			logger := zap.NewNop()
			rp := NewResponseProcessor(mockHandler, logger)

			mockHandler.On("IsRetryableError", tt.err, tt.statusCode).Return(tt.mockReturn)

			result := rp.IsRetryableError(tt.err, tt.statusCode)

			assert.Equal(t, tt.expectedResult, result)
			mockHandler.AssertExpectations(t)
		})
	}
}

func TestCheckForApplicationError(t *testing.T) {
	tests := []struct {
		name          string
		responseBody  []byte
		statusCode    int
		reqContext    *RequestProcessor
		mockError     error
		expectedError string
	}{
		{
			name:          "successful 2xx response with no application error",
			responseBody:  []byte(`{"result": "success"}`),
			statusCode:    200,
			reqContext:    &RequestProcessor{},
			mockError:     nil,
			expectedError: "",
		},
		{
			name:          "successful 2xx response with application error",
			responseBody:  []byte(`{"error": {"code": -32601, "message": "Method not found"}}`),
			statusCode:    200,
			reqContext:    &RequestProcessor{},
			mockError:     errors.New("JSON-RPC error: Method not found"),
			expectedError: "JSON-RPC error: Method not found",
		},
		{
			name:          "HTTP 400 error",
			responseBody:  []byte(`{"error": "bad request"}`),
			statusCode:    400,
			reqContext:    &RequestProcessor{},
			mockError:     nil, // Won't be called for non-2xx
			expectedError: "HTTP error: 400",
		},
		{
			name:          "HTTP 500 error",
			responseBody:  []byte(`Internal Server Error`),
			statusCode:    500,
			reqContext:    &RequestProcessor{},
			mockError:     nil, // Won't be called for non-2xx
			expectedError: "HTTP error: 500",
		},
		{
			name:          "HTTP 404 not found",
			responseBody:  []byte(`Not Found`),
			statusCode:    404,
			reqContext:    &RequestProcessor{},
			mockError:     nil, // Won't be called for non-2xx
			expectedError: "HTTP error: 404",
		},
		{
			name:          "successful 204 no content",
			responseBody:  []byte{},
			statusCode:    204,
			reqContext:    &RequestProcessor{},
			mockError:     nil,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockHandler := new(MockNetworkHandler)
			logger := zap.NewNop()
			rp := NewResponseProcessor(mockHandler, logger)

			// Only mock ParseResponse for 2xx status codes
			if tt.statusCode >= 200 && tt.statusCode < 300 {
				mockHandler.On("ParseResponse", tt.responseBody, tt.statusCode).Return(tt.mockError)
			}

			err := rp.CheckForApplicationError(tt.responseBody, tt.statusCode, tt.reqContext)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err.Error())
			} else {
				assert.NoError(t, err)
			}

			mockHandler.AssertExpectations(t)
		})
	}
}

// Test edge cases
func TestResponseProcessorEdgeCases(t *testing.T) {
	t.Run("nil handler panics", func(t *testing.T) {
		assert.Panics(t, func() {
			rp := &ResponseProcessor{
				handler: nil,
				logger:  zap.NewNop(),
			}
			_ = rp.ProcessResponse([]byte{}, 200)
		})
	})

	t.Run("very large response body", func(t *testing.T) {
		mockHandler := new(MockNetworkHandler)
		logger := zap.NewNop()
		rp := NewResponseProcessor(mockHandler, logger)

		// Create a 10MB response body
		largeBody := make([]byte, 10*1024*1024)
		for i := range largeBody {
			largeBody[i] = 'a'
		}

		mockHandler.On("ParseResponse", largeBody, 200).Return(nil)

		err := rp.ProcessResponse(largeBody, 200)
		assert.NoError(t, err)

		mockHandler.AssertExpectations(t)
	})
}
