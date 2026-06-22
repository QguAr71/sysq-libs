// common/token_saver.go — diagnostyka prefix cache (Token Saver W1).
//
// Generyczny: przyjmuje UsageExtractor jako zależność.
// Backend implementuje Extract() — parsuje usage ze swojego formatu odpowiedzi.
// Token Saver nie wie nic o formacie backendu.
//
// Diagnostyka: co 10 requestów loguje podsumowanie (cached_tokens).
// Dla stream: parsuje SSE, ekstraktuje ostatni event z usage.
// Dla non-stream: czyta body, parsuje JSON.

package common

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"os"
	"sync/atomic"
	"time"
)

// UsageExtractor — każdy backend implementuje własne parsowanie usage.
type UsageExtractor interface {
	Extract(body []byte) *Usage
}

// Usage — dane o zużyciu tokenów z odpowiedzi API.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64 // opcjonalne — tylko jeśli backend wspiera cache
}

// TokenSaver — diagnostyka prefix cache.
// Loguje co 10 requestów.
type TokenSaver struct {
	diagCounter atomic.Uint64
	logPath     string
	extractor   UsageExtractor
}

// NewTokenSaver tworzy TokenSaver z ekstraktorem użycia.
// extractor — implementacja backendu (np. DeepSeek.Extract)
// logPath — ścieżka do token_saver.log
func NewTokenSaver(logPath string, extractor UsageExtractor) *TokenSaver {
	return &TokenSaver{
		logPath:   logPath,
		extractor: extractor,
	}
}

// RecordNonStream — diagnostyka dla non-stream response.
// Czyta body, ekstraktuje usage, inkrementuje liczniki.
// Zwraca zrekonstruowane body (bo zostało skonsumowane).
func (ts *TokenSaver) RecordNonStream(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	if ts.extractor != nil {
		usage := ts.extractor.Extract(data)
		ts.logUsage(usage)
	}

	return data, nil
}

// RecordStream — diagnostyka dla stream (SSE) response.
// Czyta SSE linia po linii, ekstraktuje usage z ostatniego eventu data:.
// Zwraca wszystkie dane (do przekazania klientowi) + liczbę chunków.
func (ts *TokenSaver) RecordStream(reader io.Reader, writer io.Writer) (chunkCount int64) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1MB max line
	var lastData []byte

	for scanner.Scan() {
		line := scanner.Bytes()
		writer.Write(line)
		writer.Write([]byte("\n"))
		chunkCount++

		if len(line) > 6 && string(line[:6]) == "data: " {
			data := line[6:]
			if string(data) != "[DONE]" {
				lastData = make([]byte, len(data))
				copy(lastData, data)
			}
		}
	}

	// Ekstraktuj usage z ostatniego eventu
	if len(lastData) > 0 && ts.extractor != nil {
		usage := ts.extractor.Extract(lastData)
		ts.logStreamUsage(usage, chunkCount)
	}

	return chunkCount
}

func (ts *TokenSaver) logUsage(usage *Usage) {
	if usage == nil {
		return
	}
	n := ts.diagCounter.Add(1)
	if n%10 == 0 {
		log.Printf("[token-saver] DIAG #%d: requests=%d cached_tokens=%d input=%d output=%d",
			n, n, usage.CachedTokens, usage.InputTokens, usage.OutputTokens)
		ts.writeLog(usage)
	}
}

func (ts *TokenSaver) logStreamUsage(usage *Usage, chunks int64) {
	if usage == nil {
		return
	}
	n := ts.diagCounter.Add(1)
	if n%10 == 0 {
		log.Printf("[token-saver] DIAG #%d: requests=%d cached_tokens=%d chunks=%d",
			n, n, usage.CachedTokens, chunks)
		ts.writeLog(usage)
	}
}

func (ts *TokenSaver) writeLog(usage *Usage) {
	if ts.logPath == "" {
		return
	}
	if err := os.MkdirAll(os.ExpandEnv("$HOME/.local/share/lazarus"), 0700); err != nil {
		return
	}
	f, err := os.OpenFile(ts.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	entry := map[string]interface{}{
		"ts":            time.Now().UTC().Format(time.RFC3339),
		"cached_tokens": usage.CachedTokens,
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	}
	data, _ := json.Marshal(entry)
	f.Write(data)
	f.Write([]byte("\n"))
}
