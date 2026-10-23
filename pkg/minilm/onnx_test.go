package minilm

import (
	"testing"
)

// TestNewMiniLMEmbedder_Dimensions verifies Dim() returns 384.
func TestNewMiniLMEmbedder_Dimensions(t *testing.T) {
	e, err := NewMiniLMEmbedder()
	if err != nil {
		t.Skipf("ONNX not available (no GPU/libs): %v", err)
		return
	}
	defer e.Close()

	if e.Dim() != 384 {
		t.Errorf("Dim() = %d, want 384", e.Dim())
	}
}

// TestEmbed_ReturnsFloat32 verifies Embed returns non-empty float32 slice.
func TestEmbed_ReturnsFloat32(t *testing.T) {
	e, err := NewMiniLMEmbedder()
	if err != nil {
		t.Skipf("ONNX not available: %v", err)
		return
	}
	defer e.Close()

	vec, err := e.Embed("test sentence")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != 384 {
		t.Errorf("len(vec) = %d, want 384", len(vec))
	}
	// Check at least one non-zero value
	hasNonZero := false
	for _, v := range vec {
		if v != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Error("all embedding values are zero")
	}
}

// TestEmbedBatch_Consistency verifies batch output dimensions match.
func TestEmbedBatch_Consistency(t *testing.T) {
	e, err := NewMiniLMEmbedder()
	if err != nil {
		t.Skipf("ONNX not available: %v", err)
		return
	}
	defer e.Close()

	texts := []string{"pierwsze zdanie", "drugie zdanie", "trzecie"}
	vecs, err := e.EmbedBatch(texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != len(texts) {
		t.Errorf("len(vecs) = %d, want %d", len(vecs), len(texts))
	}
	for i, v := range vecs {
		if len(v) != 384 {
			t.Errorf("vecs[%d] len = %d, want 384", i, len(v))
		}
	}
}

// TestModelName returns the expected identifier.
func TestModelName(t *testing.T) {
	e, err := NewMiniLMEmbedder()
	if err != nil {
		t.Skipf("ONNX not available: %v", err)
		return
	}
	defer e.Close()

	if e.ModelName() == "" {
		t.Error("ModelName() is empty")
	}
}
