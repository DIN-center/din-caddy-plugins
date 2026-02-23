package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	libai "github.com/DIN-center/din-caddy-plugins/lib/ai"
	"go.uber.org/zap"
)

// streamResult contains the outcome of a streaming attempt.
type streamResult struct {
	// firstChunkData is the first valid SSE data payload (already transformed to OpenAI format).
	firstChunkData []byte
	// resp is the raw HTTP response (caller must close Body after streaming completes).
	resp *http.Response
	// reader is the buffered reader wrapping resp.Body. Must be used for all subsequent reads
	// to avoid losing data buffered during the first-chunk read.
	reader *bufio.Reader
	// ttft is the time-to-first-token measurement.
	ttft time.Duration
	// adapter is the provider adapter used for this stream.
	adapter libai.ProviderAdapter
}

type streamAttemptError struct {
	statusCode int
	retryAfter time.Duration
	err        error
}

func (e *streamAttemptError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("stream attempt failed with HTTP %d", e.statusCode)
}

func (e *streamAttemptError) Unwrap() error {
	return e.err
}

// attemptStream tries to open a streaming connection to a provider and validate the first chunk.
// Returns a streamResult on success, or an error if the provider fails.
func attemptStream(
	ctx context.Context,
	provider *AIProvider,
	adapter libai.ProviderAdapter,
	body []byte,
	client libai.IStreamingHTTPClient,
	logger *zap.Logger,
) (*streamResult, error) {
	// Transform the request body for this provider.
	transformedBody, extraHeaders, err := adapter.TransformRequest(body)
	if err != nil {
		return nil, fmt.Errorf("transform request: %w", err)
	}

	// Build headers: provider static headers + any adapter-added headers.
	headers := make(map[string]string)
	for k, v := range provider.Headers {
		headers[k] = v
	}
	for k, v := range extraHeaders {
		headers[k] = v
	}
	headers["Content-Type"] = "application/json"
	headers["Accept"] = "text/event-stream"

	startTime := time.Now()

	resp, err := client.PostStream(ctx, provider.HttpUrl, headers, transformedBody)
	if err != nil {
		return nil, fmt.Errorf("post stream: %w", err)
	}

	// Check HTTP status before reading the body.
	if resp.StatusCode != http.StatusOK {
		retryAfter := time.Duration(0)
		if isRetryableStatus(resp.StatusCode) {
			if d, parseErr := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now(), defaultRetryAfterCap); parseErr == nil {
				retryAfter = d
			}
		}
		resp.Body.Close()
		return nil, &streamAttemptError{
			statusCode: resp.StatusCode,
			retryAfter: retryAfter,
			err:        fmt.Errorf("provider returned HTTP %d", resp.StatusCode),
		}
	}

	// Create a single buffered reader that persists across reads.
	bufReader := bufio.NewReaderSize(resp.Body, 64*1024)

	// Read the first SSE event and validate it's not an error.
	eventType, data, err := readNextSSEEvent(bufReader)
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("read first event: %w", err)
	}

	ttft := time.Since(startTime)

	// Check if the first chunk is an error.
	if adapter.IsErrorChunk(eventType, data) {
		resp.Body.Close()
		return nil, fmt.Errorf("provider returned error in first chunk: %s", string(data))
	}

	// Transform the first chunk to OpenAI format.
	transformed, err := adapter.TransformStreamEvent(eventType, data)
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("transform first event: %w", err)
	}

	return &streamResult{
		firstChunkData: transformed,
		resp:           resp,
		reader:         bufReader,
		ttft:           ttft,
		adapter:        adapter,
	}, nil
}

