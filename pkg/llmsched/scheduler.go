// Package llmsched — single-GPU cooperative scheduler for LLM inference.
//
// Jeden worker, dwie kolejki (HIGH/LOW), credit-based fairness.
// Zero zależności od Lazarusa — czysty Go, tylko Ollama HTTP API.
//
// Invarianty: patrz BLUEPRINT_LLM_SCHEDULER.md §13
package llmsched

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
	lru "github.com/hashicorp/golang-lru/v2"
)

// ──────────────────────────────────────────────────────────────────────────────
// Typy
// ──────────────────────────────────────────────────────────────────────────────

// Priority to priorytet requestu w kolejce.
type Priority int

const (
	PrioHigh   Priority = 0  // Router — blokuje write
	PrioNormal Priority = 10 // interaktywne zapytania (przyszłość)
	PrioLow    Priority = 20 // GraphUnit, PSL — async, retryable
)

// Błędy eksportowane
var (
	ErrQueueFull = errors.New("llmsched: queue full")
	ErrTimeout   = errors.New("llmsched: request timeout")
	ErrShutdown  = errors.New("llmsched: scheduler shutdown")
)

// ──────────────────────────────────────────────────────────────────────────────
// Request
// ──────────────────────────────────────────────────────────────────────────────

type Request struct {
	Prompt     string
	Priority   Priority
	Timeout    time.Duration
	Ctx        context.Context
	cancel     context.CancelFunc
	responseCh chan Response
	SubmitTime time.Time
}

// ──────────────────────────────────────────────────────────────────────────────
// Response
// ──────────────────────────────────────────────────────────────────────────────

type Response struct {
	Text     string
	Model    string
	Duration time.Duration
	Tokens   int
	Err      error
}

// ──────────────────────────────────────────────────────────────────────────────
// Stats — diagnostyka
// ──────────────────────────────────────────────────────────────────────────────

type Stats struct {
	HighQueued          int
	LowQueued           int
	Inflight            int
	Dropped             int64
	TotalDispatched     int64
	PanicsRecovered     int64
	StarvedLowDispatched int64
	LastDispatch        time.Time
	Uptime              time.Duration
}

// ──────────────────────────────────────────────────────────────────────────────
// Scheduler
// ──────────────────────────────────────────────────────────────────────────────

type Scheduler struct {
	client       *Client
	highQueue    chan *Request
	lowQueue     chan *Request
	ctx          context.Context
	cancel       context.CancelFunc
	maxSize      int
	defaultTimeout time.Duration
	workerWg     sync.WaitGroup

	// Credit-based starvation protection (scale ×10 dla atomic.Int64)
	lowCredit           atomic.Int64 // credit ×10: 2.0 → 20, 1.0 → 10, 0.2 → 2

	// Metryki
	highQueued          atomic.Int64
	lowQueued           atomic.Int64
	inflight            atomic.Int64
	dropped             atomic.Int64
	totalDispatched     atomic.Int64
	starvedLowDispatched atomic.Int64
	panicsRecovered     atomic.Int64
	lastDispatch        atomic.Value  // time.Time
	startTime           time.Time
	logger              *slog.Logger
}

