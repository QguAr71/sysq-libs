// core/shutdown.go — graceful shutdown: inputs → metrics.
//
// Kolejność: Shutdown wszystkie inputy (każdy z timeoutem z ctx),
// potem flush metrics. Zgodnie z audytem — metrics LAST.

package core

import (
	"context"
	"log"
	"time"
)

// GracefulShutdown zatrzymuje wszystkie inputy z timeoutem, potem flush metrics.
//
// Każdy Input.Shutdown(ctx) powinien blokować do zakończenia
// in-flight requestów (lub do timeoutu z ctx). Po wszystkich shutdown
// wołane jest WriteToDisk() — metrics są zapisywane dopiero gdy
// wszystkie requesty są zakończone.
func GracefulShutdown(ctx context.Context, inputs []Input, m *Metrics) error {
	// 1. Shutdown all inputs — each blocks until in-flight requests complete
	//    (or until timeout from ctx)
	perInputTimeout := 5 * time.Second
	if len(inputs) > 0 {
		perInputTimeout = time.Until(deadlineOrMax(ctx, 5*time.Second)) / time.Duration(len(inputs))
		if perInputTimeout < 500*time.Millisecond {
			perInputTimeout = 500 * time.Millisecond
		}
	}

	for _, in := range inputs {
		shutdownCtx, cancel := context.WithTimeout(ctx, perInputTimeout)
		if err := in.Shutdown(shutdownCtx); err != nil {
			log.Printf("[lnet] shutdown input %s: %v", in.Name(), err)
		}
		cancel()
	}

	// 2. Flush metrics LAST (after all requests are done)
	m.WriteToDisk()
	return nil
}

func deadlineOrMax(ctx context.Context, max time.Duration) time.Time {
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < max {
			return deadline
		}
	}
	return time.Now().Add(max)
}
