package minilm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	sp "github.com/tggo/goSentencePiece"
	ort "github.com/yalue/onnxruntime_go"
)

// ============================================================
// Constants
// ============================================================

const (
	modelURL     = "https://huggingface.co/sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2/resolve/main/onnx/model.onnx"
	modelSHA256  = "" // Set after first verified download
	tokenizerURL = "https://huggingface.co/sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2/resolve/main/tokenizer.json"
	tokenSHA256  = "" // Set after first verified download
	
	maxTokens = 512
	embedDim  = 384
)

// ============================================================
// MiniLMEmbedder
// ============================================================

// MiniLMEmbedder implements Embedder using MiniLM ONNX on CPU.
type MiniLMEmbedder struct {
	mu        sync.RWMutex
	session   *ort.AdvancedSession
	modelPath string
	tokenizer *sp.Tokenizer
	dim       int
}

// NewMiniLMEmbedder creates a MiniLM embedder.
func NewMiniLMEmbedder() (*MiniLMEmbedder, error) {
	cacheDir, err := ensureCacheDir()
	if err != nil {
		return nil, fmt.Errorf("cache dir: %w", err)
	}

	modelPath := filepath.Join(cacheDir, "model.onnx")
	tokPath := filepath.Join(cacheDir, "tokenizer.json")

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		slog.Info("MiniLM: downloading model")
		if err := downloadAndVerify(modelPath, resolveURL("LAZARUS_MODEL_URL", modelURL), modelSHA256); err != nil {
			return nil, fmt.Errorf("download model: %w", err)
		}
	}
	if _, err := os.Stat(tokPath); os.IsNotExist(err) {
		slog.Info("MiniLM: downloading tokenizer")
		if err := downloadAndVerify(tokPath, resolveURL("LAZARUS_TOKENIZER_URL", tokenizerURL), tokenSHA256); err != nil {
			return nil, fmt.Errorf("download tokenizer: %w", err)
		}
	}

	// Load tokenizer
	tok, err := sp.NewTokenizerFromJSON(tokPath)

	// Set shared library path and initialize ONNX Runtime environment
	if path := os.Getenv("ONNXRUNTIME_SHARED_LIBRARY_PATH"); path != "" {
		ort.SetSharedLibraryPath(path)
	} else {
		ort.SetSharedLibraryPath("/usr/lib/libonnxruntime.so")
	}
	if err := ort.InitializeEnvironment(); err != nil && !strings.Contains(err.Error(), "already been initialized") {
		return nil, fmt.Errorf("ONNX init: %w", err)
	}

	// Prepare ONNX tensors — batch=1, seq=maxTokens
	batchSize := int64(1)
	inputShape := ort.NewShape(batchSize, int64(maxTokens))
	inputIDs := make([]int64, batchSize*maxTokens)
	attentionMask := make([]int64, batchSize*maxTokens)
	outputShape := ort.NewShape(batchSize, int64(maxTokens), int64(embedDim))
	outputData := make([]float32, batchSize*maxTokens*embedDim)

	inputIDTensor, err := ort.NewTensor(inputShape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	maskTensor, err := ort.NewTensor(inputShape, attentionMask)
	if err != nil {
		inputIDTensor.Destroy()
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	outputTensor, err := ort.NewTensor(outputShape, outputData)
	if err != nil {
		inputIDTensor.Destroy()
		maskTensor.Destroy()
		return nil, fmt.Errorf("create output tensor: %w", err)
	}

	session, err := ort.NewAdvancedSession(
		modelPath,
		[]string{"input_ids", "attention_mask"},
		[]string{"last_hidden_state"},
		[]ort.Value{inputIDTensor, maskTensor},
		[]ort.Value{outputTensor},
		nil,
	)
	if err != nil {
		inputIDTensor.Destroy()
		maskTensor.Destroy()
		outputTensor.Destroy()
		return nil, fmt.Errorf("create ONNX session: %w", err)
	}

	return &MiniLMEmbedder{
		session:   session,
		modelPath: modelPath,
		tokenizer: tok,
		dim:       embedDim,
	}, nil
}

// Embed returns a single text embedding.
func (m *MiniLMEmbedder) Embed(text string) ([]float32, error) {
	batch, err := m.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return batch[0], nil
}

// EmbedBatch computes embeddings for a batch of texts.
// Inputs truncated to maxTokens, invalid UTF-8 replaced.
// WARNING: batch size is fixed at session creation (currently 1).
// For variable batches, recreate session or use max expected batch.
func (m *MiniLMEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty batch")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == nil {
		return nil, fmt.Errorf("embedder closed")
	}

	// Serialize batch into individual embeddings
	result := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := m.embedSingle(text)
		if err != nil {
			return nil, fmt.Errorf("text %d: %w", i, err)
		}
		result[i] = vec
	}

	return result, nil
}