// streamToClient reads remaining SSE events from the provider response and writes
// them to the client as OpenAI-format SSE events. The first chunk has already been
// validated and should be flushed separately before calling this.
//
// bufReader must be the same buffered reader created in attemptStream, to avoid
// losing data that was buffered during the first-chunk read.
//
// The context is monitored for cancellation (e.g., client disconnect). When cancelled,
// the response body is closed to unblock the scanner.
func streamToClient(
	ctx context.Context,
	w http.ResponseWriter,
	respBody io.ReadCloser,
	bufReader *bufio.Reader,
	adapter libai.ProviderAdapter,
	logger *zap.Logger,
) (*libai.UsageInfo, error) {
	var closeOnce sync.Once
	closeBody := func() {
		closeOnce.Do(func() {
			_ = respBody.Close()
		})
	}
	defer closeBody()

	flusher, _ := w.(http.Flusher)

	scanner := bufio.NewScanner(bufReader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	// Monitor context cancellation to unblock the scanner on client disconnect.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			closeBody()
		case <-done:
		}
	}()

	var currentEventType string
	var usage *libai.UsageInfo

	for scanner.Scan() {
		line := scanner.Text()

		// Parse SSE protocol lines.
		if strings.HasPrefix(line, "event:") {
			currentEventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if strings.HasPrefix(line, "data:") {
			rawData := strings.TrimPrefix(line, "data:")
			rawData = strings.TrimSpace(rawData)

			if rawData == "[DONE]" {
				writeSSE(w, []byte("[DONE]"))
				if flusher != nil {
					flusher.Flush()
				}
				break
			}

			transformed, err := adapter.TransformStreamEvent(currentEventType, []byte(rawData))
			if err != nil {
				logger.Warn("failed to transform stream event",
					zap.String("event_type", currentEventType),
					zap.Error(err))
				errorChunk, marshalErr := json.Marshal(libai.ErrorResponse{
					Error: &libai.ErrorDetail{
						Message: "stream transform error",
						Type:    "upstream_error",
					},
				})
				if marshalErr == nil {
					writeSSE(w, errorChunk)
				}
				writeSSE(w, []byte("[DONE]"))
				if flusher != nil {
					flusher.Flush()
				}
				return usage, fmt.Errorf("transform stream event: %w", err)
			}

			if transformed == nil {
				// Adapter says skip this event (metadata event).
				currentEventType = ""
				continue
			}

			// Check if this is the [DONE] marker from the adapter.
			if bytes.Equal(transformed, []byte("[DONE]")) {
				writeSSE(w, []byte("[DONE]"))
				if flusher != nil {
					flusher.Flush()
				}
				break
			}

			// Try to extract usage info from the chunk.
			usage = tryExtractUsage(transformed, usage)

			writeSSE(w, transformed)
			if flusher != nil {
				flusher.Flush()
			}

			currentEventType = ""
			continue
		}

		// Empty lines are event delimiters in SSE — just skip.
	}

	return usage, nil
}

// readNextSSEEvent reads the next complete SSE event from a buffered reader.
// Returns the event type (empty for OpenAI) and the data payload.
func readNextSSEEvent(r *bufio.Reader) (string, []byte, error) {
	var eventType string
	var dataLines []string

	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", nil, err
		}

		line = strings.TrimRight(line, "\r\n")

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			dataLines = append(dataLines, data)
		} else if line == "" && len(dataLines) > 0 {
			// Empty line = end of event.
			return eventType, []byte(strings.Join(dataLines, "\n")), nil
		}
		// Skip comment lines (starting with :) and other lines.

		if err == io.EOF {
			break
		}
	}

	// If we collected data but hit EOF, still return it.
	if len(dataLines) > 0 {
		return eventType, []byte(strings.Join(dataLines, "\n")), nil
	}

	return "", nil, io.EOF
}

// writeSSE writes an SSE data line to the response writer.
func writeSSE(w http.ResponseWriter, data []byte) {
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// tryExtractUsage tries to extract usage information from an OpenAI-format chunk.
func tryExtractUsage(data []byte, existing *libai.UsageInfo) *libai.UsageInfo {
	var chunk struct {
		Usage *libai.UsageInfo `json:"usage,omitempty"`
	}
	if err := json.Unmarshal(data, &chunk); err == nil && chunk.Usage != nil {
		return chunk.Usage
	}
	return existing
}
