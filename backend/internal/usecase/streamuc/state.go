package streamuc

import "time"

// streamMeta holds static metadata for a registered stream.
// Computed once at startup and never mutated — no locking required.
type streamMeta struct {
	segCount       int
	targetDuration int
	startedAt      time.Time
}
