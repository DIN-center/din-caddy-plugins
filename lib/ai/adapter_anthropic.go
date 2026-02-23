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
	toolCalls map[int]toolStreamState
}

type toolStreamState struct {
	ID   string
	Name string
}

func NewAnthropicAdapter() *AnthropicAdapter {
	return &AnthropicAdapter{
		toolCalls: make(map[int]toolStreamState),
	}
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
		Model:       req.Model,
		Stream:      req.Stream,
		MaxTokens:   4096, // Anthropic requires max_tokens
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}

	if req.MaxTokens != nil {
		anthropicReq.MaxTokens = *req.MaxTokens
	}
	if len(req.Tools) > 0 {
		anthropicReq.Tools = mapOpenAIToolsToAnthropic(req.Tools)
	}
	if req.ToolChoice != nil {
		anthropicReq.ToolChoice = mapOpenAIToolChoiceToAnthropic(req.ToolChoice)
	}

	// Convert OpenAI stop field to Anthropic stop_sequences.
	if req.Stop != nil {
		switch v := req.Stop.(type) {
		case string:
			anthropicReq.StopSequences = []string{v}
		case []interface{}:
			for _, s := range v {
				if str, ok := s.(string); ok {
					anthropicReq.StopSequences = append(anthropicReq.StopSequences, str)
				}
			}
		default:
			// Unexpected type — ignore gracefully.
		}
	}

	// Extract system messages and convert remaining messages.
	// Multiple system messages are concatenated with newlines (Anthropic only supports one system field).
	var systemParts []string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			text, err := extractSystemText(msg.Content)
			if err != nil {
				return nil, nil, err
			}
			systemParts = append(systemParts, text)
			continue
		}
		if msg.Role == "tool" {
			toolContent, err := extractToolResultContent(msg.Content)
			if err != nil {
				return nil, nil, err
			}
			anthropicReq.Messages = append(anthropicReq.Messages, AnthropicMessage{
				Role: "user",
				Content: []AnthropicRequestContentBlock{
					{
						Type:      "tool_result",
						ToolUseID: msg.ToolCallID,
						Content:   toolContent,
					},
				},
			})
			continue
		}
		anthropicContent, err := toAnthropicContent(msg.Content)
		if err != nil {
			return nil, nil, err
		}
		anthropicContent, err = mergeAssistantToolCalls(msg, anthropicContent)
		if err != nil {
			return nil, nil, err
		}
		anthropicReq.Messages = append(anthropicReq.Messages, AnthropicMessage{
			Role:    msg.Role,
			Content: anthropicContent,
		})
	}
	if len(systemParts) > 0 {
		anthropicReq.System = strings.Join(systemParts, "\n")
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
	var toolCalls []ToolCall
	var unsupportedTypes []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			contentParts = append(contentParts, block.Text)
		} else if block.Type == "tool_use" {
			argsBytes, err := json.Marshal(block.Input)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool_use input: %w", err)
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: string(argsBytes),
				},
			})
		} else if block.Type == "thinking" {
			// No OpenAI equivalent; intentionally skip.
			continue
		} else {
			unsupportedTypes = append(unsupportedTypes, block.Type)
		}
	}
	// If the response has no text content but has unsupported block types (tool_use, thinking, etc.),
	// return an error rather than silently dropping the content.
	// When text IS present alongside unsupported types, the text is returned and unsupported
	// blocks (tool calls, thinking) are intentionally dropped — full tool-call translation is out of scope.
	if len(contentParts) == 0 && len(toolCalls) == 0 && len(unsupportedTypes) > 0 {
		return nil, fmt.Errorf("anthropic response contains only unsupported content types: %v", unsupportedTypes)
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
					Role:      resp.Role,
					Content:   content,
					ToolCalls: toolCalls,
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
		if err := json.Unmarshal(data, &evt); err != nil {
			return nil, fmt.Errorf("failed to parse message_start: %w", err)
		}
		a.messageID = evt.Message.ID
		a.model = evt.Message.Model
		a.toolCalls = make(map[int]toolStreamState)
		// Emit an initial role chunk like OpenAI does.
		return a.buildOpenAIDelta("assistant", "", nil)

	case "content_block_start":
		var start struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock struct {
				Type string `json:"type"`
				ID   string `json:"id,omitempty"`
				Name string `json:"name,omitempty"`
			} `json:"content_block"`
		}
		if err := json.Unmarshal(data, &start); err != nil {
			return nil, fmt.Errorf("failed to parse content_block_start: %w", err)
		}
		if start.ContentBlock.Type == "tool_use" {
			a.toolCalls[start.Index] = toolStreamState{
				ID:   start.ContentBlock.ID,
				Name: start.ContentBlock.Name,
			}
			return a.buildOpenAIToolDelta(start.Index, start.ContentBlock.ID, start.ContentBlock.Name, "")
		}
		return nil, nil

	case "content_block_delta":
		var delta struct {
			Type  string `json:"type"`
			Index int    `json:"index"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &delta); err != nil {
			return nil, fmt.Errorf("failed to parse content_block_delta: %w", err)
		}
		if delta.Delta.Type == "text_delta" {
			return a.buildOpenAIDelta("", delta.Delta.Text, nil)
		}
		if delta.Delta.Type == "input_json_delta" {
			if _, ok := a.toolCalls[delta.Index]; !ok {
				return nil, nil
			}
			return a.buildOpenAIToolDelta(delta.Index, "", "", delta.Delta.PartialJSON)
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

	case "content_block_stop", "ping":
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
	case "tool_use":
		mapped = "tool_calls"
	default:
		mapped = *reason
	}
	return &mapped
}

func extractSystemText(content any) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []ChatMessageContentPart:
		var sb strings.Builder
		for _, part := range v {
			if part.Type != "text" {
				return "", fmt.Errorf("unsupported system content part type: %s", part.Type)
			}
			sb.WriteString(part.Text)
		}
		return sb.String(), nil
	case []any:
		parts, err := normalizeOpenAIContentParts(v)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		for _, part := range parts {
			if part.Type != "text" {
				return "", fmt.Errorf("unsupported system content part type: %s", part.Type)
			}
			sb.WriteString(part.Text)
		}
		return sb.String(), nil
	default:
		return "", fmt.Errorf("unsupported system content type: %T", content)
	}
}

func toAnthropicContent(content any) (any, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []ChatMessageContentPart:
		return mapOpenAIContentPartsToAnthropic(v)
	case []any:
		parts, err := normalizeOpenAIContentParts(v)
		if err != nil {
			return nil, err
		}
		return mapOpenAIContentPartsToAnthropic(parts)
	default:
		return nil, fmt.Errorf("unsupported OpenAI message content type: %T", content)
	}
}

func normalizeOpenAIContentParts(rawParts []any) ([]ChatMessageContentPart, error) {
	parts := make([]ChatMessageContentPart, 0, len(rawParts))
	for _, rawPart := range rawParts {
		partMap, ok := rawPart.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unsupported content part shape: %T", rawPart)
		}
		partType, _ := partMap["type"].(string)
		switch partType {
		case "text":
			text, _ := partMap["text"].(string)
			parts = append(parts, ChatMessageContentPart{
				Type: "text",
				Text: text,
			})
		case "image_url":
			imageMap, ok := partMap["image_url"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("image_url part must include image_url object")
			}
			imageURL, _ := imageMap["url"].(string)
			if imageURL == "" {
				return nil, fmt.Errorf("image_url part must include a non-empty url")
			}
			parts = append(parts, ChatMessageContentPart{
				Type: "image_url",
				ImageURL: &ChatMessageImageURLValue{
					URL: imageURL,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported content part type: %s", partType)
		}
	}
	return parts, nil
}

func mapOpenAIContentPartsToAnthropic(parts []ChatMessageContentPart) ([]AnthropicRequestContentBlock, error) {
	blocks := make([]AnthropicRequestContentBlock, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "text":
			blocks = append(blocks, AnthropicRequestContentBlock{
				Type: "text",
				Text: part.Text,
			})
		case "image_url":
			if part.ImageURL == nil || part.ImageURL.URL == "" {
				return nil, fmt.Errorf("image_url part must include a non-empty url")
			}
			if strings.HasPrefix(part.ImageURL.URL, "data:") {
				mediaType, data, err := parseBase64DataURL(part.ImageURL.URL)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, AnthropicRequestContentBlock{
					Type: "image",
					Source: &AnthropicImageSource{
						Type:      "base64",
						MediaType: mediaType,
						Data:      data,
					},
				})
				continue
			}
			blocks = append(blocks, AnthropicRequestContentBlock{
				Type: "image",
				Source: &AnthropicImageSource{
					Type: "url",
					URL:  part.ImageURL.URL,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported content part type: %s", part.Type)
		}
	}
	return blocks, nil
}

func mapOpenAIToolsToAnthropic(tools []OpenAITool) []AnthropicTool {
	result := make([]AnthropicTool, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}
		result = append(result, AnthropicTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}
	return result
}

func mapOpenAIToolChoiceToAnthropic(choice any) any {
	switch v := choice.(type) {
	case string:
		switch v {
		case "required":
			return map[string]any{"type": "any"}
		case "auto":
			return map[string]any{"type": "auto"}
		default:
			return map[string]any{"type": "auto"}
		}
	case map[string]any:
		if v["type"] == "function" {
			if fn, ok := v["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok && name != "" {
					return map[string]any{
						"type": "tool",
						"name": name,
					}
				}
			}
		}
	}
	return nil
}

func extractToolResultContent(content any) (string, error) {
	switch v := content.(type) {
	case string:
		return v, nil
	case []ChatMessageContentPart:
		var sb strings.Builder
		for _, part := range v {
			if part.Type != "text" {
				return "", fmt.Errorf("tool result supports only text content parts")
			}
			sb.WriteString(part.Text)
		}
		return sb.String(), nil
	case []any:
		parts, err := normalizeOpenAIContentParts(v)
		if err != nil {
			return "", err
		}
		var sb strings.Builder
		for _, part := range parts {
			if part.Type != "text" {
				return "", fmt.Errorf("tool result supports only text content parts")
			}
			sb.WriteString(part.Text)
		}
		return sb.String(), nil
	default:
		return "", fmt.Errorf("unsupported tool result content type: %T", content)
	}
}

func mergeAssistantToolCalls(msg ChatMessage, anthropicContent any) (any, error) {
	if len(msg.ToolCalls) == 0 {
		return anthropicContent, nil
	}

	blocks := []AnthropicRequestContentBlock{}
	switch v := anthropicContent.(type) {
	case string:
		if v != "" {
			blocks = append(blocks, AnthropicRequestContentBlock{
				Type: "text",
				Text: v,
			})
		}
	case []AnthropicRequestContentBlock:
		blocks = append(blocks, v...)
	default:
		return nil, fmt.Errorf("unsupported assistant content container for tool calls: %T", anthropicContent)
	}

	for _, tc := range msg.ToolCalls {
		var input map[string]any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				return nil, fmt.Errorf("invalid tool call arguments for %s: %w", tc.Function.Name, err)
			}
		}
		blocks = append(blocks, AnthropicRequestContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return blocks, nil
}

func (a *AnthropicAdapter) buildOpenAIToolDelta(index int, id, name, partialArgs string) ([]byte, error) {
	idx := index
	tc := ToolCall{
		Index:    &idx,
		Function: ToolCallFunction{Arguments: partialArgs},
	}
	if id != "" {
		tc.ID = id
		tc.Type = "function"
		tc.Function.Name = name
	}
	chunk := ChatCompletionResponse{
		ID:      a.messageID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   a.model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Delta: &ChatMessage{ToolCalls: []ToolCall{tc}},
			},
		},
	}
	return json.Marshal(chunk)
}

func parseBase64DataURL(dataURL string) (string, string, error) {
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return "", "", fmt.Errorf("invalid image data URL format")
	}
	prefix := dataURL[:comma]
	data := dataURL[comma+1:]

	if !strings.Contains(prefix, ";base64") {
		return "", "", fmt.Errorf("image data URL must be base64 encoded")
	}

	mediaType := strings.TrimPrefix(strings.Split(prefix, ";")[0], "data:")
	if mediaType == "" {
		mediaType = "image/png"
	}
	if data == "" {
		return "", "", fmt.Errorf("image data URL payload is empty")
	}
	return mediaType, data, nil
}
