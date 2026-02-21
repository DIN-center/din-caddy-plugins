package ai

// ProviderAdapter normalizes AI provider APIs to the OpenAI-compatible format.
// Each provider (OpenAI, Anthropic, etc.) has its own adapter implementation.
type ProviderAdapter interface {
	// TransformRequest converts an OpenAI-format request body to the provider's native format.
	// Returns the transformed body and any additional headers to set on the outgoing request.
	TransformRequest(body []byte) ([]byte, map[string]string, error)

	// TransformResponse converts a provider's non-streaming response to OpenAI format.
	TransformResponse(body []byte) ([]byte, error)

	// TransformStreamEvent converts an SSE event (type + data) to an OpenAI-format SSE data payload.
	// eventType is the SSE "event:" field (empty string for OpenAI which only uses "data:").
	// Returns the transformed "data:" payload bytes, or nil to skip this event (e.g. Anthropic metadata events).
	TransformStreamEvent(eventType string, data []byte) ([]byte, error)

	// IsErrorChunk checks if an SSE data payload indicates a provider-side error.
	// Used for first-chunk validation before committing to streaming.
	IsErrorChunk(eventType string, data []byte) bool

	// Name returns the adapter type identifier (e.g., "openai", "anthropic").
	Name() string
}
