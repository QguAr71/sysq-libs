// input/http_proxy.go — HTTP forward proxy (OpenAI-compatible).
//
// Implementuje Input. Nasłuchuje na LNES_BIND_ADDR.
// Przyjmuje POST /v1/chat/completions, wstrzykuje thinking:disabled,
// forwarduje przez router do backendu, streamuje odpowiedź.
// Token Saver + metrics + CORS.

package input

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/QguAr71/sysq-libs/pkg/lnet/core"
	"github.com/QguAr71/sysq-libs/pkg/lnet/modules/common"
)

// HTTPProxy — moduł wejściowy HTTP (OpenAI-compatible proxy).
type HTTPProxy struct {
	server     *http.Server
	router     *core.Router
	metrics    *core.Metrics
	tokenSaver *common.TokenSaver
	maxBody    int64
	healthMon  *core.HealthMonitor
}

// NewHTTPProxy tworzy moduł HTTP proxy.
// cfg — konfiguracja LNES
// router — router dispatchujący do outputów
// metrics — globalne metryki
// tokenSaver — diagnostyka prefix cache (może być nil)
func NewHTTPProxy(cfg *core.Config, router *core.Router, metrics *core.Metrics, tokenSaver *common.TokenSaver) *HTTPProxy {
	hp := &HTTPProxy{
		router:     router,
		metrics:    metrics,
		tokenSaver: tokenSaver,
		maxBody:    cfg.MaxBodySize,
	}

	// Health monitor dla DeepSeek
	if cfg.DeepSeekAPIKey != "" {
		hp.healthMon = core.NewHealthMonitor(map[string]core.HealthChecker{
			"deepseek": core.DeepSeekHealthCheck(cfg.DeepSeekAPIKey),
		}, 30*time.Second)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", hp.serveHTTP)
	hp.server = &http.Server{
		Addr:    cfg.BindAddr,
		Handler: mux,
	}
	return hp
}

// Name zwraca nazwę modułu.
func (hp *HTTPProxy) Name() string { return "http-proxy" }

// ListenAndServe uruchamia serwer HTTP (blokuje).
func (hp *HTTPProxy) ListenAndServe() error {
	log.Printf("[lnet] HTTP proxy listening on %s", hp.server.Addr)
	return hp.server.ListenAndServe()
}

// Shutdown zatrzymuje serwer HTTP z timeoutem.
func (hp *HTTPProxy) Shutdown(ctx context.Context) error {
	return hp.server.Shutdown(ctx)
}

func (hp *HTTPProxy) serveHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.WriteHeader(204)
		return
	}

	// Health
	if r.Method == http.MethodGet && (r.URL.Path == "/health" || r.URL.Path == "/health/deepseek") {
		if hp.healthMon != nil {
			hp.healthMon.ServeHTTP(w, r)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		}
		return
	}

	// Tylko POST /v1/chat/completions
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		http.Error(w, "not found", 404)
		return
	}

	hp.handleChat(w, r)
}

func (hp *HTTPProxy) handleChat(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// 1. Read body z limitem
	body, err := io.ReadAll(io.LimitReader(r.Body, hp.maxBody))
	if err != nil {
		http.Error(w, "read error", 500)
		return
	}

	// 2. JSON parse
	var chatReq map[string]interface{}
	if err := json.Unmarshal(body, &chatReq); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	// 3. Inject thinking:disabled
	chatReq["thinking"] = map[string]string{"type": "disabled"}

	injectedBody, err := json.Marshal(chatReq)
	if err != nil {
		http.Error(w, "marshal error", 500)
		return
	}

	// 4. Build internal Request
	req := &core.Request{
		Method:  "POST",
		Path:    "/v1/chat/completions",
		Headers: common.FilterHeaders(r.Header),
		Body:    injectedBody,
	}

	// 5. Route → output
	ctx, cancel := context.WithTimeout(r.Context(), 65*time.Second)
	defer cancel()

	resp, err := hp.router.Route(ctx, "http-proxy", req)
	if err != nil {
		log.Printf("[lnet] route error: %v", err)
		http.Error(w, "upstream error", 502)
		return
	}
	defer resp.Body.Close()

	// 6. Stream response → client + Token Saver
	isStream := false
	if s, ok := chatReq["stream"].(bool); ok {
		isStream = s
	}

	var tokens uint64
	if isStream && hp.tokenSaver != nil {
		// Stream: Tee — przekazujemy do klienta i Token Saver równolegle
		pr, pw := io.Pipe()
		go func() {
			chunks := hp.tokenSaver.RecordStream(pr, io.Discard)
			tokens = uint64(chunks) // przybliżenie
			pr.Close()
		}()
		// Kopiuj odpowiedź do klienta + pipe dla Token Saver
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		tee := io.TeeReader(resp.Body, pw)
		io.Copy(w, tee)
		pw.Close()
	} else if !isStream && hp.tokenSaver != nil {
		// Non-stream: czytaj całe body → klient + Token Saver
		data, err := io.ReadAll(resp.Body)
		if err == nil && len(data) > 0 {
			// Token Saver
			hp.tokenSaver.RecordNonStream(bytes.NewReader(data))

			// Klient
			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			w.Write(data)
		} else {
			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
		}
	} else {
		// Brak Token Saver — prosty forward
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}

	// 7. Metrics
	duration := time.Since(startTime)
	hp.metrics.Record(1, tokens)
	log.Printf("[lnet] request: method=%s status=%d duration=%s stream=%s",
		r.Method, resp.StatusCode, duration.Round(time.Millisecond),
		strconv.FormatBool(isStream))
}
