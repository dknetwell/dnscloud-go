package main

import (
	"encoding/json"
	"net/http"
)

type HTTPServer struct {
	engine *CheckEngine
	cfg    *Config
}

func NewHTTPServer(engine *CheckEngine, cfg *Config) *HTTPServer {
	return &HTTPServer{
		engine: engine,
		cfg:    cfg,
	}
}

func (s *HTTPServer) Start() {

	http.HandleFunc("/stats", s.handleStats)
	http.Handle("/metrics", promHandler())

	LogInfo("http", "HTTP server started on "+s.cfg.HTTP.Listen)
	http.ListenAndServe(s.cfg.HTTP.Listen, nil)
}

func (s *HTTPServer) handleStats(w http.ResponseWriter, r *http.Request) {

	stats := s.engine.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
