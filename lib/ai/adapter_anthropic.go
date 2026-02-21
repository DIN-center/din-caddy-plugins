package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AnthropicAdapter translates between OpenAI chat format and Anthropic's Messages API.
// It is stateful for streaming — it tracks event sequences to correctly assemble
// OpenAI-format delta chunks from Anthropic's multi-event protocol.
//
// NOT safe for concurrent use. Each streaming request must use its own adapter instance.
type AnthropicAdapter struct {
	// Streaming state — tracks the current message context.
	messageID string
	model     string
}

func NewAnthropicAdapter() *AnthropicAdapter {
	return &AnthropicAdapter{}
}

func (a *AnthropicAdapter) Name() string {
	return "anthropic"
}

// TransformRequest converts an OpenAI ChatCompletionRequest to Anthropic Messages API format.
func (a *AnthropicAdapter) TransformRequest(body []byte) ([]byte, map[string]string, error) {
	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, nil, fmt.Errorf("failed to parse OpenAI request: %w", err)
	}

	anthropicReq := AnthropicRequest{
		Model:     req.Model,
		Stream:    req.Stream,
		MaxTokens: 4096, // Anthropic requires max_tokens
	}

	if req.MaxTokens != nil {
		anthropicReq.MaxTokens = *req.MaxTokens
	}

	// Extract system message and convert remaining messages.
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			anthropicReq.System = msg.Content
			continue
		}
		anthropicReq.Messages = append(anthropicReq.Messages, AnthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	out, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal Anthropic request: %w", err)
	}

	return out, nil, nil
}

// TransformResponse converts an Anthropic Messages API response to OpenAI format.
func (a *AnthropicAdapter) TransformResponse(body []byte) ([]byte, error) {
	var resp AnthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic response: %w", err)
	}

	// Build content from content blocks.
	var contentParts []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			contentParts = append(contentParts, block.Text)
		}
	}
	content := ""
	if len(contentParts) == 1 {
		content = contentParts[0]
	} else if len(contentParts) > 1 {
		content = strings.Join(contentParts, "")
	}

	finishReason := mapAnthropicStopReason(resp.StopReason)

	openAIResp := ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: &ChatMessage{
					Role:    resp.Role,
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
	}

	if resp.Usage != nil {
		openAIResp.Usage = &UsageInfo{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}

	out, err := json.Marshal(openAIResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI response: %w", err)
	}
	return out, nil
}

// TransformStreamEvent converts an Anthropic SSE event to OpenAI SSE format.
// Returns nil to skip events that have no OpenAI equivalent (e.g. message_start metadata).
func (a *AnthropicAdapter) TransformStreamEvent(eventType string, data []byte) ([]byte, error) {
	switch eventType {
	case "message_start":
		// Extract message ID and model for subsequent events.
		var evt struct {
			Type    string `json:"type"`
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &evt); err == nil {
			a.messageID = evt.Message.ID
			a.model = evt.Message.Model
		}
		// Emit an initial role chunk like OpenAI does.
		return a.buildOpenAIDelta("assistant", "", nil)

	case "content_block_delta":
		var delta AnthropicContentBlockDelta
		if err := json.Unmarshal(data, &delta); err != nil {
			return nil, fmt.Errorf("failed to parse content_block_delta: %w", err)
		}
		if delta.Delta.Type == "text_delta" {
			return a.buildOpenAIDelta("", delta.Delta.Text, nil)
		}
		return nil, nil // skip non-text deltas

	case "message_delta":
		var delta AnthropicMessageDelta
		if err := json.Unmarshal(data, &delta); err != nil {
			return nil, fmt.Errorf("failed to parse message_delta: %w", err)
		}
		finishReason := mapAnthropicStopReason(delta.Delta.StopReason)
		return a.buildOpenAIDelta("", "", finishReason)

	case "message_stop":
		return []byte("[DONE]"), nil

	case "content_block_start", "content_block_stop", "ping":
		// Skip metadata events — no OpenAI equivalent.
		return nil, nil

	default:
		// Unknown event type — skip.
		return nil, nil
	}
}

// IsErrorChunk checks if the SSE event is an Anthropic error.
func (a *AnthropicAdapter) IsErrorChunk(eventType string, data []byte) bool {
	if eventType == "error" {
		return true
	}
	// Also check the data for error type field.
	var evt struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &evt); err == nil {
		return evt.Type == "error"
	}
	return false
}

// buildOpenAIDelta constructs an OpenAI-format streaming chunk.
func (a *AnthropicAdapter) buildOpenAIDelta(role, content string, finishReason *string) ([]byte, error) {
	delta := &ChatMessage{}
	if role != "" {
		delta.Role = role
	}
	if content != "" {
		delta.Content = content
	}

	chunk := ChatCompletionResponse{
		ID:      a.messageID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   a.model,
		Choices: []ChatCompletionChoice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	}

	out, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// mapAnthropicStopReason maps Anthropic stop reasons to OpenAI finish reasons.
func mapAnthropicStopReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	var mapped string
	switch *reason {
	case "end_turn":
		mapped = "stop"
	case "max_tokens":
		mapped = "length"
	case "stop_sequence":
		mapped = "stop"
	default:
		mapped = *reason
	}
	return &mapped
}
