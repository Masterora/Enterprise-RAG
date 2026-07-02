package embedding

import (
	"context"
	"hash/fnv"
	"math"
)

// MockEmbedder is a temporary local-only provider for iteration validation.
// It should be removed when the project switches fully to a real embedding provider.
type MockEmbedder struct {
	dimension int
}

func NewMockEmbedder(dimension int) *MockEmbedder {
	if dimension <= 0 {
		dimension = 1536
	}
	return &MockEmbedder{dimension: dimension}
}

func (m *MockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vectors = append(vectors, m.embedText(text))
	}
	return vectors, nil
}

func (m *MockEmbedder) embedText(text string) []float32 {
	vector := make([]float32, m.dimension)

	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(text))
	seed := hasher.Sum64()

	for index := 0; index < m.dimension; index++ {
		value := float64((seed+uint64(index*2654435761))%10000)/5000 - 1
		vector[index] = float32(value)
	}

	var norm float64
	for _, value := range vector {
		norm += float64(value * value)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return vector
	}
	for index, value := range vector {
		vector[index] = float32(float64(value) / norm)
	}
	return vector
}
