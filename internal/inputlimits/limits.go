// Package inputlimits defines the resource limits used at local trust boundaries.
package inputlimits

const (
	HarnessYAMLBytes        int64 = 4 << 20
	PersistedIndexBytes     int64 = 16 << 20
	SourceFileBytes         int64 = 1 << 20
	HistoricalEvidenceBytes int64 = 4 << 20
	SSEEventBytes           int64 = 1 << 20
	SSEBufferedBytes        int64 = 4 << 20
	RepositoryFiles               = 10000
	IndexChunks                   = 10000
	ChunkTextBytes          int64 = 64 << 10
)
