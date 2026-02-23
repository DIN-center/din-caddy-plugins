package ai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- OpenAI Adapter Tests ---

func TestOpenAIAdapterName(t *testing.T) {
	a := NewOpenAIAdapter()
	assert.Equal(t, "openai", a.Name())
}

func TestOpenAIAdapterTransformRequest(t *testing.T) {
	a := NewOpenAIAdapter()

	t.Run("non-streaming passthrough", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}]}`)
		out, headers, err := a.TransformRequest(body)
		assert.NoError(t, err)
		assert.Equal(t, body, out)
		assert.Nil(t, headers)
	})

	t.Run("streaming injects stream_options", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"stream":true}`)
		out, headers, err := a.TransformRequest(body)
		assert.NoError(t, err)
		assert.Nil(t, headers)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(out, &raw))
		assert.Contains(t, string(raw["stream_options"]), `"include_usage":true`)
	})

	t.Run("streaming preserves existing stream_options", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4o","messages":[],"stream":true,"stream_options":{"include_usage":false}}`)
		out, headers, err := a.TransformRequest(body)
		assert.NoError(t, err)
		assert.Nil(t, headers)

		var raw map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(out, &raw))
		// Should NOT overwrite existing stream_options.
		assert.Contains(t, string(raw["stream_options"]), `"include_usage":false`)
	})

	t.Run("stream false no injection", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4o","messages":[],"stream":false}`)
		out, headers, err := a.TransformRequest(body)
		assert.NoError(t, err)
		assert.Equal(t, body, out)
		assert.Nil(t, headers)
	})

	t.Run("invalid json passthrough", func(t *testing.T) {
		body := []byte(`not json`)
		out, headers, err := a.TransformRequest(body)
		assert.NoError(t, err)
		assert.Equal(t, body, out)
		assert.Nil(t, headers)
	})
}

func TestOpenAIAdapterTransformResponse(t *testing.T) {
	a := NewOpenAIAdapter()
	body := []byte(`{"id":"chatcmpl-123","choices":[{"message":{"content":"hi"}}]}`)

	out, err := a.TransformResponse(body)
	assert.NoError(t, err)
	assert.Equal(t, body, out) // passthrough
}

func TestOpenAIAdapterTransformStreamEvent(t *testing.T) {
	a := NewOpenAIAdapter()
	data := []byte(`{"choices":[{"delta":{"content":"Hello"}}]}`)

	out, err := a.TransformStreamEvent("", data)
	assert.NoError(t, err)
	assert.Equal(t, data, out) // passthrough
}

func TestOpenAIAdapterIsErrorChunk(t *testing.T) {
	a := NewOpenAIAdapter()

	tests := []struct {
		name    string
		data    string
		isError bool
	}{
		{
			name:    "normal content",
			data:    `{"choices":[{"delta":{"content":"Hello"}}]}`,
			isError: false,
		},
		{
			name:    "error response",
			data:    `{"error":{"message":"rate limited","type":"tokens"}}`,
			isError: true,
		},
		{
			name:    "malformed json",
			data:    `not json`,
			isError: false,
		},
		{
			name:    "DONE marker",
			data:    `[DONE]`,
			isError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isError, a.IsErrorChunk("", []byte(tt.data)))
		})
	}
}

// --- Anthropic Adapter Tests ---

func TestAnthropicAdapterName(t *testing.T) {
	a := NewAnthropicAdapter()
	assert.Equal(t, "anthropic", a.Name())
}

