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
)

const maxJSONBytes = 4 << 20

type (
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	Request struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
		Temperature *float64  `json:"temperature,omitempty"`
	}

	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}

	Response struct {
		Choices []struct {
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}

	Client struct {
		HTTP    *http.Client
		BaseURL string
		APIKey  string
	}
)

func (c *Client) Complete(ctx context.Context, in Request) (Response, error) {
	if c.HTTP == nil {
		return Response{}, errors.New("missing http client")
	}
	if in.Model == "" || len(in.Messages) == 0 {
		return Response{}, errors.New("model and message required")
	}
	body, err := json.Marshal(in)
	if err != nil {
		return Response{}, err
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("call model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("model API status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONBytes+1))
	if err != nil {
		return Response{}, fmt.Errorf("read model response: %w", err)
	}
	if len(data) > maxJSONBytes {
		return Response{}, fmt.Errorf("model response too large")
	}
	var out Response
	if err := json.Unmarshal(data, &out); err != nil {
		return Response{}, fmt.Errorf("decode model response: %w", err)
	}
	if len(out.Choices) == 0 {
		return Response{}, errors.New("model response has no choices")
	}
	if out.Choices[0].FinishReason == "length" {
		return Response{}, errors.New("model output was truncated")
	}
	if strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return Response{}, errors.New("model returned empty text (or non-text content)")
	}
	if out.Usage.PromptTokens < 0 || out.Usage.CompletionTokens < 0 {
		return Response{}, errors.New("negative usage")
	}
	return out, nil
}

type Rates struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

func EstimateCostUSD(usage Usage, rates Rates) (float64, error) {
	if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || rates.InputPerMillion < 0 {
		return 0, errors.New("negative tokens or rates")
	}
	return (float64(usage.PromptTokens)*rates.InputPerMillion + float64(usage.CompletionTokens)*rates.OutputPerMillion) / 1_000_000, nil
}
