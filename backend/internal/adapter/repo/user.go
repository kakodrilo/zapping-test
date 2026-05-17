package repo

import (
	"context"
	"database/sql"
	"strings"

	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/port"
)

type mysqlUserRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) port.UserRepository {
	return &mysqlUserRepo{db: db}
}

func (r *mysqlUserRepo) Create(ctx context.Context, name, email, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, "CALL sp_register_user(?,?,?)", name, email, passwordHash)
	if err != nil && strings.Contains(err.Error(), "ya se encuentra registrado") {
		return domain.ErrEmailTaken
	}
	return err
}

func (r *mysqlUserRepo) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRowContext(ctx, "CALL sp_get_user_by_email(?)", email).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash)
	if err == sql.ErrNoRows {
		return domain.User{}, domain.ErrInvalidCreds
	}
	return u, err
}
