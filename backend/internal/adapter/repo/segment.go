package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/port"
)

type mysqlSegmentRepo struct {
	db *sql.DB
}

func NewSegmentRepository(db *sql.DB) port.SegmentRepository {
	return &mysqlSegmentRepo{db: db}
}

func (r *mysqlSegmentRepo) Exists(ctx context.Context, streamID int) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stream_segments WHERE stream_id = ? LIMIT 1", streamID,
	).Scan(&count)
	return count > 0, err
}

func (r *mysqlSegmentRepo) Count(ctx context.Context, streamID int) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM stream_segments WHERE stream_id = ?", streamID,
	).Scan(&count)
	return count, err
}

func (r *mysqlSegmentRepo) MaxDuration(ctx context.Context, streamID int) (float64, error) {
	var maxDur float64
	err := r.db.QueryRowContext(ctx,
		"SELECT MAX(duration) FROM stream_segments WHERE stream_id = ?", streamID,
	).Scan(&maxDur)
	return maxDur, err
}

func (r *mysqlSegmentRepo) BulkInsert(ctx context.Context, streamID int, segments []domain.Segment) error {
	if len(segments) == 0 {
		return nil
	}
	placeholders := strings.Repeat("(?,?,?,?),", len(segments))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, 0, len(segments)*4)
	for _, s := range segments {
		args = append(args, streamID, s.Name, s.Duration, s.Position)
	}

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO stream_segments (stream_id, name, duration, position) VALUES "+placeholders,
		args...,
	)
	return err
}

// GetByPositions fetches exactly the segments at the given positions.
// Positions may wrap around (e.g. [62, 63, 0]) — the caller re-orders by window index.
func (r *mysqlSegmentRepo) GetByPositions(ctx context.Context, streamID int, positions []int) ([]domain.Segment, error) {
	if len(positions) == 0 {
		return nil, nil
	}

	ph := strings.Repeat("?,", len(positions))
	ph = ph[:len(ph)-1]
	query := fmt.Sprintf(
		"SELECT name, duration, position FROM stream_segments WHERE stream_id = ? AND position IN (%s)",
		ph,
	)

	args := make([]any, 0, 1+len(positions))
	args = append(args, streamID)
	for _, p := range positions {
		args = append(args, p)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segs []domain.Segment
	for rows.Next() {
		var s domain.Segment
		if err := rows.Scan(&s.Name, &s.Duration, &s.Position); err != nil {
			return nil, err
		}
		segs = append(segs, s)
	}
	return segs, rows.Err()
}
