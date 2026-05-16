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
	windowSize       = 3
	rotationInterval = 10 * time.Second
)

type StreamService struct {
	streamRepo  port.StreamRepository
	segmentRepo port.SegmentRepository
	mediaClient port.MediaClient
	state       *StateRegistry
	paths       sync.Map // map[int]string: streamID → segment_path (read-only after startup)
}

func NewService(sr port.StreamRepository, sgr port.SegmentRepository, mc port.MediaClient) *StreamService {
	return &StreamService{
		streamRepo:  sr,
		segmentRepo: sgr,
		mediaClient: mc,
		state:       newStateRegistry(),
	}
}

// LoadAll seeds segments from NGINX (if not already in DB) and starts rotation goroutines.
func (svc *StreamService) LoadAll(ctx context.Context) (int, error) {
	streams, err := svc.streamRepo.ListActive(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, s := range streams {
		if err := svc.seedAndStart(ctx, s); err != nil {
			fmt.Printf("warning: skipping stream %d (%s): %v\n", s.ID, s.Title, err)
			continue
		}
		count++
	}
	return count, nil
}

func (svc *StreamService) seedAndStart(ctx context.Context, s domain.Stream) error {
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

	count, err := svc.segmentRepo.Count(ctx, s.ID)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no segments found")
	}

	maxDur, err := svc.segmentRepo.MaxDuration(ctx, s.ID)
	if err != nil {
		return err
	}

	svc.state.init(s.ID, count, int(math.Ceil(maxDur)), s.InitialOffset)
	svc.paths.Store(s.ID, s.Path)

	go func() {
		ticker := time.NewTicker(rotationInterval)
		defer ticker.Stop()
		for range ticker.C {
			svc.state.advance(s.ID)
		}
	}()

	return nil
}

func (svc *StreamService) ListAll(ctx context.Context) ([]domain.Stream, error) {
	return svc.streamRepo.ListActive(ctx)
}

// GetPlaylist builds the current 3-segment HLS window by querying the DB for exactly
// those positions. Memory usage is O(windowSize), not O(total segments).
func (svc *StreamService) GetPlaylist(ctx context.Context, streamID int) (domain.PlaylistSnapshot, error) {
	start, seq, count, targetDur, ok := svc.state.snapshot(streamID)
	if !ok {
		return domain.PlaylistSnapshot{}, domain.ErrNotFound
	}

	positions := make([]int, windowSize)
	for i := range positions {
		positions[i] = (start + i) % count
	}

	segs, err := svc.segmentRepo.GetByPositions(ctx, streamID, positions)
	if err != nil {
		return domain.PlaylistSnapshot{}, err
	}

	// Re-order to match the window order (DB may not preserve it).
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
		MediaSequence:  seq,
		TargetDuration: targetDur,
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
