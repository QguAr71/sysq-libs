// core/health.go — health check z cache (30s) dla upstream.
//
// Osobna goroutine monitoruje upstream. GET /health zwraca status z cache,
// nie odpytuje upstream per request — unikamy spamu.
// GET /health/deepseek zwraca szczegółowy status DeepSeek.

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

// HealthChecker — funkcja sprawdzająca stan upstream.
// Zwraca nil jeśli OK, error jeśli degraded/down.
type HealthChecker func(ctx context.Context) error

// HealthMonitor monitoruje upstream, cache'uje status.
// Handler HTTP zwraca status z cache.
type HealthMonitor struct {
	status   atomic.Value // map[string]string — "ok" / "degraded" / "down"
	checkers map[string]HealthChecker
	interval time.Duration
}

// NewHealthMonitor tworzy monitor z listą checkerów.
// Uruchamia goroutine z periodycznym sprawdzaniem co interval.
func NewHealthMonitor(checkers map[string]HealthChecker, interval time.Duration) *HealthMonitor {
	hm := &HealthMonitor{
		checkers: checkers,
		interval: interval,
	}
	hm.status.Store(make(map[string]string))
	go hm.run()
	return hm
}

// run odpytuje wszystkie checkery co interval, aktualizuje cache.
func (hm *HealthMonitor) run() {
	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	// pierwszy run od razu
	hm.checkAll()

	for range ticker.C {
		hm.checkAll()
	}
}

func (hm *HealthMonitor) checkAll() {
	statuses := make(map[string]string)
	for name, checker := range hm.checkers {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := checker(ctx)
		cancel()
		if err == nil {
			statuses[name] = "ok"
		} else {
			statuses[name] = "degraded"
			log.Printf("[lnet] health check %s: %v", name, err)
		}
	}
	hm.status.Store(statuses)
}

// ServeHTTP obsługuje health endpointy.
// GET /health → {"status":"ok"}
// GET /health/deepseek → {"status":"ok"} lub {"status":"degraded","reason":"..."}
func (hm *HealthMonitor) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	statuses := hm.status.Load().(map[string]string)

	switch r.URL.Path {
	case "/health":
		writeJSON(w, 200, map[string]string{"status": "ok"})

	case "/health/deepseek":
		s, ok := statuses["deepseek"]
		if !ok {
			writeJSON(w, 200, map[string]string{"status": "unknown"})
			return
		}
		if s == "ok" {
			writeJSON(w, 200, map[string]string{"status": "ok"})
		} else {
			writeJSON(w, 200, map[string]string{"status": "degraded", "reason": "upstream unreachable"})
		}

	default:
		w.WriteHeader(404)
	}
}

// deepseekHealthCheck sprawdza DeepSeek API — GET /v1/models, timeout 3s.
// TODO: zweryfikować stabilność endpointu przed implementacją.
func DeepSeekHealthCheck(apiKey string) HealthChecker {
	client := &http.Client{Timeout: 3 * time.Second}
	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, "GET", "https://api.deepseek.com/v1/models", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("upstream status %d", resp.StatusCode)
		}
		return nil
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
