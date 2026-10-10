package ingest

import "fmt"

// DefaultChunkSize is the default number of blocks per backfill chunk.
const DefaultChunkSize = 2000

// PlanChunks splits [from, to] into chunks of at most chunkSize blocks.
// If chunkSize is 0 it defaults to DefaultChunkSize.
// The last chunk may be smaller than chunkSize.
func PlanChunks(from, to, chunkSize uint64) []Chunk {
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
	}
	if from > to {
		return nil
	}
	chunks := make([]Chunk, 0, (to-from)/chunkSize+1)
	for start := from; start <= to; start += chunkSize {
		end := start + chunkSize - 1
		if end > to {
			end = to
		}
		chunks = append(chunks, Chunk{From: start, To: end})
	}
	return chunks
}

// cursorKey returns the unique job string for a chunk used in meta.ingest_cursors.
func cursorKey(jobPrefix string, c Chunk) string {
	return fmt.Sprintf("%s:%d-%d", jobPrefix, c.From, c.To)
}
