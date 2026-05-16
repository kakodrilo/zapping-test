package streamuc

import "sync"

// streamState holds only the rotation cursor for one live stream.
// Memory cost: 4 ints + mutex, regardless of how many segments exist.
type streamState struct {
	windowStart    int
	sequence       int
	segCount       int
	targetDuration int
	mu             sync.RWMutex
}

type StateRegistry struct {
	states map[int]*streamState
	mu     sync.RWMutex
}

func newStateRegistry() *StateRegistry {
	return &StateRegistry{states: make(map[int]*streamState)}
}

func (r *StateRegistry) init(streamID, segCount, targetDuration, initialOffset int) {
	s := &streamState{
		windowStart:    initialOffset % segCount,
		segCount:       segCount,
		targetDuration: targetDuration,
	}
	r.mu.Lock()
	r.states[streamID] = s
	r.mu.Unlock()
}

func (r *StateRegistry) advance(streamID int) {
	r.mu.RLock()
	s := r.states[streamID]
	r.mu.RUnlock()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.windowStart = (s.windowStart + 1) % s.segCount
	s.sequence++
	s.mu.Unlock()
}

// snapshot returns a consistent read of the current cursor state.
func (r *StateRegistry) snapshot(streamID int) (windowStart, sequence, segCount, targetDuration int, ok bool) {
	r.mu.RLock()
	s := r.states[streamID]
	r.mu.RUnlock()
	if s == nil {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.windowStart, s.sequence, s.segCount, s.targetDuration, true
}
