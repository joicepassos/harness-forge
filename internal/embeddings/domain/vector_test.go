package domain

import (
	"math"
	"testing"
)

func vector(values ...float64) Vector {
	return Vector{Model: "fixture-v1", Dimensions: len(values), Values: values}
}
func TestCosineSimilarity(t *testing.T) {
	value, err := Cosine(vector(1, 2), vector(1, 2))
	if err != nil || math.Abs(value-1) > 1e-12 {
		t.Fatalf("%v %v", value, err)
	}
	value, err = Cosine(vector(1, 0), vector(0, 1))
	if err != nil || math.Abs(value) > 1e-12 {
		t.Fatalf("%v %v", value, err)
	}
}
func TestCosineRejectsInvalidVectors(t *testing.T) {
	cases := [][2]Vector{{vector(1), {Model: "fixture-v1", Dimensions: 2, Values: []float64{1, 2}}}, {vector(0, 0), vector(0, 0)}, {{Model: "fixture-v1", Dimensions: 1, Values: []float64{math.NaN()}}, vector(1)}, {vector(1), {Model: "other", Dimensions: 1, Values: []float64{1}}}}
	for _, pair := range cases {
		if _, err := Cosine(pair[0], pair[1]); err == nil {
			t.Fatal("invalid vectors accepted")
		}
	}
}
