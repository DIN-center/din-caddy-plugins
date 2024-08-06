package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPClient_Post(t *testing.T) {
	// Create a new HTTP client
	client := NewHTTPClient()

	// Test cases
	testCases := []struct {
		name           string
		serverResponse string
		serverStatus   int
		headers        map[string]string
		payload        []byte
		expectedBody   []byte
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "Successful POST request",
			serverResponse: `{"message": "Success"}`,
			serverStatus:   http.StatusOK,
			headers:        map[string]string{"X-Custom-Header": "test"},
			payload:        []byte(`{"key": "value"}`),
			expectedBody:   []byte(`{"message": "Success"}`),
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Server error",
			serverResponse: `{"error": "Internal Server Error"}`,
			serverStatus:   http.StatusInternalServerError,
			headers:        nil,
			payload:        []byte(`{"key": "value"}`),
			expectedBody:   []byte(`{"error": "Internal Server Error"}`),
			expectedStatus: http.StatusInternalServerError,
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Check if the custom header is set
				if tc.headers != nil {
					for k, v := range tc.headers {
						assert.Equal(t, v, r.Header.Get(k))
					}
				}

				// Check if Content-Type is set to application/json
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

				// Set the response status code
				w.WriteHeader(tc.serverStatus)

				// Write the response body
				w.Write([]byte(tc.serverResponse))
			}))
			defer server.Close()

			// Make the POST request
			body, status, err := client.Post(server.URL, tc.headers, tc.payload)

			// Check the results
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedBody, body)
				assert.Equal(t, tc.expectedStatus, *status)
			}
		})
	}
}
