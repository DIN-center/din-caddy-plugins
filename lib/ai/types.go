package ai

// ChatCompletionRequest represents an OpenAI-compatible chat completion request.
type ChatCompletionRequest struct {
	Model            string        `json:"model,omitempty"`
	Messages         []ChatMessage `json:"messages"`
	Stream           bool          `json:"stream,omitempty"`
	Temperature      *float64      `json:"temperature,omitempty"`
	MaxTokens        *int          `json:"max_tokens,omitempty"`
	TopP             *float64      `json:"top_p,omitempty"`
	FrequencyPenalty *float64      `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64      `json:"presence_penalty,omitempty"`
	Stop             any           `json:"stop,omitempty"`
	N                *int          `json:"n,omitempty"`
	User             string        `json:"user,omitempty"`
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse represents an OpenAI-compatible chat completion response.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   *UsageInfo             `json:"usage,omitempty"`
}

// ChatCompletionChoice represents a single choice in a completion response.
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      *ChatMessage `json:"message,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"`
	FinishReason *string     `json:"finish_reason,omitempty"`
}

// UsageInfo tracks token consumption for a request.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// SSEEvent represents a parsed Server-Sent Event.
type SSEEvent struct {
	Event string // the "event:" field (empty for OpenAI, populated for Anthropic)
	Data  []byte // the "data:" field payload
}

// ErrorResponse represents an OpenAI-compatible error response.
type ErrorResponse struct {
	Error *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail contains error information.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

// --- Anthropic Types ---

// AnthropicRequest represents an Anthropic Messages API request.
type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []AnthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
	System    string             `json:"system,omitempty"`
}

// AnthropicMessage represents a message in the Anthropic format.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicResponse represents an Anthropic Messages API response.
type AnthropicResponse struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Role         string            `json:"role"`
	Content      []AnthropicContent `json:"content"`
	Model        string            `json:"model"`
	StopReason   *string           `json:"stop_reason,omitempty"`
	StopSequence *string           `json:"stop_sequence,omitempty"`
	Usage        *AnthropicUsage   `json:"usage,omitempty"`
}

// AnthropicContent represents a content block in an Anthropic response.
type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicUsage represents token usage in the Anthropic format.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicStreamEvent represents a streaming event from the Anthropic API.
type AnthropicStreamEvent struct {
	Type string `json:"type"`
}

// AnthropicContentBlockDelta represents a content_block_delta streaming event.
type AnthropicContentBlockDelta struct {
	Type  string                      `json:"type"`
	Index int                         `json:"index"`
	Delta AnthropicContentBlockText   `json:"delta"`
}

// AnthropicContentBlockText represents the delta text in a streaming event.
type AnthropicContentBlockText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// AnthropicMessageDelta represents a message_delta streaming event.
type AnthropicMessageDelta struct {
	Type  string                    `json:"type"`
	Delta AnthropicMessageDeltaBody `json:"delta"`
	Usage *AnthropicUsage           `json:"usage,omitempty"`
}

// AnthropicMessageDeltaBody contains the stop reason from a message_delta event.
type AnthropicMessageDeltaBody struct {
	StopReason   *string `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
}

// AnthropicErrorEvent represents an error event from Anthropic's streaming API.
type AnthropicErrorEvent struct {
	Type  string              `json:"type"`
	Error AnthropicErrorDetail `json:"error"`
}

// AnthropicErrorDetail contains Anthropic error information.
type AnthropicErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
