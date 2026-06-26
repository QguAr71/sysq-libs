package llmsched

import (
	"context"
	"testing"
	"time"
)

// TestSingleFlight — dwa identyczne prompty → jeden request HTTP, dwie odpowiedzi
func TestSingleFlight(t *testing.T) {
	t.Skip("wymaga działającej Ollamy — test integracyjny")
}

// TestPriorityOrder — HIGH przed LOW (bounded fairness struktura)
func TestPriorityOrder(t *testing.T) {
	s := New("http://localhost:11434", "phi3.5:3.8b", 32, 120*time.Second)

	// Wrzuć 5 HIGH, potem sprawdź że dispatcher z bounded fairness
	// daje max 10 HIGH przed LOW
	// Test BoundedFairness już to sprawdza lepiej — ten tylko sanity check

	// Wrzuć 1 HIGH, 1 LOW — dispatcher powinien obsłużyć oba
	s.highQueue <- &Request{Priority: PrioHigh}
	s.lowQueue <- &Request{Priority: PrioLow}

	req1 := s.dispatch()
	req2 := s.dispatch()

	if req1 == nil || req2 == nil {
		t.Fatal("expected 2 requests")
	}

	// Nie sprawdzamy kolejności — Go select jest pseudolosowy
	// Sprawdzamy tylko że oba zostały zdispatchowane
	t.Logf("priorities: %d, %d", req1.Priority, req2.Priority)
}

// TestBackpressure — LOW drop przy pełnej kolejce (test struktury)
func TestBackpressure(t *testing.T) {
	s := New("http://localhost:11434", "phi3.5:3.8b", 2, 120*time.Second)

	// Wypełnij LOW queue do pełna
	for i := 0; i < 2; i++ {
		s.lowQueue <- &Request{Priority: PrioLow}
		s.lowQueued.Add(1)
	}

	// Trzeci LOW powinien dostać ErrQueueFull
	req := Request{
		Prompt:   "test",
		Priority: PrioLow,
		Timeout:  100 * time.Millisecond,
		Ctx:      context.Background(),
	}
	resp := s.Submit(req)
	if resp.Err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", resp.Err)
	}
}

// TestTimeout — request timeoutuje przed dispatch
func TestTimeout(t *testing.T) {
	s := New("http://localhost:11434", "phi3.5:3.8b", 32, 120*time.Second)
	s.Start()
	defer s.Shutdown(context.Background())

	// Wypełnij kolejkę tak żeby timeout minął zanim worker dojdzie
	s.lowQueue <- &Request{} // blokada — worker będzie czekał

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := Request{
		Prompt:   "test",
		Priority: PrioHigh,
		Timeout:  50 * time.Millisecond,
		Ctx:      ctx,
	}
	resp := s.Submit(req)

	if resp.Err == nil {
		t.Error("expected timeout, got success")
	}
	if resp.Err != ErrTimeout && resp.Err != context.DeadlineExceeded {
		t.Logf("got error: %v (expected timeout or deadline exceeded)", resp.Err)
	}
}

// TestShutdown — po shutdown Submit zwraca błąd
func TestShutdown(t *testing.T) {
	s := New("http://localhost:11434", "phi3.5:3.8b", 32, 120*time.Second)
	s.Start()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	s.Shutdown(ctx)

	req := Request{
		Prompt:   "test",
		Priority: PrioHigh,
		Timeout:  5 * time.Second,
		Ctx:      context.Background(),
	}
	resp := s.Submit(req)

	if resp.Err == nil {
		t.Error("expected error after shutdown")
	}
}

// TestBoundedFairness — max 10 HIGH między LOW
func TestBoundedFairness(t *testing.T) {
	s := New("http://localhost:11434", "phi3.5:3.8b", 32, 120*time.Second)
	// Nie startujemy workera — testujemy dispatch() bezpośrednio

	// Wrzucamy 15 HIGH, 3 LOW
	for i := 0; i < 15; i++ {
		s.highQueue <- &Request{Priority: PrioHigh}
	}
	for i := 0; i < 3; i++ {
		s.lowQueue <- &Request{Priority: PrioLow}
	}

	// Test dispatch — liczymy HIGH między LOW
	maxHighBetweenLow := 0
	currentHigh := 0
	lowCount := 0

	for i := 0; i < 18; i++ {
		req := s.dispatch()
		if req == nil {
			break
		}
		if req.Priority == PrioHigh {
			currentHigh++
		} else {
			lowCount++
			if currentHigh > maxHighBetweenLow {
				maxHighBetweenLow = currentHigh
			}
			currentHigh = 0
		}
	}

	t.Logf("max HIGH between LOW: %d, LOW count: %d", maxHighBetweenLow, lowCount)

	if maxHighBetweenLow > 10 {
		t.Errorf("bounded fairness violated: %d HIGH between LOW (max allowed: 10)", maxHighBetweenLow)
	}
}

// TestStats — metryki po kilku dispatchach
func TestStats(t *testing.T) {
	s := New("http://localhost:11434", "phi3.5:3.8b", 32, 120*time.Second)

	stats := s.Stats()
	if stats.Uptime <= 0 {
		t.Error("uptime should be > 0")
	}
	if stats.HighQueued != 0 || stats.LowQueued != 0 {
		t.Error("queues should be empty at start")
	}
}
