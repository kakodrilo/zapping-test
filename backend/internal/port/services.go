package port

import (
	"context"
	"io"

	"zapping-test-service/internal/domain"
)

type TokenService interface {
	Generate(userID int, email string) (string, error)
	Validate(token string) (userID int, email string, err error)
}

type MediaClient interface {
	FetchM3U8(ctx context.Context, path string) ([]domain.Segment, error)
	StreamSegment(ctx context.Context, path, name string, dst io.Writer) error
}
