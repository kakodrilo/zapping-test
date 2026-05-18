package streamuc_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/usecase/streamuc"
)

// ---- mocks ----

type mockStreamRepo struct {
	streams []domain.Stream
	err     error
}

func (m *mockStreamRepo) ListActive(_ context.Context) ([]domain.Stream, error) {
	return m.streams, m.err
}

type mockSegmentRepo struct {
	existsResult bool
	existsErr    error
	count        int
	countErr     error
	maxDuration  float64
	maxDurErr    error
	bulkErr      error
	segments     []domain.Segment
	getErr       error
}

func (m *mockSegmentRepo) Exists(_ context.Context, _ int) (bool, error) {
	return m.existsResult, m.existsErr
}

func (m *mockSegmentRepo) Count(_ context.Context, _ int) (int, error) {
	return m.count, m.countErr
}

func (m *mockSegmentRepo) MaxDuration(_ context.Context, _ int) (float64, error) {
	return m.maxDuration, m.maxDurErr
}

func (m *mockSegmentRepo) BulkInsert(_ context.Context, _ int, _ []domain.Segment) error {
	return m.bulkErr
}

func (m *mockSegmentRepo) GetByPositions(_ context.Context, _ int, positions []int) ([]domain.Segment, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	posSet := make(map[int]struct{}, len(positions))
	for _, p := range positions {
		posSet[p] = struct{}{}
	}
	var result []domain.Segment
	for _, s := range m.segments {
		if _, ok := posSet[s.Position]; ok {
			result = append(result, s)
		}
	}
	return result, nil
}

type mockMediaClient struct {
	fetchSegments []domain.Segment
	fetchErr      error
	streamErr     error
}

func (m *mockMediaClient) FetchM3U8(_ context.Context, _ string) ([]domain.Segment, error) {
	return m.fetchSegments, m.fetchErr
}

func (m *mockMediaClient) StreamSegment(_ context.Context, _, _ string, _ io.Writer) error {
	return m.streamErr
}

// ---- helpers ----

func threeSegments() []domain.Segment {
	return []domain.Segment{
		{Name: "seg0.ts", Duration: 10.0, Position: 0},
		{Name: "seg1.ts", Duration: 10.0, Position: 1},
		{Name: "seg2.ts", Duration: 10.0, Position: 2},
	}
}

// loadedService builds a StreamService with one pre-loaded stream (ID=1).
// startedAt controls the HLS window position in GetPlaylist tests.
func loadedService(t *testing.T, startedAt time.Time) (*streamuc.StreamService, *mockSegmentRepo) {
	t.Helper()
	streams := []domain.Stream{{ID: 1, Title: "Live", Path: "/hls/live", StartedAt: startedAt}}
	segRepo := &mockSegmentRepo{
		existsResult: true,
		count:        3,
		maxDuration:  10.0,
		segments:     threeSegments(),
	}
	svc := streamuc.NewService(&mockStreamRepo{streams: streams}, segRepo, &mockMediaClient{})
	if _, _, err := svc.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll setup: %v", err)
	}
	return svc, segRepo
}

// ---- LoadAll ----

func TestLoadAll_RepoError(t *testing.T) {
	repoErr := errors.New("db down")
	svc := streamuc.NewService(&mockStreamRepo{err: repoErr}, &mockSegmentRepo{}, &mockMediaClient{})
	count, warnings, err := svc.LoadAll(context.Background())
	if !errors.Is(err, repoErr) {
		t.Errorf("want repoErr, got %v", err)
	}
	if count != 0 || len(warnings) != 0 {
		t.Errorf("want count=0 warnings=nil, got count=%d warnings=%v", count, warnings)
	}
}

func TestLoadAll_AllStreamsLoaded(t *testing.T) {
	streams := []domain.Stream{
		{ID: 1, Title: "A", Path: "/a", StartedAt: time.Now()},
		{ID: 2, Title: "B", Path: "/b", StartedAt: time.Now()},
	}
	segRepo := &mockSegmentRepo{existsResult: true, count: 3, maxDuration: 10.0}
	svc := streamuc.NewService(&mockStreamRepo{streams: streams}, segRepo, &mockMediaClient{})
	count, warnings, err := svc.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if count != 2 {
		t.Errorf("want count=2, got %d", count)
	}
	if len(warnings) != 0 {
		t.Errorf("want no warnings, got %v", warnings)
	}
}

func TestLoadAll_PerStreamWarning(t *testing.T) {
	streams := []domain.Stream{{ID: 1, Title: "Bad", Path: "/bad", StartedAt: time.Now()}}
	segRepo := &mockSegmentRepo{existsErr: errors.New("segment repo unavailable")}
	svc := streamuc.NewService(&mockStreamRepo{streams: streams}, segRepo, &mockMediaClient{})
	count, warnings, err := svc.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected fatal error: %v", err)
	}
	if count != 0 {
		t.Errorf("want count=0, got %d", count)
	}
	if len(warnings) != 1 {
		t.Errorf("want 1 warning, got %d", len(warnings))
	}
}

