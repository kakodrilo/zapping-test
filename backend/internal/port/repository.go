package port

import (
	"context"

	"zapping-test-service/internal/domain"
)

type StreamRepository interface {
	ListActive(ctx context.Context) ([]domain.Stream, error)
}

type SegmentRepository interface {
	Exists(ctx context.Context, streamID int) (bool, error)
	Count(ctx context.Context, streamID int) (int, error)
	MaxDuration(ctx context.Context, streamID int) (float64, error)
	BulkInsert(ctx context.Context, streamID int, segments []domain.Segment) error
	GetByPositions(ctx context.Context, streamID int, positions []int) ([]domain.Segment, error)
}

type UserRepository interface {
	Create(ctx context.Context, name, email, passwordHash string) error
	FindByEmail(ctx context.Context, email string) (domain.User, error)
}
