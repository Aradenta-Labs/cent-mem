package embed

// ModelInfo holds metadata for supported embedding models.
type ModelInfo struct {
	Name   string
	URL    string
	SHA256 string
	Dims   int
	File   string
}

// ModelCatalog lists all official models supported by cent-mem.
// The default model is "bge-small-en-v1.5" (384 dimensions).
var ModelCatalog = map[string]ModelInfo{
	"bge-small-en-v1.5": {
		Name:   "bge-small-en-v1.5",
		URL:    "https://huggingface.co/BAAI/bge-small-en-v1.5/resolve/main/onnx/model.onnx",
		SHA256: "f07936a0d24e9cb7bf4e5e8e4544d6ff0b5406c64bc25d6b41d06e236371c667",
		Dims:   384,
		File:   "bge-small-en-v1.5.onnx",
	},
}
