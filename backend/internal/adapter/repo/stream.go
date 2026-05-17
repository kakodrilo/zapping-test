package repo

import (
	"context"
	"database/sql"

	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/port"
)

type mysqlStreamRepo struct {
	db *sql.DB
}

func NewStreamRepository(db *sql.DB) port.StreamRepository {
	return &mysqlStreamRepo{db: db}
}

func (r *mysqlStreamRepo) ListActive(ctx context.Context) ([]domain.Stream, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, title, COALESCE(description,''), segment_path, started_at
		FROM streams_metadata
		WHERE is_active = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []domain.Stream
	for rows.Next() {
		var s domain.Stream
		if err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.Path, &s.StartedAt); err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	return streams, rows.Err()
}
