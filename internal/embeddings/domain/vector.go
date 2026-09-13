package domain

import (
	"fmt"
	"math"
)

type Vector struct {
	Model      string    `json:"model"`
	Dimensions int       `json:"dimensions"`
	Values     []float64 `json:"values"`
	Tokens     int       `json:"tokens,omitempty"`
}

func (v Vector) Validate() error {
	if v.Model == "" || len(v.Values) == 0 || v.Dimensions != len(v.Values) {
		return fmt.Errorf("embedding must include a model and matching non-empty dimensions")
	}
	for _, value := range v.Values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("embedding contains a non-finite value")
		}
	}
	return nil
}

func Cosine(a, b Vector) (float64, error) {
	if err := a.Validate(); err != nil {
		return 0, err
	}
	if err := b.Validate(); err != nil {
		return 0, err
	}
	if a.Model != b.Model || a.Dimensions != b.Dimensions {
		return 0, fmt.Errorf("embeddings use incompatible model or dimensions")
	}
	var dot, aa, bb float64
	for i := range a.Values {
		dot += a.Values[i] * b.Values[i]
		aa += a.Values[i] * a.Values[i]
		bb += b.Values[i] * b.Values[i]
	}
	if aa == 0 || bb == 0 {
		return 0, fmt.Errorf("cosine similarity is undefined for a zero vector")
	}
	return dot / (math.Sqrt(aa) * math.Sqrt(bb)), nil
}
