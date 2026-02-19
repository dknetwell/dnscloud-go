package main

import (
	"context"
	"encoding/json"
	"net/http"
)

type HTTPServer struct {
	engine *CheckEngine
	cfg    *Config
	server *http.Server
}

func NewHTTPServer(engine *CheckEngine, cfg *Config) *HTTPServer {
	return &HTTPServer{
		engine: engine,
		cfg:    cfg,
	}
}

func (s *HTTPServer) Start() {

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/stats", s.handleStats)
	mux.Handle("/metrics", promHandler())

	s.server = &http.Server{
		Addr:    s.cfg.HTTP.Listen,
		Handler: mux,
	}

	LogInfo("http", "HTTP server started on "+s.cfg.HTTP.Listen)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		LogError("http", err.Error())
	}
}

func (s *HTTPServer) Shutdown(ctx context.Context) {
	if s.server != nil {
		_ = s.server.Shutdown(ctx)
	}
}

func (s *HTTPServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
