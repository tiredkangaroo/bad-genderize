package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds every tunable for the service, all sourced from the
// environment so nothing is hardcoded (especially the LLM proxy URL and
// the Redis connection/credentials).
type Config struct {
	HTTPAddr      string
	RedisAddr     string
	RedisUsername string
	RedisPassword string
	RedisDB       int
	RedisTLS      bool
	CacheTTL      time.Duration // 0 means cached forever
	LLMBaseURL    string        // OpenAI-compatible base, e.g. http://localhost:8080/v1
	LLMModel      string
	LLMAPIKey     string
	LLMTimeout    time.Duration
}

func loadConfig() (Config, error) {
	cfg := Config{
		HTTPAddr:      getenv("HTTP_ADDR", ":8081"),
		RedisAddr:     getenv("REDIS_ADDR", "localhost:6379"),
		RedisUsername: getenv("REDIS_USERNAME", ""),
		RedisPassword: getenv("REDIS_PASSWORD", ""),
		RedisDB:       getenvInt("REDIS_DB", 0),
		RedisTLS:      getenvBool("REDIS_TLS", false),
		LLMBaseURL:    strings.TrimRight(getenv("LLM_PROXY_URL", "http://localhost:8080/v1"), "/"),
		LLMModel:      getenv("LLM_MODEL", "gpt-4o-mini"),
		LLMAPIKey:     getenv("LLM_API_KEY", ""),
	}

	ttl, err := getenvDuration("CACHE_TTL", 0)
	if err != nil {
		return cfg, fmt.Errorf("invalid CACHE_TTL: %w", err)
	}
	cfg.CacheTTL = ttl

	timeout, err := getenvDuration("LLM_TIMEOUT", 30*time.Second)
	if err != nil {
		return cfg, fmt.Errorf("invalid LLM_TIMEOUT: %w", err)
	}
	cfg.LLMTimeout = timeout

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	if v := os.Getenv(key); v != "" {
		return time.ParseDuration(v)
	}
	return fallback, nil
}