// embedSingle tokenizes and runs inference for a single text.
func (m *MiniLMEmbedder) embedSingle(text string) ([]float32, error) {
	sanitized := strings.ToValidUTF8(text, "\uFFFD")
	
	ids, err := m.tokenizer.Encode(sanitized)
	if err != nil || len(ids) == 0 {
		// Fallback: return zero vector
		return make([]float32, embedDim), nil
	}

	// Build input tensors
	inputIDs := make([]int64, maxTokens)
	mask := make([]int64, maxTokens)
	
	if len(ids) > maxTokens {
		ids = ids[:maxTokens]
	}
	for j, id := range ids {
		inputIDs[j] = int64(id)
		mask[j] = 1
	}

	// Create tensors for this run
	inputShape := ort.NewShape(int64(1), int64(maxTokens))
	outputShape := ort.NewShape(int64(1), int64(maxTokens), int64(embedDim))
	outputData := make([]float32, 1*maxTokens*embedDim)

	inputIDTensor, err := ort.NewTensor(inputShape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("input_ids: %w", err)
	}
	defer inputIDTensor.Destroy()

	maskTensor, err := ort.NewTensor(inputShape, mask)
	if err != nil {
		inputIDTensor.Destroy()
		return nil, fmt.Errorf("attention_mask: %w", err)
	}
	defer maskTensor.Destroy()

	// XLM-R requires token_type_ids (segment IDs). All zeros for single sentence.
	tokenType := make([]int64, maxTokens)
	typeIDTensor, err := ort.NewTensor(inputShape, tokenType)
	if err != nil {
		return nil, fmt.Errorf("token_type_ids: %w", err)
	}
	defer typeIDTensor.Destroy()

	outputTensor, err := ort.NewTensor(outputShape, outputData)
	if err != nil {
		return nil, fmt.Errorf("output: %w", err)
	}
	defer outputTensor.Destroy()

	// Create temporary session
	tmpSess, err := ort.NewAdvancedSession(
		m.modelPath,
		[]string{"input_ids", "attention_mask", "token_type_ids"},
		[]string{"last_hidden_state"},
		[]ort.Value{inputIDTensor, maskTensor, typeIDTensor},
		[]ort.Value{outputTensor},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}
	defer tmpSess.Destroy()

	if err := tmpSess.Run(); err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	// Mean pooling
	return meanPoolFromData(outputData, mask, 0, 1, maxTokens), nil
}

func meanPoolFromData(data []float32, mask []int64, batchIdx, batchSize, seqLen int) []float32 {
	vec := make([]float32, embedDim)
	var count float64

	offset := batchIdx * seqLen * embedDim
	maskOffset := batchIdx * seqLen

	for t := 0; t < seqLen; t++ {
		if mask[maskOffset+t] == 0 {
			continue
		}
		count++
		tokOff := offset + t*embedDim
		for d := 0; d < embedDim; d++ {
			vec[d] += data[tokOff+d]
		}
	}

	if count > 0 {
		invCount := float32(1.0 / float64(math.Sqrt(float64(count))))
		for d := range vec {
			vec[d] *= invCount
		}
	}
	return vec
}

func (m *MiniLMEmbedder) Dim() int          { return m.dim }
func (m *MiniLMEmbedder) ModelName() string { return "paraphrase-multilingual-MiniLM-L12-v2 (ONNX CPU)" }

func (m *MiniLMEmbedder) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		m.session.Destroy()
		m.session = nil
	}
	return nil
}

// ============================================================
// Utilities (package-private)
// ============================================================

func ensureCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "lazarus", "models")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func resolveURL(envVar, defaultURL string) string {
	if url := os.Getenv(envVar); url != "" {
		return url
	}
	return defaultURL
}

func downloadAndVerify(path, url, expectedSHA string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	hasher := sha256.New()
	w := io.MultiWriter(f, hasher)

	if _, err := io.Copy(w, resp.Body); err != nil {
		os.Remove(path)
		return fmt.Errorf("download: %w", err)
	}

	actualSHA := hex.EncodeToString(hasher.Sum(nil))

	if expectedSHA == "" {
		slog.Warn("SHA256 not configured — trusting first download",
			"file", filepath.Base(path),
			"sha256", actualSHA,
		)
		return nil
	}

	if actualSHA != expectedSHA {
		os.Remove(path)
		return fmt.Errorf("SHA256 mismatch for %s: expected %s, got %s",
			filepath.Base(path), expectedSHA, actualSHA)
	}

	slog.Info("verified", "file", filepath.Base(path), "sha256", actualSHA)
	return nil
}

func init() {
	_ = utf8.RuneError // SEC4 — ensure utf8 package linked
}