func TestAnthropicAdapterTransformRequest(t *testing.T) {
	a := NewAnthropicAdapter()

	t.Run("basic request with system message", func(t *testing.T) {
		input := ChatCompletionRequest{
			Model: "claude-sonnet-4-20250514",
			Messages: []ChatMessage{
				{Role: "system", Content: "You are helpful."},
				{Role: "user", Content: "Hello"},
			},
			Stream: true,
		}
		body, _ := json.Marshal(input)

		out, headers, err := a.TransformRequest(body)
		require.NoError(t, err)
		assert.Nil(t, headers)

		var result AnthropicRequest
		require.NoError(t, json.Unmarshal(out, &result))

		assert.Equal(t, "claude-sonnet-4-20250514", result.Model)
		assert.Equal(t, "You are helpful.", result.System)
		assert.True(t, result.Stream)
		assert.Len(t, result.Messages, 1) // system message extracted
		assert.Equal(t, "user", result.Messages[0].Role)
		assert.Equal(t, "Hello", result.Messages[0].Content)
	})

	t.Run("request with max_tokens", func(t *testing.T) {
		maxTokens := 100
		input := ChatCompletionRequest{
			Model: "claude-haiku-4-5-20251001",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hi"},
			},
			MaxTokens: &maxTokens,
		}
		body, _ := json.Marshal(input)

		out, _, err := a.TransformRequest(body)
		require.NoError(t, err)

		var result AnthropicRequest
		require.NoError(t, json.Unmarshal(out, &result))
		assert.Equal(t, 100, result.MaxTokens)
	})

	t.Run("request without max_tokens gets default", func(t *testing.T) {
		input := ChatCompletionRequest{
			Model: "claude-haiku-4-5-20251001",
			Messages: []ChatMessage{
				{Role: "user", Content: "Hi"},
			},
		}
		body, _ := json.Marshal(input)

		out, _, err := a.TransformRequest(body)
		require.NoError(t, err)

		var result AnthropicRequest
		require.NoError(t, json.Unmarshal(out, &result))
		assert.Equal(t, 4096, result.MaxTokens)
	})

	t.Run("malformed json returns error", func(t *testing.T) {
		_, _, err := a.TransformRequest([]byte(`not json`))
		assert.Error(t, err)
	})

	t.Run("multimodal text-only content array", func(t *testing.T) {
		input := `{
			"model":"claude-sonnet-4-20250514",
			"messages":[
				{
					"role":"user",
					"content":[
						{"type":"text","text":"hello"},
						{"type":"text","text":" world"}
					]
				}
			]
		}`
		out, _, err := a.TransformRequest([]byte(input))
		require.NoError(t, err)

		var result AnthropicRequest
		require.NoError(t, json.Unmarshal(out, &result))
		require.Len(t, result.Messages, 1)
		blocks, ok := result.Messages[0].Content.([]interface{})
		require.True(t, ok)
		require.Len(t, blocks, 2)
		first, ok := blocks[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "text", first["type"])
		assert.Equal(t, "hello", first["text"])
	})

	t.Run("multimodal mixed text and image_url", func(t *testing.T) {
		input := `{
			"model":"claude-sonnet-4-20250514",
			"messages":[
				{
					"role":"user",
					"content":[
						{"type":"text","text":"describe image"},
						{"type":"image_url","image_url":{"url":"https://example.com/cat.png"}}
					]
				}
			]
		}`
		out, _, err := a.TransformRequest([]byte(input))
		require.NoError(t, err)

		var result AnthropicRequest
		require.NoError(t, json.Unmarshal(out, &result))
		require.Len(t, result.Messages, 1)
		blocks, ok := result.Messages[0].Content.([]interface{})
		require.True(t, ok)
		require.Len(t, blocks, 2)

		second, ok := blocks[1].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "image", second["type"])
		source, ok := second["source"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "url", source["type"])
		assert.Equal(t, "https://example.com/cat.png", source["url"])
	})

	t.Run("multimodal image_url base64 data URL", func(t *testing.T) {
		input := `{
			"model":"claude-sonnet-4-20250514",
			"messages":[
				{
					"role":"user",
					"content":[
						{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}
					]
				}
			]
		}`
		out, _, err := a.TransformRequest([]byte(input))
		require.NoError(t, err)

		var result AnthropicRequest
		require.NoError(t, json.Unmarshal(out, &result))
		blocks, ok := result.Messages[0].Content.([]interface{})
		require.True(t, ok)
		block, ok := blocks[0].(map[string]interface{})
		require.True(t, ok)
		source, ok := block["source"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "base64", source["type"])
		assert.Equal(t, "image/png", source["media_type"])
		assert.Equal(t, "aGVsbG8=", source["data"])
	})

	t.Run("unsupported multimodal content part returns error", func(t *testing.T) {
		input := `{
			"model":"claude-sonnet-4-20250514",
			"messages":[
				{
					"role":"user",
					"content":[
						{"type":"audio_url","audio_url":{"url":"https://example.com/a.mp3"}}
					]
				}
			]
		}`
		_, _, err := a.TransformRequest([]byte(input))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported content part type")
	})

	t.Run("maps tools and function tool_choice", func(t *testing.T) {
		input := `{
			"model":"claude-sonnet-4-20250514",
			"messages":[{"role":"user","content":"weather?"}],
			"tools":[
				{
					"type":"function",
					"function":{
						"name":"get_weather",
						"description":"Get weather for a city",
						"parameters":{"type":"object","properties":{"city":{"type":"string"}}}
					}
				}
			],
			"tool_choice":{"type":"function","function":{"name":"get_weather"}}
		}`
		out, _, err := a.TransformRequest([]byte(input))
		require.NoError(t, err)

		var result AnthropicRequest
		require.NoError(t, json.Unmarshal(out, &result))
		require.Len(t, result.Tools, 1)
		assert.Equal(t, "get_weather", result.Tools[0].Name)

		choice, ok := result.ToolChoice.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "tool", choice["type"])
		assert.Equal(t, "get_weather", choice["name"])
	})
}

