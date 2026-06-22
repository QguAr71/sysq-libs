// core/router.go — dispatch: input → output przez jawną routing table.
//
// Router trzyma mapę outputów (Output) i tablicę routingu (input name → output name).
// Route() znajduje output dla danego inputu przez routing table, woła Forward().
// Żadnej magii. Żadnego plugin discovery. III-41.

package core

import (
	"context"
	"fmt"
	"sync"
)

// Router dispatchuje requesty z inputów do outputów.
// Routing table jest jawna — podawana przy tworzeniu.
type Router struct {
	outputs map[string]Output // output name → output
	routes  map[string]string // input name → output name
	mu      sync.RWMutex
}

// NewRouter tworzy router z jawną routing table.
// routes to tablica Route{Input, Output} — mapowanie input → output.
func NewRouter(routes []Route) *Router {
	r := &Router{
		outputs: make(map[string]Output),
		routes:  make(map[string]string),
	}
	for _, route := range routes {
		r.routes[route.Input] = route.Output
	}
	return r
}

// RegisterOutput rejestruje output (backend) w routerze.
// Wołane przed Route() — output musi być zarejestrowany.
func (r *Router) RegisterOutput(name string, o Output) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outputs[name] = o
}

// Route znajduje output dla danego inputu przez routing table, woła Forward().
// Zwraca błąd jeśli output nie znaleziony.
func (r *Router) Route(ctx context.Context, inputName string, req *Request) (*Response, error) {
	r.mu.RLock()
	outputName := r.routes[inputName]
	output := r.outputs[outputName]
	r.mu.RUnlock()

	if output == nil {
		return nil, fmt.Errorf("no output for input %s (resolved: %s)", inputName, outputName)
	}

	return output.Forward(ctx, req)
}
