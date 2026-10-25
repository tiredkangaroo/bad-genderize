package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	llm := NewLLMClient(cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMAPIKey, cfg.LLMTimeout)
	cache := NewRedisCache(cfg)

	// Redis is required for caching; if it's down we still serve, just uncached.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := cache.Ping(ctx); err != nil {
		log.Printf("warning: redis unreachable (%v); serving uncached", err)
	}
	cancel()

	handler := newGenderServer(llm, cache, cfg.CacheTTL)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.LLMTimeout + 10*time.Second,
	}

	log.Printf("listening on %s (LLM model %q via %s)", cfg.HTTPAddr, cfg.LLMModel, cfg.LLMBaseURL)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
