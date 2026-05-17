package streamuc

import (
	"context"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/port"
)

const (
	windowSize = 3

	// rotationInterval matches the segment duration specified in the technical requirements.
	// Each .ts segment is exactly 10 seconds; one segment is dropped and one is added every interval.
	rotationInterval = 10 * time.Second
)

type StreamService struct {
	streamRepo  port.StreamRepository
	segmentRepo port.SegmentRepository
	mediaClient port.MediaClient
	meta        sync.Map // map[int]streamMeta — read-only after startup
	paths       sync.Map // map[int]string   — read-only after startup
}

func NewService(sr port.StreamRepository, sgr port.SegmentRepository, mc port.MediaClient) *StreamService {
	return &StreamService{
		streamRepo:  sr,
		segmentRepo: sgr,
		mediaClient: mc,
	}
}

// LoadAll seeds segments from NGINX (if not already in DB) and registers stream metadata.
// Returns the number of successfully loaded streams, non-fatal per-stream warnings, and a
// fatal error if the stream list itself could not be retrieved.
func (svc *StreamService) LoadAll(ctx context.Context) (int, []error, error) {
	streams, err := svc.streamRepo.ListActive(ctx)
	if err != nil {
		return 0, nil, err
	}
	var warnings []error
	count := 0
	for _, s := range streams {
		if err := svc.seedAndRegister(ctx, s); err != nil {
			warnings = append(warnings, fmt.Errorf("stream %d (%s): %w", s.ID, s.Title, err))
			continue
		}
		count++
	}
	return count, warnings, nil
}

func (svc *StreamService) seedAndRegister(ctx context.Context, s domain.Stream) error {
	exists, err := svc.segmentRepo.Exists(ctx, s.ID)
	if err != nil {
		return err
	}

	if !exists {
		segs, err := svc.mediaClient.FetchM3U8(ctx, s.Path)
		if err != nil {
			return fmt.Errorf("fetch m3u8: %w", err)
		}
		if err := svc.segmentRepo.BulkInsert(ctx, s.ID, segs); err != nil {
			return fmt.Errorf("seed segments: %w", err)
		}
	}

	segCount, err := svc.segmentRepo.Count(ctx, s.ID)
	if err != nil {
		return err
	}
	if segCount == 0 {
		return fmt.Errorf("no segments found for stream %d", s.ID)
	}

	maxDur, err := svc.segmentRepo.MaxDuration(ctx, s.ID)
	if err != nil {
		return err
	}

	svc.meta.Store(s.ID, streamMeta{
		segCount:       segCount,
		targetDuration: int(math.Ceil(maxDur)),
		startedAt:      s.StartedAt,
	})
	svc.paths.Store(s.ID, s.Path)

	return nil
}

func (svc *StreamService) ListAll(ctx context.Context) ([]domain.Stream, error) {
	return svc.streamRepo.ListActive(ctx)
}

// GetPlaylist builds the current 3-segment HLS window from a deterministic clock-based
// position derived from started_at. No shared mutable state — safe across restarts and
// multiple replicas: any instance computes the same window for the same stream at the same instant.
func (svc *StreamService) GetPlaylist(ctx context.Context, streamID int) (domain.PlaylistSnapshot, error) {
	val, ok := svc.meta.Load(streamID)
	if !ok {
		return domain.PlaylistSnapshot{}, domain.ErrNotFound
	}
	m := val.(streamMeta)

	rotations := int(time.Since(m.startedAt) / rotationInterval)
	windowStart := rotations % m.segCount

	positions := make([]int, windowSize)
	for i := range positions {
		positions[i] = (windowStart + i) % m.segCount
	}

	segs, err := svc.segmentRepo.GetByPositions(ctx, streamID, positions)
	if err != nil {
		return domain.PlaylistSnapshot{}, err
	}

	byPos := make(map[int]domain.Segment, len(segs))
	for _, sg := range segs {
		byPos[sg.Position] = sg
	}
	ordered := make([]domain.Segment, windowSize)
	for i, p := range positions {
		ordered[i] = byPos[p]
	}

	return domain.PlaylistSnapshot{
		Segments:       ordered,
		MediaSequence:  rotations,
		TargetDuration: m.targetDuration,
	}, nil
}

// ProxySegment streams a .ts file from NGINX to dst without buffering it in memory.
func (svc *StreamService) ProxySegment(ctx context.Context, streamID int, file string, dst io.Writer) error {
	val, ok := svc.paths.Load(streamID)
	if !ok {
		return domain.ErrNotFound
	}
	return svc.mediaClient.StreamSegment(ctx, val.(string), file, dst)
}
