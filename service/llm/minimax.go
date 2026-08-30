package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type MiniMaxClient struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
}

type miniMaxRequest struct {
	Model     string           `json:"model"`
	System    string           `json:"system,omitempty"`
	Messages  []miniMaxMessage `json:"messages"`
	Thinking  miniMaxThinking  `json:"thinking"`
	MaxTokens int              `json:"max_tokens"`
	Stream    bool             `json:"stream"`
}

type miniMaxMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type miniMaxThinking struct {
	Type string `json:"type"`
}

type miniMaxResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewMiniMaxClient(baseURL, apiKey, model string, timeout time.Duration) (*MiniMaxClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("MiniMax base URL is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("MiniMax API key is required")
	}
	if model == "" {
		model = "MiniMax-M3"
	}
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	return &MiniMaxClient{
		endpoint: baseURL + "/v1/messages", apiKey: apiKey, model: model,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (c *MiniMaxClient) Complete(ctx context.Context, request Request) (string, error) {
	maxTokens := request.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 180
	}
	payload := miniMaxRequest{
		Model: c.model, System: request.System,
		Messages:  []miniMaxMessage{{Role: "user", Content: request.Prompt}},
		Thinking:  miniMaxThinking{Type: "disabled"},
		MaxTokens: maxTokens, Stream: false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("MiniMax returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var completion miniMaxResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return "", fmt.Errorf("decode MiniMax response: %w", err)
	}
	if completion.Error != nil && completion.Error.Message != "" {
		return "", errors.New(completion.Error.Message)
	}
	for _, block := range completion.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", errors.New("MiniMax response contains no text")
}
