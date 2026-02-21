package ai

import "encoding/json"

// OpenAIAdapter is a passthrough adapter for OpenAI-compatible providers.
// Since most AI providers natively support the OpenAI format, this adapter
// performs identity transforms — requests and responses pass through unchanged.
type OpenAIAdapter struct{}

func NewOpenAIAdapter() *OpenAIAdapter {
	return &OpenAIAdapter{}
}

func (a *OpenAIAdapter) Name() string {
	return "openai"
}

// TransformRequest passes through the body unchanged. No transformation needed.
func (a *OpenAIAdapter) TransformRequest(body []byte) ([]byte, map[string]string, error) {
	return body, nil, nil
}

// TransformResponse passes through the body unchanged.
func (a *OpenAIAdapter) TransformResponse(body []byte) ([]byte, error) {
	return body, nil
}

// TransformStreamEvent passes through the data unchanged.
// OpenAI only uses "data:" lines (no "event:" field), so eventType is always empty.
func (a *OpenAIAdapter) TransformStreamEvent(eventType string, data []byte) ([]byte, error) {
	return data, nil
}

// IsErrorChunk checks if the SSE data payload contains an error field.
// OpenAI returns errors as JSON with an "error" key, even in streaming mode.
func (a *OpenAIAdapter) IsErrorChunk(eventType string, data []byte) bool {
	var resp struct {
		Error *json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return false
	}
	return resp.Error != nil
}