// New tworzy Scheduler.
func New(endpoint, model string, maxQueueSize int, defaultTimeout time.Duration) *Scheduler {
	if maxQueueSize <= 0 {
		maxQueueSize = 32
	}
	if maxQueueSize > 1024 {
		maxQueueSize = 1024
	}
	if defaultTimeout <= 0 {
		defaultTimeout = 120 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Scheduler{
		client:         NewClient(endpoint, model),
		highQueue:      make(chan *Request, maxQueueSize),
		lowQueue:       make(chan *Request, maxQueueSize),
		ctx:            ctx,
		cancel:         cancel,
		maxSize:        maxQueueSize,
		defaultTimeout: defaultTimeout,
		startTime:      time.Now(),
		logger:         slog.Default().With("component", "llmsched"),
	}

	return s
}

// Start uruchamia worker goroutine.
func (s *Scheduler) Start() {
	s.workerWg.Add(1)
	go func() {
		defer s.workerWg.Done()
		s.worker()
	}()
}

// Shutdown zamyka Scheduler — czeka na inflight request.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	s.cancel()

	done := make(chan struct{})
	go func() {
		s.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats zwraca metryki.
func (s *Scheduler) Stats() Stats {
	lastDispatch, _ := s.lastDispatch.Load().(time.Time)
	return Stats{
		HighQueued:          int(s.highQueued.Load()),
		LowQueued:           int(s.lowQueued.Load()),
		Inflight:            int(s.inflight.Load()),
		Dropped:             s.dropped.Load(),
		TotalDispatched:     s.totalDispatched.Load(),
		PanicsRecovered:     s.panicsRecovered.Load(),
		StarvedLowDispatched: s.starvedLowDispatched.Load(),
		LastDispatch:        lastDispatch,
		Uptime:              time.Since(s.startTime),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// worker — single worker goroutine, per-request recover
// ──────────────────────────────────────────────────────────────────────────────

func (s *Scheduler) worker() {
	for {
		req := s.dispatch()
		if req == nil {
			return // shutdown
		}

		s.inflight.Store(1)

		func() {
			defer func() {
				if r := recover(); r != nil {
					s.panicsRecovered.Add(1)
					s.logger.Error("worker_panic_recovered", "panic", r)
				}
				s.inflight.Store(0)
			}()

			// Dual-layer timeout: queue + HTTP
			if req.Timeout > 0 && time.Since(req.SubmitTime) > req.Timeout {
				req.responseCh <- Response{Err: ErrTimeout}
				return
			}
			if req.Ctx.Err() != nil {
				req.responseCh <- Response{Err: req.Ctx.Err()}
				return
			}

			text, err := s.client.Generate(req.Ctx, req.Prompt)
			if err != nil {
				req.responseCh <- Response{Err: err}
				return
			}

			duration := time.Since(req.SubmitTime)
			s.totalDispatched.Add(1)
			s.lastDispatch.Store(time.Now())
			s.logger.Debug("dispatch",
				"priority", req.Priority,
				"queue_wait", duration,
				"prompt_len", len(req.Prompt),
			)

			req.responseCh <- Response{
				Text:     text,
				Model:    s.client.Model(),
				Duration: duration,
			}
		}()
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// dispatch — credit-based, pojedynczy select, bounded fairness
// ──────────────────────────────────────────────────────────────────────────────

func (s *Scheduler) dispatch() *Request {
	for {
		credit := s.lowCredit.Load() // scale ×10

		// Cap 20 (2.0 ×10) — zapobiega windowaniu przy pustej LOW queue
		if credit > 20 {
			s.lowCredit.Store(20)
			credit = 20
		}

		// Gdy credit >= 10 (1.0 ×10) i obie kolejki niepuste: Go select pseudolosowy,
		// ale bounded fairness: max 10 HIGH między LOW.
		select {
		case req := <-s.highQueue:
			s.highQueued.Add(-1)
			for {
				old := s.lowCredit.Load()
				new := old + 2 // +0.2 ×10
				if new > 20 {
					new = 20 // cap 2.0
				}
				if s.lowCredit.CompareAndSwap(old, new) {
					break
				}
			}
			return req
		case req := <-s.lowQueue:
			s.lowQueued.Add(-1)
			s.lowCredit.Store(0) // reset
			s.starvedLowDispatched.Add(1)
			return req
		case <-s.ctx.Done():
			return nil
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Submit — blokujące, z timeoutem i backpressure
// ──────────────────────────────────────────────────────────────────────────────

func (s *Scheduler) Submit(req Request) Response {
	if req.SubmitTime.IsZero() {
		req.SubmitTime = time.Now()
	}
	if req.Timeout == 0 {
		req.Timeout = s.defaultTimeout
	}
	if req.Ctx == nil {
		req.Ctx = context.Background()
	}

	// Dual-layer timeout: ctx deadline = SubmitTime + Timeout
	deadline := req.SubmitTime.Add(req.Timeout)
	ctx, cancel := context.WithDeadline(req.Ctx, deadline)
	req.Ctx = ctx
	req.cancel = cancel
	defer cancel()

	req.responseCh = make(chan Response, 1)

	// Enqueue
	var q chan *Request
	if req.Priority == PrioHigh {
		q = s.highQueue
		s.highQueued.Add(1)
	} else {
		q = s.lowQueue
		s.lowQueued.Add(1)
	}

	select {
	case q <- &req:
		// OK
	default:
		if req.Priority == PrioLow {
			s.dropped.Add(1)
			if s.dropped.Load()%10 == 0 {
				s.logger.Warn("backpressure_dropped", "total", s.dropped.Load())
			}
			return Response{Err: ErrQueueFull}
		}
		// HIGH: czeka z ctx, nie blokuje wiecznie
		select {
		case q <- &req:
			// OK
		case <-req.Ctx.Done():
			s.highQueued.Add(-1)
			return Response{Err: req.Ctx.Err()}
		}
	}

	// Czekaj na odpowiedź
	select {
	case resp := <-req.responseCh:
		return resp
	case <-req.Ctx.Done():
		return Response{Err: req.Ctx.Err()}
	}
}

// SubmitAsync — nieblokujący
func (s *Scheduler) SubmitAsync(req Request) <-chan Response {
	if req.SubmitTime.IsZero() {
		req.SubmitTime = time.Now()
	}
	if req.Ctx == nil {
		req.Ctx = context.Background()
	}
	req.responseCh = make(chan Response, 1)

	var q chan *Request
	if req.Priority == PrioHigh {
		q = s.highQueue
		s.highQueued.Add(1)
	} else {
		q = s.lowQueue
		s.lowQueued.Add(1)
	}

	select {
	case q <- &req:
	default:
		if req.Priority == PrioLow {
			s.dropped.Add(1)
			req.responseCh <- Response{Err: ErrQueueFull}
			return req.responseCh
		}
	}

	return req.responseCh
}

// ──────────────────────────────────────────────────────────────────────────────
// Client — HTTP do Ollamy, singleflight + LRU cache
// ──────────────────────────────────────────────────────────────────────────────

type Client struct {
	endpoint    string
	model       string
	httpClient  *http.Client
	sfGroup     singleflight.Group
	cache       *lru.Cache[string, cacheEntry]
	defaultTTL  time.Duration
	cacheEnabled bool
}

type cacheEntry struct {
	response  string
	timestamp time.Time
}

func NewClient(endpoint, model string) *Client {
	// LRU cache: max 1000 entries
	cache, _ := lru.New[string, cacheEntry](1000)

	return &Client{
		endpoint:   endpoint,
		model:      model,
		httpClient: &http.Client{Timeout: 90 * time.Second},
		cache:      cache,
		defaultTTL: 1 * time.Hour,
		cacheEnabled: true,
	}
}

func (c *Client) Model() string { return c.model }

// Generate wysyła prompt do Ollamy i zwraca surowy tekst.
// Używa singleflight dla identycznych promptów.
// Retry: HTTP 503 "loading model" → max 3 próby z exponential backoff.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	if !c.cacheEnabled {
		return c.doGenerate(ctx, prompt)
	}

	key := sha256Hex(prompt)

	// Singleflight — deduplikacja równoległych requestów
	v, err, _ := c.sfGroup.Do(key, func() (interface{}, error) {
		// Sprawdź cache
		if entry, ok := c.cache.Get(key); ok {
			return entry.response, nil
		}

		text, err := c.doGenerate(ctx, prompt)
		if err == nil {
			c.cache.Add(key, cacheEntry{response: text, timestamp: time.Now()})
		}
		return text, err
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// doGenerate — surowe HTTP POST do Ollamy z retry dla 503.
func (c *Client) doGenerate(ctx context.Context, prompt string) (string, error) {
	type ollamaRequest struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}

	body := ollamaRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.endpoint+"/api/generate", strings.NewReader(string(jsonBody)))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http post: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body: %w", err)
			continue
		}

		// Retry 503 "loading model"
		if resp.StatusCode == 503 && strings.Contains(string(respBody), "loading") {
			lastErr = fmt.Errorf("ollama loading model (503)")
			continue
		}

		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
			continue
		}

		var result struct {
			Response string `json:"response"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			lastErr = fmt.Errorf("unmarshal response: %w", err)
			continue
		}

		return result.Response, nil
	}

	return "", lastErr
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:])
}
