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
		SHA256: "828e1496d7fabb79cfa4dcd84fa38625c0d3d21da474a00f08db0f559940cf35",
		Dims:   384,
		File:   "bge-small-en-v1.5.onnx",
	},
}
