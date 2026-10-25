package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GenderResult is the classified gender plus a confidence in [0,1].
type GenderResult struct {
	Gender     string  `json:"gender"`
	Confidence float64 `json:"confidence"`
}

// cacheKey returns a stable, normalized key for a name so that casing and
// surrounding whitespace never cause a second LLM call.
func cacheKey(name string) string {
	return "gender:" + strings.ToLower(strings.TrimSpace(name))
}

// normalizeGender coerces the LLM's answer into "male" or "female".
func normalizeGender(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "male", "m", "man", "boy":
		return "male", nil
	case "female", "f", "woman", "girl":
		return "female", nil
	default:
		return "", fmt.Errorf("unrecognized gender %q", s)
	}
}

// confidenceValue accepts a JSON number or a numeric JSON string, since some
// models return "0.31" rather than 0.31.
type confidenceValue float64

func (c *confidenceValue) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		*c = 0
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*c = confidenceValue(f)
	return nil
}

// parseGenderResponse extracts the JSON object the model returned, validates
// the gender, and clamps the confidence into [0,1].
func parseGenderResponse(content string) (GenderResult, error) {
	obj, err := extractJSONObject(content)
	if err != nil {
		return GenderResult{}, err
	}
	var raw struct {
		Gender     string          `json:"gender"`
		Confidence confidenceValue `json:"confidence"`
	}
	if err := json.Unmarshal(obj, &raw); err != nil {
		return GenderResult{}, fmt.Errorf("invalid JSON from model: %w", err)
	}
	gender, err := normalizeGender(raw.Gender)
	if err != nil {
		return GenderResult{}, err
	}
	conf := float64(raw.Confidence)
	if conf < 0 {
		conf = 0
	}
	if conf > 1 {
		conf = 1
	}
	return GenderResult{Gender: gender, Confidence: conf}, nil
}

// extractJSONObject returns the first complete JSON object found in s, so a
// model that wraps its answer in prose or markdown fences still parses.
func extractJSONObject(s string) ([]byte, error) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return nil, fmt.Errorf("no JSON object in model output")
	}
	dec := json.NewDecoder(strings.NewReader(s[start:]))
	var obj json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("could not decode model output: %w", err)
	}
	return obj, nil
}
