package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/openrelayxyz/din-caddy-plugins/lib/auth"
	"github.com/stretchr/testify/assert"
)

func TestHTTPClientPost(t *testing.T) {
	// Create a new HTTP client
	client := NewHTTPClient()

	// Create a mock auth client
	mockCtrl := gomock.NewController(t)
	mockAuthClient := auth.NewMockIAuthClient(mockCtrl)

	// Test cases
	testCases := []struct {
		name           string
		authClient     auth.IAuthClient
		serverResponse string
		serverStatus   int
		headers        map[string]string
		payload        []byte
		expectedBody   []byte
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "Successful POST request, no auth",
			authClient:     nil,
			serverResponse: `{"message": "Success"}`,
			serverStatus:   http.StatusOK,
			headers:        map[string]string{"X-Custom-Header": "test"},
			payload:        []byte(`{"key": "value"}`),
			expectedBody:   []byte(`{"message": "Success"}`),
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Successful POST request, with auth",
			authClient:     mockAuthClient,
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
			authClient:     nil,
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

			if tc.authClient != nil {
				mockAuthClient.EXPECT().Sign(gomock.Any()).Return(nil)
			}
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
			body, status, err := client.Post(server.URL, tc.headers, tc.payload, tc.authClient)

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
