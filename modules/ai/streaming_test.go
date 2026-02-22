package ai

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock HTTP Client for streaming tests ---

type mockStreamingClient struct {
	handler func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error)
}

func (m *mockStreamingClient) Post(ctx context.Context, url string, headers map[string]string, payload []byte) ([]byte, int, error) {
	resp, err := m.PostStream(ctx, url, headers, payload)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (m *mockStreamingClient) PostStream(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
	return m.handler(ctx, url, headers, payload)
}

// --- SSE Event Helpers ---

func sseEvent(eventType, data string) string {
	var sb strings.Builder
	if eventType != "" {
		sb.WriteString("event: " + eventType + "\n")
	}
	sb.WriteString("data: " + data + "\n\n")
	return sb.String()
}

func makeSSEResponse(statusCode int, events string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(events)),
	}
}

func newBufReader(s string) *bufio.Reader {
	return bufio.NewReader(strings.NewReader(s))
}

// --- Tests ---

func TestReadNextSSEEvent(t *testing.T) {
	t.Run("simple data event", func(t *testing.T) {
		input := "data: {\"hello\":\"world\"}\n\n"
		eventType, data, err := readNextSSEEvent(newBufReader(input))
		require.NoError(t, err)
		assert.Empty(t, eventType)
		assert.Equal(t, `{"hello":"world"}`, string(data))
	})

	t.Run("event with type", func(t *testing.T) {
		input := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n"
		eventType, data, err := readNextSSEEvent(newBufReader(input))
		require.NoError(t, err)
		assert.Equal(t, "message_start", eventType)
		assert.Equal(t, `{"type":"message_start"}`, string(data))
	})

	t.Run("EOF returns data collected so far", func(t *testing.T) {
		input := "data: {\"partial\":true}\n"
		eventType, data, err := readNextSSEEvent(newBufReader(input))
		require.NoError(t, err)
		assert.Empty(t, eventType)
		assert.Equal(t, `{"partial":true}`, string(data))
	})

	t.Run("empty reader returns EOF", func(t *testing.T) {
		_, _, err := readNextSSEEvent(newBufReader(""))
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("comment lines are skipped", func(t *testing.T) {
		input := ": this is a comment\ndata: {\"content\":\"hi\"}\n\n"
		eventType, data, err := readNextSSEEvent(newBufReader(input))
		require.NoError(t, err)
		assert.Empty(t, eventType)
		assert.Equal(t, `{"content":"hi"}`, string(data))
	})
}

func TestAttemptStream_Success(t *testing.T) {
	adapter := libai.NewOpenAIAdapter()
	logger := zap.NewNop()

	sseData := sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"role":"assistant"}}]}`)
	sseData += sseEvent("", `{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[{"delta":{"content":"Hello"}}]}`)

	client := &mockStreamingClient{
		handler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			return makeSSEResponse(200, sseData), nil
		},
	}

	provider := newTestProvider("test-openai", Healthy)
	provider.httpClient = client

	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	result, err := attemptStream(context.Background(), provider, adapter, body, client, logger)
	require.NoError(t, err)
	require.NotNil(t, result)
	defer result.resp.Body.Close()

	assert.NotNil(t, result.firstChunkData)
	assert.NotNil(t, result.reader)
	assert.Greater(t, result.ttft, time.Duration(0))
}

func TestAttemptStream_NonOKStatus(t *testing.T) {
	adapter := libai.NewOpenAIAdapter()
	logger := zap.NewNop()

	client := &mockStreamingClient{
		handler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			return makeSSEResponse(429, "rate limited"), nil
		},
	}

	provider := newTestProvider("test-openai", Healthy)
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	result, err := attemptStream(context.Background(), provider, adapter, body, client, logger)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "HTTP 429")
}

func TestAttemptStream_ErrorInFirstChunk(t *testing.T) {
	adapter := libai.NewOpenAIAdapter()
	logger := zap.NewNop()

	sseData := sseEvent("", `{"error":{"message":"rate limited","type":"tokens"}}`)

	client := &mockStreamingClient{
		handler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			return makeSSEResponse(200, sseData), nil
		},
	}

	provider := newTestProvider("test-openai", Healthy)
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	result, err := attemptStream(context.Background(), provider, adapter, body, client, logger)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "error in first chunk")
}

func TestAttemptStream_AnthropicFormat(t *testing.T) {
	adapter := libai.NewAnthropicAdapter()
	logger := zap.NewNop()

	sseData := sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_1","model":"claude-sonnet-4-20250514","role":"assistant"}}`)
	sseData += sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)

	client := &mockStreamingClient{
		handler: func(ctx context.Context, url string, headers map[string]string, payload []byte) (*http.Response, error) {
			return makeSSEResponse(200, sseData), nil
		},
	}

	provider := newTestProvider("test-anthropic", Healthy)
	provider.AdapterType = AdapterAnthropic
	body := []byte(`{"model":"claude-sonnet-4-20250514","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	result, err := attemptStream(context.Background(), provider, adapter, body, client, logger)
	require.NoError(t, err)
	require.NotNil(t, result)
	defer result.resp.Body.Close()

	// First chunk should be the role delta from message_start.
	assert.NotNil(t, result.firstChunkData)
}

func TestStreamToClient_OpenAI(t *testing.T) {
	adapter := libai.NewOpenAIAdapter()
	logger := zap.NewNop()

	events := sseEvent("", `{"id":"chatcmpl-1","choices":[{"delta":{"content":"Hello"}}]}`)
	events += sseEvent("", `{"id":"chatcmpl-1","choices":[{"delta":{"content":" world"}}]}`)
	events += "data: [DONE]\n\n"

	body := io.NopCloser(strings.NewReader(events))
	bufReader := bufio.NewReader(strings.NewReader(events))
	// We need body and bufReader to be from the same underlying reader for real usage,
	// but for testing we just need to verify the output.
	// Create a proper setup: body wraps the reader that bufReader reads from.
	reader := strings.NewReader(events)
	body = io.NopCloser(reader)
	bufReader = bufio.NewReader(reader)

	recorder := httptest.NewRecorder()
	usage := streamToClient(context.Background(), recorder, body, bufReader, adapter, logger)

	result := recorder.Body.String()
	assert.Contains(t, result, `"content":"Hello"`)
	assert.Contains(t, result, `"content":" world"`)
	assert.Contains(t, result, "[DONE]")
	assert.Nil(t, usage) // No usage in these chunks
}

func TestStreamToClient_Anthropic(t *testing.T) {
	adapter := libai.NewAnthropicAdapter()
	logger := zap.NewNop()

	// Simulate remaining Anthropic events after first chunk was already consumed.
	events := sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)
	events += sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`)
	events += sseEvent("content_block_stop", `{"type":"content_block_stop","index":0}`)
	events += sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)
	events += sseEvent("message_stop", `{"type":"message_stop"}`)

	reader := strings.NewReader(events)
	body := io.NopCloser(reader)
	bufReader := bufio.NewReader(reader)

	recorder := httptest.NewRecorder()
	usage := streamToClient(context.Background(), recorder, body, bufReader, adapter, logger)

	result := recorder.Body.String()
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, " world")
	assert.Contains(t, result, "[DONE]")
	assert.Nil(t, usage)
}

