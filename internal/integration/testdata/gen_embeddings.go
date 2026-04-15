//go:build ignore

// gen_embeddings generates deterministic embedding vectors for integration test fixtures.
// Run with: go run internal/integration/testdata/gen_embeddings.go
//
// The output file (embeddings.json) contains:
//   - Pre-computed 1536-dim vectors for each chunk in the sample documents
//   - Query embeddings designed to match specific chunks for search tests
//
// Vector formula: vector[j] = sin(i * 0.1 + j * 0.01) where i is the chunk index.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
)

const dims = 1536

// generateVector produces a deterministic 1536-dim embedding using sin(i*0.1 + j*0.01).
func generateVector(i float64) []float64 {
	vec := make([]float64, dims)
	for j := 0; j < dims; j++ {
		vec[j] = math.Sin(i*0.1 + float64(j)*0.01)
	}
	return vec
}

// roundTo rounds a float to n decimal places to keep the JSON file manageable.
func roundTo(val float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Round(val*pow) / pow
}

func roundVector(vec []float64, decimals int) []float64 {
	rounded := make([]float64, len(vec))
	for i, v := range vec {
		rounded[i] = roundTo(v, decimals)
	}
	return rounded
}

type QueryEmbedding struct {
	Text                 string   `json:"text"`
	Embedding            []float64 `json:"embedding"`
	ExpectedNearestChunk string   `json:"expected_nearest_chunk,omitempty"`
	ExpectedTop3         []string `json:"expected_top3,omitempty"`
}

type EmbeddingsFile struct {
	Chunks  map[string][]float64          `json:"chunks"`
	Queries map[string]QueryEmbedding     `json:"queries"`
}

func main() {
	// Determine output path relative to this source file.
	_, thisFile, _, _ := runtime.Caller(0)
	outDir := filepath.Dir(thisFile)
	outPath := filepath.Join(outDir, "embeddings.json")

	data := EmbeddingsFile{
		Chunks:  make(map[string][]float64),
		Queries: make(map[string]QueryEmbedding),
	}

	// sample_markdown.md: 8 chunks (indices 0-7)
	for i := 0; i < 8; i++ {
		key := fmt.Sprintf("sample_markdown_%d", i)
		data.Chunks[key] = roundVector(generateVector(float64(i)), 8)
	}

	// sample_technical.md: 21 chunks (indices 0-20)
	for i := 0; i < 21; i++ {
		key := fmt.Sprintf("sample_technical_%d", i)
		data.Chunks[key] = roundVector(generateVector(float64(i)), 8)
	}

	// Query: exact match for chunk 5 (BM25 Full-Text Search)
	// Uses i=5.001 so the vector is very close to chunk 5 but not identical.
	data.Queries["exact_match_chunk5"] = QueryEmbedding{
		Text:                 "BM25 full-text search ranking",
		Embedding:            roundVector(generateVector(5.001), 8),
		ExpectedNearestChunk: "sample_markdown_5",
	}

	// Query: cross-document search for architecture and deployment
	// Uses i=1.5 to land between chunk 1 (Architecture Overview) and chunk 2.
	data.Queries["cross_document"] = QueryEmbedding{
		Text:         "architecture and deployment",
		Embedding:    roundVector(generateVector(1.5), 8),
		ExpectedTop3: []string{"sample_markdown_1", "sample_technical_8", "sample_technical_9"},
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}

	// Print stats.
	chunkCount := len(data.Chunks)
	queryCount := len(data.Queries)
	fi, _ := os.Stat(outPath)
	fmt.Printf("Generated %s\n", outPath)
	fmt.Printf("  Chunks: %d (each %d dims)\n", chunkCount, dims)
	fmt.Printf("  Queries: %d\n", queryCount)
	fmt.Printf("  File size: %.2f MB\n", float64(fi.Size())/(1024*1024))
}
