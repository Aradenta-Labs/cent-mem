package embed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Embedder generates dense vector representations for input text.
type Embedder interface {
	Dims() int
	Embed(ctx context.Context, text string) ([]float32, error)
	Close() error
}

// ONNXEmbedder wraps the yalue/onnxruntime_go session.
type ONNXEmbedder struct {
	modelPath string
	dims      int
	session   *ort.AdvancedSession
	mu        sync.Mutex
	closed    bool
}

// NewONNX creates a new ONNX-backed Embedder.
func NewONNX(modelPath string, dims int, sharedLibPath string) (*ONNXEmbedder, error) {
	if modelPath == "" {
		return nil, errors.New("model path is required")
	}

	// Initialize onnxruntime environment if not already initialized
	if !ort.IsInitialized() {
		if sharedLibPath != "" {
			ort.SetSharedLibraryPath(sharedLibPath)
		}
		err := ort.InitializeEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize onnxruntime environment: %w", err)
		}
	}

	return &ONNXEmbedder{
		modelPath: modelPath,
		dims:      dims,
	}, nil
}

// Dims returns the output dimension size.
func (e *ONNXEmbedder) Dims() int {
	return e.dims
}

// Embed generates a vector for the given text.
func (e *ONNXEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil, errors.New("embedder is closed")
	}

	if e.session == nil {
		// Lazy load the session
		if _, err := os.Stat(e.modelPath); err != nil {
			return nil, fmt.Errorf("model file not found at %q: %w", e.modelPath, err)
		}

		// Simple sentence embedding inputs: for complete sentence transformers, inputs typically are
		// input_ids, token_type_ids, and attention_mask.
		// For a pure ONNX spike, we expect our models' inputs/outputs or use AdvancedSession.
		// In M0 we just want the wrapper interface and basic execution ready.
		// Note: The actual tensor tokenization and mean pooling are added in Phase 2.
		// For the Phase 0 spike, we implement a placeholder execution or direct session load.
		session, err := ort.NewAdvancedSession(
			e.modelPath,
			[]string{"input_ids", "attention_mask"},
			[]string{"last_hidden_state"},
			nil, // inputs
			nil, // outputs
			nil, // options
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create onnx session: %w", err)
		}
		e.session = session
	}

	// Tokenization + Tensor creation + session execution + pooling goes here in Phase 2.
	// For M0/Spike, we return a mock-filled array matching the dimensions of the model.
	// We will also support a StubEmbedder for pure unit tests without loading shared libs.
	out := make([]float32, e.dims)
	// Return a pseudo-random deterministic vector based on input text content
	var sum float32
	for i, char := range text {
		val := float32(math.Sin(float64(i) + float64(char)))
		out[i%e.dims] += val
		sum += val * val
	}
	// Normalize
	if sum > 0 {
		norm := float32(math.Sqrt(float64(sum)))
		for i := range out {
			out[i] /= norm
		}
	}

	return out, nil
}

// Close releases the ONNX session.
func (e *ONNXEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	if e.session != nil {
		err := e.session.Destroy()
		if err != nil {
			return fmt.Errorf("failed to destroy onnx session: %w", err)
		}
		e.session = nil
	}
	e.closed = true
	return nil
}

// StubEmbedder is a CGO-free, offline-ready mock embedder for tests and local development fallbacks.
type StubEmbedder struct {
	dims int
}

func NewStub(dims int) *StubEmbedder {
	return &StubEmbedder{dims: dims}
}

func (s *StubEmbedder) Dims() int {
	return s.dims
}

func (s *StubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return make([]float32, s.dims), nil
	}
	out := make([]float32, s.dims)
	var sum float32
	for i, char := range text {
		val := float32(math.Cos(float64(i) + float64(char)))
		out[i%s.dims] += val
		sum += val * val
	}
	if sum > 0 {
		norm := float32(math.Sqrt(float64(sum)))
		for i := range out {
			out[i] /= norm
		}
	}
	return out, nil
}

func (s *StubEmbedder) Close() error {
	return nil
}
