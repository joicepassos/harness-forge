package domain

type Document struct{ Path, Text, Hash string }
type Chunk struct {
	ID, Source, SourceHash, Text, Hash, Model string
	StartLine, EndLine                        int
	Vector                                    []float64
}
type Index struct {
	Version, Model string
	Dimensions     int
	Chunks         []Chunk
}
type Report struct {
	Added, Updated, Removed, Reused, Embedded, Tokens int
	Limitations                                       []string
}
