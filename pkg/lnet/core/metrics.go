// core/metrics.go — atomic counters + write-to-disk.
//
// LNES jest jedynym właścicielem echo_metrics.json.
// Metrics bridge (lazarusd) tylko czyta, nie zapisuje.
// Atomic write: tmp + fsync + dir fsync + rename.

package core

import (
	"encoding/json"
	"log"
	"os"
	"sync/atomic"
)

// Metrics trzyma globalne liczniki msg_count/tokens.
// Thread-safe przez atomic.Uint64.
type Metrics struct {
	msgCount atomic.Uint64
	tokens   atomic.Uint64
}

// NewMetrics tworzy Metrics z zerowymi licznikami.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// Record inkrementuje liczniki.
// msgDelta — liczba wiadomości (zwykle 1)
// tokenDelta — liczba tokenów (z usage response)
func (m *Metrics) Record(msgDelta uint64, tokenDelta uint64) {
	m.msgCount.Add(msgDelta)
	m.tokens.Add(tokenDelta)
}

// WriteToDisk zapisuje obecne liczniki do echo_metrics.json.
// Atomic write: tymczasowy plik → fsync → rename → dir fsync.
// Nigdy nie panikuje — błędy IO są logowane.
func (m *Metrics) WriteToDisk() {
	metrics := map[string]uint64{
		"msg_count": m.msgCount.Load(),
		"tokens":    m.tokens.Load(),
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		log.Printf("[lnet] metrics marshal error: %v", err)
		return
	}

	path := os.ExpandEnv("$HOME/.cache/goose/echo_metrics.json")
	tmpPath := path + ".tmp"

	if err := os.MkdirAll(os.ExpandEnv("$HOME/.cache/goose"), 0700); err != nil {
		log.Printf("[lnet] metrics mkdir error: %v", err)
		return
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		log.Printf("[lnet] metrics write error: %v", err)
		return
	}

	// fsync tymczasowego pliku
	if f, err := os.Open(tmpPath); err == nil {
		f.Sync()
		f.Close()
	}

	// atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		log.Printf("[lnet] metrics rename error: %v", err)
		return
	}

	// dir fsync (dla filesystemów które potrzebują)
	if dir, err := os.Open(os.ExpandEnv("$HOME/.cache/goose")); err == nil {
		dir.Sync()
		dir.Close()
	}

	log.Printf("[lnet] metrics saved: msg_count=%d tokens=%d", m.msgCount.Load(), m.tokens.Load())
}
