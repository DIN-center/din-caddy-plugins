package modules

import (
	"net/http/httptest"
	"testing"
)

func TestResponseWriterWrapperWrite(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantN    int
		wantErr  bool
		wantBody string
	}{
		{
			name:     "Basic write",
			input:    []byte("test response"),
			wantN:    13,
			wantErr:  false,
			wantBody: "test response",
		},
		// Add more test cases here as needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock http.ResponseWriter
			mockResponseWriter := httptest.NewRecorder()

			// Create a new ResponseWriterWrapper
			rww := NewResponseWriterWrapper(mockResponseWriter)

			// Call the Write method
			gotN, err := rww.Write(tt.input)

			// Check if the response body was captured correctly
			if got := rww.body.String(); got != tt.wantBody {
				t.Errorf("ResponseWriterWrapper.Write() captured body = %s, want %s", got, tt.wantBody)
			}

			// Check if the Write method returned the correct number of bytes written
			if gotN != tt.wantN {
				t.Errorf("ResponseWriterWrapper.Write() returned %d bytes written, want %d", gotN, tt.wantN)
			}

			// Check if the Write method returned any error
			if (err != nil) != tt.wantErr {
				t.Errorf("ResponseWriterWrapper.Write() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
