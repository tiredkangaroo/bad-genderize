package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GenderClassifier classifies a name into a gender + confidence.
type GenderClassifier interface {
	Classify(ctx context.Context, name string) (GenderResult, error)
}

// LLMClient implements GenderClassifier against an OpenAI-compatible
// chat/completions endpoint (e.g. the user's LLM proxy at /v1/chat/completions).
type LLMClient struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

func NewLLMClient(baseURL, model, apiKey string, timeout time.Duration) *LLMClient {
	return &LLMClient{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

const systemPrompt = `You are a name-to-gender classifier. Given a person's name, respond with ONLY a JSON object that has exactly two keys: "gender" (either "male" or "female") and "confidence" (a number from 0 to 1 indicating how confident you are). Output no other text, explanation, or markdown.`

func (c *LLMClient) Classify(ctx context.Context, name string) (GenderResult, error) {
	reqBody := chatCompletionRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: "Name: " + name},
		},
		Temperature: 0,
		MaxTokens:   64,
		Stream:      false,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return GenderResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return GenderResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return GenderResult{}, fmt.Errorf("calling LLM: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return GenderResult{}, fmt.Errorf("reading LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var parsed chatCompletionResponse
		if json.Unmarshal(body, &parsed) == nil && parsed.Error != nil {
			return GenderResult{}, fmt.Errorf("LLM error (%d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return GenderResult{}, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, stringsTrim(body))
	}

	var parsed chatCompletionResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return GenderResult{}, fmt.Errorf("decoding LLM response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return GenderResult{}, fmt.Errorf("LLM returned no choices")
	}

	return parseGenderResponse(parsed.Choices[0].Message.Content)
}

func stringsTrim(b []byte) string {
	const max = 512
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
