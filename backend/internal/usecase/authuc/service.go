package authuc

import (
	"context"

	"golang.org/x/crypto/bcrypt"
	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/port"
)

type AuthService struct {
	userRepo port.UserRepository
	tokenSvc port.TokenService
}

func NewService(ur port.UserRepository, ts port.TokenService) *AuthService {
	return &AuthService{userRepo: ur, tokenSvc: ts}
}

func (svc *AuthService) Register(ctx context.Context, name, email, password string) error {
	if name == "" || email == "" || password == "" {
		return domain.ErrMissingFields
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return svc.userRepo.Create(ctx, name, email, string(hash))
}

func (svc *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := svc.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return "", domain.ErrInvalidCreds
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrInvalidCreds
	}
	return svc.tokenSvc.Generate(user.ID, user.Email)
}

func (svc *AuthService) Refresh(ctx context.Context, userID int, email string) (string, error) {
	return svc.tokenSvc.Generate(userID, email)
}