func TestAnthropicAdapterTransformResponse(t *testing.T) {
	a := NewAnthropicAdapter()

	t.Run("basic response", func(t *testing.T) {
		stopReason := "end_turn"
		anthropicResp := AnthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []AnthropicContent{
				{Type: "text", Text: "Hello! How can I help?"},
			},
			Model:      "claude-sonnet-4-20250514",
			StopReason: &stopReason,
			Usage: &AnthropicUsage{
				InputTokens:  10,
				OutputTokens: 8,
			},
		}
		body, _ := json.Marshal(anthropicResp)

		out, err := a.TransformResponse(body)
		require.NoError(t, err)

		var result ChatCompletionResponse
		require.NoError(t, json.Unmarshal(out, &result))

		assert.Equal(t, "msg_123", result.ID)
		assert.Equal(t, "chat.completion", result.Object)
		assert.Equal(t, "claude-sonnet-4-20250514", result.Model)
		assert.Len(t, result.Choices, 1)
		assert.Equal(t, "assistant", result.Choices[0].Message.Role)
		assert.Equal(t, "Hello! How can I help?", result.Choices[0].Message.Content)
		assert.NotNil(t, result.Choices[0].FinishReason)
		assert.Equal(t, "stop", *result.Choices[0].FinishReason)
		assert.Equal(t, 10, result.Usage.PromptTokens)
		assert.Equal(t, 8, result.Usage.CompletionTokens)
		assert.Equal(t, 18, result.Usage.TotalTokens)
	})

	t.Run("malformed json returns error", func(t *testing.T) {
		_, err := a.TransformResponse([]byte(`not json`))
		assert.Error(t, err)
	})
}

