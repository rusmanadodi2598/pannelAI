// Package gateway carries the HTTP transport for the builder's chat completions.
//
// @file      internal/gateway/gateway.go
// @for       A thin OpenAI-shaped chat-completions client behind the gateway data plane.
// @uses      bytes, context, encoding/json, fmt, net/http, strings, time.
// @reason    The builder must stay domain code, so the network edge sits here and is swapped for a fake in tests.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-23
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-bot/internal/masterprompt"
)

// Client POSTs one completion per attempt to the gateway data plane.
type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

// New wires a client with the base URL, bearer key, and one-attempt timeout.
func New(baseURL, key string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    &http.Client{Timeout: timeout},
	}
}

// Complete implements masterprompt.Gateway.
func (c *Client) Complete(ctx context.Context, req masterprompt.ChatRequest) (masterprompt.ChatResult, error) {
	payload, err := json.Marshal(completionRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Messages: []message{
			{Role: "system", Content: req.System},
			{Role: "user", Content: req.User},
		},
	})
	if err != nil {
		return masterprompt.ChatResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return masterprompt.ChatResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return masterprompt.ChatResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return masterprompt.ChatResult{}, fmt.Errorf("gateway completed with status %d", resp.StatusCode)
	}

	var out completionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return masterprompt.ChatResult{}, fmt.Errorf("decoding the gateway response: %w", err)
	}
	if len(out.Choices) == 0 {
		return masterprompt.ChatResult{}, fmt.Errorf("gateway returned no choices")
	}

	usage := masterprompt.Usage{
		PromptTokens:     out.Usage.PromptTokens,
		CompletionTokens: out.Usage.CompletionTokens,
		TotalTokens:      out.Usage.TotalTokens,
	}
	return masterprompt.ChatResult{ModelID: out.Model, Content: out.Choices[0].Message.Content, Usage: usage}, nil
}

type completionRequest struct {
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Messages    []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type completionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
