package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCacheKey(t *testing.T) {
	cases := map[string]string{
		"Hello World": "gender:hello world",
		"  Alice  ":   "gender:alice",
		"BOB":         "gender:bob",
		"Ann Marie":   "gender:ann marie",
	}
	for in, want := range cases {
		if got := cacheKey(in); got != want {
			t.Errorf("cacheKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeGender(t *testing.T) {
	good := map[string]string{
		"male": "male", "Male": "male", "MALE": "male",
		"female": "female", "Female": "female", "F": "female",
		" man ": "male", "woman": "female",
	}
	for in, want := range good {
		got, err := normalizeGender(in)
		if err != nil || got != want {
			t.Errorf("normalizeGender(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := normalizeGender("alien"); err == nil {
		t.Error("normalizeGender(alien) should error")
	}
}

func TestParseGenderResponse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    GenderResult
		wantErr bool
	}{
		{"plain", `{"gender":"male","confidence":0.31}`, GenderResult{"male", 0.31}, false},
		{"fenced", "```json\n{\"gender\":\"female\",\"confidence\":0.9}\n```", GenderResult{"female", 0.9}, false},
		{"prose", `Sure! Here you go: {"gender":"male","confidence":0.5} hope that helps`, GenderResult{"male", 0.5}, false},
		{"string confidence", `{"gender":"male","confidence":"0.42"}`, GenderResult{"male", 0.42}, false},
		{"clamp high", `{"gender":"female","confidence":1.7}`, GenderResult{"female", 1.0}, false},
		{"clamp low", `{"gender":"female","confidence":-0.2}`, GenderResult{"female", 0.0}, false},
		{"bad gender", `{"gender":"unknown","confidence":0.5}`, GenderResult{}, true},
		{"no json", `I think it's a male`, GenderResult{}, true},
	}
	for _, tc := range tests {
		got, err := parseGenderResponse(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error, got %+v", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}
		if got.Gender != tc.want.Gender || got.Confidence != tc.want.Confidence {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

type fakeLLM struct {
	res   GenderResult
	err   error
	calls []string
}

func (f *fakeLLM) Classify(_ context.Context, name string) (GenderResult, error) {
	f.calls = append(f.calls, name)
	if f.err != nil {
		return GenderResult{}, f.err
	}
	return f.res, nil
}

type fakeCache struct {
	m map[string]string
}

func newFakeCache() *fakeCache { return &fakeCache{m: map[string]string{}} }

func (f *fakeCache) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := f.m[key]
	return v, ok, nil
}

func (f *fakeCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	f.m[key] = value
	return nil
}

func doRequest(s *genderServer, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestHandlerCacheMissThenHit(t *testing.T) {
	llm := &fakeLLM{res: GenderResult{"male", 0.31}}
	cache := newFakeCache()
	s := newGenderServer(llm, cache, 0)

	rec := doRequest(s, http.MethodGet, "/api/v1/gender?name=Hello%20World")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got GenderResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if got != (GenderResult{"male", 0.31}) {
		t.Errorf("got %+v", got)
	}
	if len(llm.calls) != 1 {
		t.Errorf("expected 1 LLM call, got %d", len(llm.calls))
	}

	// Second request for the same name (different casing) must be cached.
	rec = doRequest(s, http.MethodGet, "/api/v1/gender?name=hello%20world")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if len(llm.calls) != 1 {
		t.Errorf("expected still 1 LLM call, got %d", len(llm.calls))
	}
}

func TestHandlerMissingName(t *testing.T) {
	s := newGenderServer(&fakeLLM{}, newFakeCache(), 0)
	rec := doRequest(s, http.MethodGet, "/api/v1/gender")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerWrongMethod(t *testing.T) {
	s := newGenderServer(&fakeLLM{}, newFakeCache(), 0)
	rec := doRequest(s, http.MethodPost, "/api/v1/gender?name=Alice")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

func TestHandlerLLMError(t *testing.T) {
	s := newGenderServer(&fakeLLM{err: errStub{}}, newFakeCache(), 0)
	rec := doRequest(s, http.MethodGet, "/api/v1/gender?name=Alice")
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}

type errStub struct{}

func (errStub) Error() string { return "boom" }