func TestAnthropicAdapterTransformStreamEvent(t *testing.T) {
	a := NewAnthropicAdapter()

	t.Run("message_start emits role delta", func(t *testing.T) {
		data := []byte(`{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4-20250514","role":"assistant"}}`)

		out, err := a.TransformStreamEvent("message_start", data)
		require.NoError(t, err)
		require.NotNil(t, out)

		var chunk ChatCompletionResponse
		require.NoError(t, json.Unmarshal(out, &chunk))
		assert.Equal(t, "msg_1", chunk.ID)
		assert.Equal(t, "chat.completion.chunk", chunk.Object)
		assert.Equal(t, "assistant", chunk.Choices[0].Delta.Role)
	})

	t.Run("content_block_delta emits text delta", func(t *testing.T) {
		data := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)

		out, err := a.TransformStreamEvent("content_block_delta", data)
		require.NoError(t, err)
		require.NotNil(t, out)

		var chunk ChatCompletionResponse
		require.NoError(t, json.Unmarshal(out, &chunk))
		assert.Equal(t, "Hello", chunk.Choices[0].Delta.Content)
	})

	t.Run("message_delta emits finish reason", func(t *testing.T) {
		data := []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)

		out, err := a.TransformStreamEvent("message_delta", data)
		require.NoError(t, err)
		require.NotNil(t, out)

		var chunk ChatCompletionResponse
		require.NoError(t, json.Unmarshal(out, &chunk))
		require.NotNil(t, chunk.Choices[0].FinishReason)
		assert.Equal(t, "stop", *chunk.Choices[0].FinishReason)
	})

	t.Run("message_stop emits DONE", func(t *testing.T) {
		out, err := a.TransformStreamEvent("message_stop", []byte(`{"type":"message_stop"}`))
		require.NoError(t, err)
		assert.Equal(t, []byte("[DONE]"), out)
	})

	t.Run("content_block_start is skipped", func(t *testing.T) {
		out, err := a.TransformStreamEvent("content_block_start", []byte(`{}`))
		assert.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("ping is skipped", func(t *testing.T) {
		out, err := a.TransformStreamEvent("ping", []byte(`{"type":"ping"}`))
		assert.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("unknown event is skipped", func(t *testing.T) {
		out, err := a.TransformStreamEvent("something_new", []byte(`{}`))
		assert.NoError(t, err)
		assert.Nil(t, out)
	})
}

func TestAnthropicAdapterIsErrorChunk(t *testing.T) {
	a := NewAnthropicAdapter()

	tests := []struct {
		name      string
		eventType string
		data      string
		isError   bool
	}{
		{
			name:      "error event type",
			eventType: "error",
			data:      `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
			isError:   true,
		},
		{
			name:      "error in data type field",
			eventType: "",
			data:      `{"type":"error","error":{"type":"rate_limit_error","message":"Rate limited"}}`,
			isError:   true,
		},
		{
			name:      "normal content_block_delta",
			eventType: "content_block_delta",
			data:      `{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}`,
			isError:   false,
		},
		{
			name:      "message_start is not error",
			eventType: "message_start",
			data:      `{"type":"message_start","message":{"id":"msg_1"}}`,
			isError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isError, a.IsErrorChunk(tt.eventType, []byte(tt.data)))
		})
	}
}

func TestAnthropicAdapterFullStreamSequence(t *testing.T) {
	a := NewAnthropicAdapter()

	// Simulate a full Anthropic streaming sequence
	events := []struct {
		eventType string
		data      string
	}{
		{"message_start", `{"type":"message_start","message":{"id":"msg_abc","model":"claude-sonnet-4-20250514","role":"assistant"}}`},
		{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
		{"ping", `{"type":"ping"}`},
		{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`},
		{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`},
		{"content_block_stop", `{"type":"content_block_stop","index":0}`},
		{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`},
		{"message_stop", `{"type":"message_stop"}`},
	}

	var outputs [][]byte
	for _, evt := range events {
		out, err := a.TransformStreamEvent(evt.eventType, []byte(evt.data))
		require.NoError(t, err)
		if out != nil {
			outputs = append(outputs, out)
		}
	}

	// Should have: role delta, "Hello" delta, " world" delta, finish reason, [DONE]
	assert.Len(t, outputs, 5)

	// First output: role
	var first ChatCompletionResponse
	require.NoError(t, json.Unmarshal(outputs[0], &first))
	assert.Equal(t, "assistant", first.Choices[0].Delta.Role)
	assert.Equal(t, "msg_abc", first.ID)

	// Second output: "Hello"
	var second ChatCompletionResponse
	require.NoError(t, json.Unmarshal(outputs[1], &second))
	assert.Equal(t, "Hello", second.Choices[0].Delta.Content)

	// Third output: " world"
	var third ChatCompletionResponse
	require.NoError(t, json.Unmarshal(outputs[2], &third))
	assert.Equal(t, " world", third.Choices[0].Delta.Content)

	// Fourth output: finish reason
	var fourth ChatCompletionResponse
	require.NoError(t, json.Unmarshal(outputs[3], &fourth))
	require.NotNil(t, fourth.Choices[0].FinishReason)
	assert.Equal(t, "stop", *fourth.Choices[0].FinishReason)

	// Fifth output: [DONE]
	assert.Equal(t, []byte("[DONE]"), outputs[4])
}

func TestAnthropicStopReasonMapping(t *testing.T) {
	tests := []struct {
		anthropic string
		openai    string
	}{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"stop_sequence", "stop"},
		{"unknown_reason", "unknown_reason"},
	}

	for _, tt := range tests {
		t.Run(tt.anthropic, func(t *testing.T) {
			reason := tt.anthropic
			result := mapAnthropicStopReason(&reason)
			require.NotNil(t, result)
			assert.Equal(t, tt.openai, *result)
		})
	}

	t.Run("nil input returns nil", func(t *testing.T) {
		assert.Nil(t, mapAnthropicStopReason(nil))
	})
}

func TestAnthropicAdapterMultipleSystemMessages(t *testing.T) {
	a := NewAnthropicAdapter()

	input := ChatCompletionRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "system", Content: "Always respond in French."},
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}
	body, _ := json.Marshal(input)

	out, _, err := a.TransformRequest(body)
	require.NoError(t, err)

	var result AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &result))

	// Both system messages should be concatenated with newline.
	assert.Equal(t, "You are helpful.\nAlways respond in French.", result.System)
	// Only non-system messages remain.
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)
}