func TestLoadAll_SeedsFromMediaWhenSegmentsAbsent(t *testing.T) {
	segs := threeSegments()
	streams := []domain.Stream{{ID: 1, Title: "A", Path: "/hls/a", StartedAt: time.Now()}}
	segRepo := &mockSegmentRepo{
		existsResult: false,
		count:        3,
		maxDuration:  10.0,
	}
	mc := &mockMediaClient{fetchSegments: segs}
	svc := streamuc.NewService(&mockStreamRepo{streams: streams}, segRepo, mc)
	count, warnings, err := svc.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if count != 1 || len(warnings) != 0 {
		t.Errorf("want count=1 warnings=nil, got count=%d warnings=%v", count, warnings)
	}
}

// ---- ListAll ----

func TestListAll_Success(t *testing.T) {
	want := []domain.Stream{{ID: 1, Title: "Live"}, {ID: 2, Title: "Sports"}}
	svc := streamuc.NewService(&mockStreamRepo{streams: want}, &mockSegmentRepo{}, &mockMediaClient{})
	got, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(got) != len(want) {
		t.Errorf("want %d streams, got %d", len(want), len(got))
	}
}

func TestListAll_RepoError(t *testing.T) {
	repoErr := errors.New("db unavailable")
	svc := streamuc.NewService(&mockStreamRepo{err: repoErr}, &mockSegmentRepo{}, &mockMediaClient{})
	_, err := svc.ListAll(context.Background())
	if !errors.Is(err, repoErr) {
		t.Errorf("want repoErr, got %v", err)
	}
}

// ---- GetPlaylist ----

func TestGetPlaylist_NotFound(t *testing.T) {
	svc := streamuc.NewService(&mockStreamRepo{}, &mockSegmentRepo{}, &mockMediaClient{})
	_, err := svc.GetPlaylist(context.Background(), 99)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestGetPlaylist_Success(t *testing.T) {
	// startedAt = now means 0 rotations elapsed → window positions [0,1,2]
	svc, _ := loadedService(t, time.Now())
	snap, err := svc.GetPlaylist(context.Background(), 1)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(snap.Segments) != 3 {
		t.Errorf("want 3 segments, got %d", len(snap.Segments))
	}
	if snap.TargetDuration != 10 {
		t.Errorf("want TargetDuration=10, got %d", snap.TargetDuration)
	}
	if snap.MediaSequence != 0 {
		t.Errorf("want MediaSequence=0, got %d", snap.MediaSequence)
	}
}

func TestGetPlaylist_WindowRotates(t *testing.T) {
	// 25 seconds elapsed → rotations=2, windowStart=2%3=2 → positions [2,0,1]
	startedAt := time.Now().Add(-25 * time.Second)
	svc, _ := loadedService(t, startedAt)
	snap, err := svc.GetPlaylist(context.Background(), 1)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if len(snap.Segments) != 3 {
		t.Errorf("want 3 segments, got %d", len(snap.Segments))
	}
	if snap.MediaSequence < 2 {
		t.Errorf("want MediaSequence>=2, got %d", snap.MediaSequence)
	}
	if snap.Segments[0].Name != "seg2.ts" {
		t.Errorf("want seg2.ts as first segment, got %s", snap.Segments[0].Name)
	}
}

func TestGetPlaylist_SegmentRepoError(t *testing.T) {
	svc, segRepo := loadedService(t, time.Now())
	segRepo.getErr = errors.New("query failed")
	_, err := svc.GetPlaylist(context.Background(), 1)
	if err == nil {
		t.Error("want error, got nil")
	}
}

// ---- ProxySegment ----

func TestProxySegment_NotFound(t *testing.T) {
	svc := streamuc.NewService(&mockStreamRepo{}, &mockSegmentRepo{}, &mockMediaClient{})
	err := svc.ProxySegment(context.Background(), 99, "seg.ts", &bytes.Buffer{})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestProxySegment_Success(t *testing.T) {
	svc, _ := loadedService(t, time.Now())
	err := svc.ProxySegment(context.Background(), 1, "seg0.ts", &bytes.Buffer{})
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestProxySegment_MediaClientError(t *testing.T) {
	streams := []domain.Stream{{ID: 1, Title: "A", Path: "/hls/a", StartedAt: time.Now()}}
	segRepo := &mockSegmentRepo{existsResult: true, count: 3, maxDuration: 10.0}
	streamErr := errors.New("NGINX unavailable")
	mc := &mockMediaClient{streamErr: streamErr}
	svc := streamuc.NewService(&mockStreamRepo{streams: streams}, segRepo, mc)
	if _, _, err := svc.LoadAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	err := svc.ProxySegment(context.Background(), 1, "seg.ts", &bytes.Buffer{})
	if !errors.Is(err, streamErr) {
		t.Errorf("want NGINX error, got %v", err)
	}
}
