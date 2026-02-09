package main

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

// HTTPServer - HTTP сервер для метрик и health
type HTTPServer struct {
    server *http.Server
    engine *CheckEngine
}

func newHTTPServer(address string, engine *CheckEngine) *HTTPServer {
    mux := http.NewServeMux()
    srv := &HTTPServer{
        engine: engine,
        server: &http.Server{
            Addr:         address,
            Handler:      mux,
            ReadTimeout:  10 * time.Second,
            WriteTimeout: 10 * time.Second,
            IdleTimeout:  60 * time.Second,
        },
    }
    
    // Регистрируем обработчики
    mux.HandleFunc("/health", srv.healthHandler)
    mux.HandleFunc("/stats", srv.statsHandler)
    mux.HandleFunc("/", srv.defaultHandler)
    
    return srv
}

func (s *HTTPServer) start() error {
    logInfo("HTTP server listening", "address", s.server.Addr)
    return s.server.ListenAndServe()
}

func (s *HTTPServer) shutdown(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}

func (s *HTTPServer) healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    
    health := map[string]interface{}{
        "status":    "healthy",
        "service":   "dns-proxy",
        "timestamp": time.Now().Format(time.RFC3339),
        "version":   "1.0.0",
    }
    
    json.NewEncoder(w).Encode(health)
}

func (s *HTTPServer) statsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    if s.engine == nil {
        json.NewEncoder(w).Encode(map[string]string{"error": "engine not initialized"})
        return
    }
    
    stats := s.engine.getStats()
    json.NewEncoder(w).Encode(stats)
}

func (s *HTTPServer) defaultHandler(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    
    w.Header().Set("Content-Type", "text/plain")
    w.Write([]byte("DNS Security Proxy v1.0.0\n"))
}