func TestAnthropicAdapterSingleSystemMessage(t *testing.T) {
	a := NewAnthropicAdapter()

	input := ChatCompletionRequest{
		Model: "claude-sonnet-4-20250514",
		Messages: []ChatMessage{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
	}
	body, _ := json.Marshal(input)

	out, _, err := a.TransformRequest(body)
	require.NoError(t, err)

	var result AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &result))

	assert.Equal(t, "You are helpful.", result.System)
}

func TestAnthropicAdapterToolUseContentBlocks(t *testing.T) {
	a := NewAnthropicAdapter()

	// Response with only tool_use blocks should map to OpenAI tool_calls.
	stopReason := "tool_use"
	anthropicResp := AnthropicResponse{
		ID:   "msg_456",
		Type: "message",
		Role: "assistant",
		Content: []AnthropicContent{
			{Type: "tool_use", ID: "toolu_123", Name: "lookup", Input: map[string]any{"city": "SF"}},
		},
		Model:      "claude-sonnet-4-20250514",
		StopReason: &stopReason,
	}
	body, _ := json.Marshal(anthropicResp)

	out, err := a.TransformResponse(body)
	require.NoError(t, err)

	var result ChatCompletionResponse
	require.NoError(t, json.Unmarshal(out, &result))
	require.Len(t, result.Choices, 1)
	require.NotNil(t, result.Choices[0].Message)
	require.Len(t, result.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "toolu_123", result.Choices[0].Message.ToolCalls[0].ID)
	assert.Equal(t, "lookup", result.Choices[0].Message.ToolCalls[0].Function.Name)
	assert.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
}

func TestAnthropicAdapterMixedContentBlocks(t *testing.T) {
	a := NewAnthropicAdapter()

	// Response with text + tool_use blocks should return text and tool calls.
	stopReason := "end_turn"
	anthropicResp := AnthropicResponse{
		ID:   "msg_789",
		Type: "message",
		Role: "assistant",
		Content: []AnthropicContent{
			{Type: "text", Text: "I'll help with that."},
			{Type: "tool_use", ID: "tool_use", Name: "lookup", Input: map[string]any{"city": "SF"}},
		},
		Model:      "claude-sonnet-4-20250514",
		StopReason: &stopReason,
	}
	body, _ := json.Marshal(anthropicResp)

	out, err := a.TransformResponse(body)
	require.NoError(t, err)

	var result ChatCompletionResponse
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "I'll help with that.", result.Choices[0].Message.Content)
	require.Len(t, result.Choices[0].Message.ToolCalls, 1)
	assert.Equal(t, "tool_use", result.Choices[0].Message.ToolCalls[0].ID)
}

func TestAnthropicAdapterEmptyContentResponse(t *testing.T) {
	a := NewAnthropicAdapter()

	// Response with empty content blocks (e.g. refusal).
	stopReason := "end_turn"
	anthropicResp := AnthropicResponse{
		ID:         "msg_empty",
		Type:       "message",
		Role:       "assistant",
		Content:    []AnthropicContent{},
		Model:      "claude-sonnet-4-20250514",
		StopReason: &stopReason,
	}
	body, _ := json.Marshal(anthropicResp)

	out, err := a.TransformResponse(body)
	require.NoError(t, err) // Empty content is fine — not an error

	var result ChatCompletionResponse
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, "", result.Choices[0].Message.Content)
}

func TestAnthropicAdapterTransformRequest_Temperature(t *testing.T) {
	a := NewAnthropicAdapter()
	temp := 0.7
	input := ChatCompletionRequest{
		Model:       "claude-sonnet-4-20250514",
		Messages:    []ChatMessage{{Role: "user", Content: "Hi"}},
		Temperature: &temp,
	}
	body, _ := json.Marshal(input)
	out, _, err := a.TransformRequest(body)
	require.NoError(t, err)

	var result AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &result))
	require.NotNil(t, result.Temperature)
	assert.Equal(t, 0.7, *result.Temperature)
}