func TestStreamToClient_WithUsage(t *testing.T) {
	adapter := libai.NewOpenAIAdapter()
	logger := zap.NewNop()

	events := sseEvent("", `{"id":"chatcmpl-1","choices":[{"delta":{"content":"Hi"}}]}`)
	events += sseEvent("", `{"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	events += "data: [DONE]\n\n"

	reader := strings.NewReader(events)
	body := io.NopCloser(reader)
	bufReader := bufio.NewReader(reader)

	recorder := httptest.NewRecorder()
	usage := streamToClient(context.Background(), recorder, body, bufReader, adapter, logger)

	require.NotNil(t, usage)
	assert.Equal(t, 10, usage.PromptTokens)
	assert.Equal(t, 5, usage.CompletionTokens)
	assert.Equal(t, 15, usage.TotalTokens)
}

func TestWriteSSE(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeSSE(recorder, []byte(`{"test":true}`))

	assert.Equal(t, "data: {\"test\":true}\n\n", recorder.Body.String())
}

func TestWriteSSE_Done(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeSSE(recorder, []byte("[DONE]"))

	assert.Equal(t, "data: [DONE]\n\n", recorder.Body.String())
}

func TestTryExtractUsage(t *testing.T) {
	t.Run("extracts usage", func(t *testing.T) {
		data := []byte(`{"id":"chatcmpl-1","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
		usage := tryExtractUsage(data, nil)
		require.NotNil(t, usage)
		assert.Equal(t, 10, usage.PromptTokens)
		assert.Equal(t, 5, usage.CompletionTokens)
	})

	t.Run("returns existing when no usage in chunk", func(t *testing.T) {
		data := []byte(`{"id":"chatcmpl-1","choices":[]}`)
		existing := &libai.UsageInfo{PromptTokens: 10}
		usage := tryExtractUsage(data, existing)
		assert.Equal(t, existing, usage)
	})

	t.Run("returns nil for invalid json", func(t *testing.T) {
		data := []byte(`not json`)
		usage := tryExtractUsage(data, nil)
		assert.Nil(t, usage)
	})
}

func TestStreamToClient_ClientDisconnect(t *testing.T) {
	adapter := libai.NewOpenAIAdapter()
	logger := zap.NewNop()

	// Create a slow streaming server that sends chunks with delays.
	pr, pw := io.Pipe()

	go func() {
		// Send first chunk.
		pw.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		// Simulate a slow stream — block for a while before next chunk.
		time.Sleep(5 * time.Second)
		pw.Write([]byte("data: [DONE]\n\n"))
		pw.Close()
	}()

	// Use the PipeReader directly as the body — it implements io.ReadCloser,
	// so when the cancellation goroutine calls Close(), it actually aborts the read.
	// (io.NopCloser would make Close() a no-op and the test would hang.)
	bufReader := bufio.NewReader(pr)
	recorder := httptest.NewRecorder()

	// Create a cancellable context to simulate client disconnect.
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		streamToClient(ctx, recorder, pr, bufReader, adapter, logger)
		close(done)
	}()

	// Wait briefly to let it start reading, then cancel (simulate disconnect).
	time.Sleep(100 * time.Millisecond)
	cancel()

	// streamToClient should return promptly after cancellation (not wait 5s for the next chunk).
	select {
	case <-done:
		// Success — goroutine exited promptly.
	case <-time.After(2 * time.Second):
		t.Fatal("streamToClient did not exit promptly after context cancellation")
	}
}

func TestAttemptStream_ContextCancelled(t *testing.T) {
	adapter := libai.NewOpenAIAdapter()
	logger := zap.NewNop()

	// Create a slow server that delays before responding.
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until request context is cancelled.
		<-r.Context().Done()
	}))
	defer slowServer.Close()

	client := newDefaultStreamingClient()
	provider, _ := NewAIProvider("slow-provider", slowServer.URL)
	provider.ModelID = "test-model"
	provider.AdapterType = AdapterOpenAI
	provider.httpClient = client

	body := []byte(`{"model":"test","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	// Cancel immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := attemptStream(ctx, provider, adapter, body, client, logger)
	assert.Error(t, err)
	assert.Nil(t, result)
}
