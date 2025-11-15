package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client is a minimal Ollama-compatible LLM client.
type Client struct {
	url    string
	model  string
	hc     *http.Client
	logger func(format string, v ...any)
}

// NewClient creates a new client. If httpClient is nil, a default with timeout is used.
func NewClient(url, model string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{
		url:   url,
		model: model,
		hc:    httpClient,
		logger: func(format string, v ...any) {
			// noop default logger — you can inject one if you want logging.
			fmt.Fprintf(io.Discard, format, v...)
		},
	}
}

// SetLogger allows injecting a simple printf-like logger for debugging.
func (c *Client) SetLogger(l func(format string, v ...any)) {
	if l == nil {
		return
	}
	c.logger = l
}

// SummarizeArticleText returns a single clean summary string for the provided title + content.
// It sends a non-streaming request to the LLM (stream=false) and extracts the returned text.
func (c *Client) SummarizeArticleText(ctx context.Context, title, content string) (string, error) {
	prompt := buildPrompt(title, content)

	// Build request body tailored for Ollama (model + prompt + max_tokens + stream:false)
	body := map[string]any{
		"model":      c.model,
		"prompt":     prompt,
		"max_tokens": 256,
		"stream":     false,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("llm marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("llm new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.hc.Do(req)
	lat := time.Since(start)
	c.logger("llm request url=%s model=%s status_err=%v latency=%s", c.url, c.model, err, lat)
	if err != nil {
		return "", fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// include body for debugging
		return "", fmt.Errorf("llm request failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	// Try to parse common shapes:
	// 1) {"response": "text..."}  (Ollama streaming final object might use "response")
	// 2) {"text": "text..."}      (some APIs)
	// 3) {"choices":[{"text":"..."}]} (openai-like)
	// 4) fallback: return entire body as string
	var parsed any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// not JSON? return raw body
		return string(respBody), nil
	}

	// parsed should be object/map
	if m, ok := parsed.(map[string]any); ok {
		// 1) response
		if v, ok := m["response"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, nil
			}
		}
		// 2) text
		if v, ok := m["text"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s, nil
			}
		}
		// 3) choices -> first -> text
		if v, ok := m["choices"]; ok {
			if arr, ok := v.([]any); ok && len(arr) > 0 {
				if first, ok := arr[0].(map[string]any); ok {
					if t, ok := first["text"]; ok {
						if s, ok := t.(string); ok && s != "" {
							return s, nil
						}
					}
					// some choices use "message": {"content": "..."}
					if msg, ok := first["message"]; ok {
						if m2, ok := msg.(map[string]any); ok {
							if content, ok := m2["content"]; ok {
								if s, ok := content.(string); ok && s != "" {
									return s, nil
								}
							}
						}
					}
				}
			}
		}
		// 4) other fields: sometimes "results" or "output"
		if v, ok := m["results"]; ok {
			// results might be array of objects with "response"/"text"
			if arr, ok := v.([]any); ok && len(arr) > 0 {
				buf := ""
				for _, it := range arr {
					if oo, ok := it.(map[string]any); ok {
						if r, ok := oo["response"]; ok {
							if s, ok := r.(string); ok {
								buf += s
							}
						} else if t, ok := oo["text"]; ok {
							if s, ok := t.(string); ok {
								buf += s
							}
						}
					}
				}
				if buf != "" {
					return buf, nil
				}
			}
		}
	}

	// fallback: return raw body as string (trim)
	return string(bytes.TrimSpace(respBody)), nil
}

// buildPrompt combines title + content into a summarization prompt.
// Adjust this as you like for style/length.
func buildPrompt(title, content string) string {
	// concise instruction + content
	return fmt.Sprintf("Summarize the following news article in 2-3 sentences. Title: %s\n\nArticle: %s\n\nSummary:", title, content)
}

// NewClientFromEnv convenience to create client based on env vars used in docker-compose.
func NewClientFromEnv() *Client {
	url := os.Getenv("LLM_URL")
	model := os.Getenv("LLM_MODEL")
	// if url is empty default to localhost ollama endpoint
	if url == "" {
		url = "http://host.docker.internal:11434/api/generate"
	}
	if model == "" {
		model = "smollm2:135m"
	}
	return NewClient(url, model, nil)
}