func TestAnthropicAdapterTransformRequest_TopP(t *testing.T) {
	a := NewAnthropicAdapter()
	topP := 0.9
	input := ChatCompletionRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
		TopP:     &topP,
	}
	body, _ := json.Marshal(input)
	out, _, err := a.TransformRequest(body)
	require.NoError(t, err)

	var result AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &result))
	require.NotNil(t, result.TopP)
	assert.Equal(t, 0.9, *result.TopP)
}

func TestAnthropicAdapterTransformRequest_StopString(t *testing.T) {
	a := NewAnthropicAdapter()
	input := ChatCompletionRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
		Stop:     "END",
	}
	body, _ := json.Marshal(input)
	out, _, err := a.TransformRequest(body)
	require.NoError(t, err)

	var result AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, []string{"END"}, result.StopSequences)
}

func TestAnthropicAdapterTransformRequest_StopArray(t *testing.T) {
	a := NewAnthropicAdapter()
	// JSON unmarshal of []interface{} from Stop field.
	input := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hi"}],"stop":["END","STOP"]}`
	out, _, err := a.TransformRequest([]byte(input))
	require.NoError(t, err)

	var result AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Equal(t, []string{"END", "STOP"}, result.StopSequences)
}

func TestAnthropicAdapterTransformRequest_StopUnexpectedType(t *testing.T) {
	a := NewAnthropicAdapter()
	// Stop is a number — should be ignored gracefully.
	input := `{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"Hi"}],"stop":42}`
	out, _, err := a.TransformRequest([]byte(input))
	require.NoError(t, err)

	var result AnthropicRequest
	require.NoError(t, json.Unmarshal(out, &result))
	assert.Nil(t, result.StopSequences)
}

func TestAnthropicAdapterTransformRequest_NilOptionals(t *testing.T) {
	a := NewAnthropicAdapter()
	input := ChatCompletionRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []ChatMessage{{Role: "user", Content: "Hi"}},
	}
	body, _ := json.Marshal(input)
	out, _, err := a.TransformRequest(body)
	require.NoError(t, err)

	// Verify temperature, top_p, stop_sequences are omitted from JSON.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &raw))
	assert.Nil(t, raw["temperature"])
	assert.Nil(t, raw["top_p"])
	assert.Nil(t, raw["stop_sequences"])
}

func TestAnthropicAdapterTransformStreamEvent_MalformedMessageStart(t *testing.T) {
	a := NewAnthropicAdapter()
	out, err := a.TransformStreamEvent("message_start", []byte(`not json`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse message_start")
	assert.Nil(t, out)
}

func TestAnthropicAdapterTransformStreamEvent_ToolCallDeltas(t *testing.T) {
	a := NewAnthropicAdapter()

	_, err := a.TransformStreamEvent("message_start", []byte(`{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4-20250514","role":"assistant"}}`))
	require.NoError(t, err)

	startChunk, err := a.TransformStreamEvent("content_block_start", []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"lookup_weather"}}`))
	require.NoError(t, err)
	require.NotNil(t, startChunk)

	deltaChunk, err := a.TransformStreamEvent("content_block_delta", []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"SF\"}"}}`))
	require.NoError(t, err)
	require.NotNil(t, deltaChunk)

	var startResp ChatCompletionResponse
	require.NoError(t, json.Unmarshal(startChunk, &startResp))
	require.Len(t, startResp.Choices[0].Delta.ToolCalls, 1)
	assert.Equal(t, "toolu_1", startResp.Choices[0].Delta.ToolCalls[0].ID)
	assert.Equal(t, "lookup_weather", startResp.Choices[0].Delta.ToolCalls[0].Function.Name)

	var deltaResp ChatCompletionResponse
	require.NoError(t, json.Unmarshal(deltaChunk, &deltaResp))
	require.Len(t, deltaResp.Choices[0].Delta.ToolCalls, 1)
	assert.Equal(t, "{\"city\":\"SF\"}", deltaResp.Choices[0].Delta.ToolCalls[0].Function.Arguments)
}

// Verify interface compliance at compile time.
var (
	_ ProviderAdapter = (*OpenAIAdapter)(nil)
	_ ProviderAdapter = (*AnthropicAdapter)(nil)
)
