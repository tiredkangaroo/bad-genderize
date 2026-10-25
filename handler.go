package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type genderServer struct {
	llm   GenderClassifier
	cache Cache
	ttl   time.Duration
}

func newGenderServer(llm GenderClassifier, cache Cache, ttl time.Duration) *genderServer {
	return &genderServer{llm: llm, cache: cache, ttl: ttl}
}

func (s *genderServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/gender" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, `missing required query parameter "name"`)
		return
	}

	ctx := r.Context()
	key := cacheKey(name)

	// 1. Serve from cache if we've already classified this name.
	if cached, ok, err := s.cache.Get(ctx, key); err == nil && ok {
		var res GenderResult
		if json.Unmarshal([]byte(cached), &res) == nil {
			writeJSON(w, http.StatusOK, res)
			return
		}
		// Corrupt cache entry: fall through and re-classify.
	} else if err != nil {
		log.Printf("cache get error: %v", err)
	}

	// 2. Cache miss (or cache unavailable): ask the LLM.
	res, err := s.llm.Classify(ctx, name)
	if err != nil {
		log.Printf("classify error for %q: %v", name, err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	// 3. Cache the result for next time (best effort).
	if encoded, err := json.Marshal(res); err == nil {
		if err := s.cache.Set(ctx, key, string(encoded), s.ttl); err != nil {
			log.Printf("cache set error: %v", err)
		}
	}

	writeJSON(w, http.StatusOK, res)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
