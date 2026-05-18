package authuc_test

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"zapping-test-service/internal/domain"
	"zapping-test-service/internal/usecase/authuc"
)

// ---- mocks ----

type mockUserRepo struct {
	findUser  domain.User
	findErr   error
	createErr error
}

func (m *mockUserRepo) Create(_ context.Context, _, _, _ string) error {
	return m.createErr
}

func (m *mockUserRepo) FindByEmail(_ context.Context, _ string) (domain.User, error) {
	return m.findUser, m.findErr
}

type mockTokenSvc struct {
	token  string
	genErr error
}

func (m *mockTokenSvc) Generate(_ int, _ string) (string, error) {
	return m.token, m.genErr
}

func (m *mockTokenSvc) Validate(_ string) (int, string, error) {
	return 0, "", nil
}

// ---- Register ----

func TestRegister_MissingFields(t *testing.T) {
	cases := []struct {
		name, email, password string
	}{
		{"", "a@b.com", "pass"},
		{"Alice", "", "pass"},
		{"Alice", "a@b.com", ""},
	}
	svc := authuc.NewService(&mockUserRepo{}, &mockTokenSvc{})
	for _, tc := range cases {
		err := svc.Register(context.Background(), tc.name, tc.email, tc.password)
		if !errors.Is(err, domain.ErrMissingFields) {
			t.Errorf("name=%q email=%q pass=%q: want ErrMissingFields, got %v",
				tc.name, tc.email, tc.password, err)
		}
	}
}

func TestRegister_EmailTaken(t *testing.T) {
	repo := &mockUserRepo{createErr: domain.ErrEmailTaken}
	svc := authuc.NewService(repo, &mockTokenSvc{})
	err := svc.Register(context.Background(), "Alice", "alice@example.com", "password")
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Errorf("want ErrEmailTaken, got %v", err)
	}
}

func TestRegister_Success(t *testing.T) {
	svc := authuc.NewService(&mockUserRepo{}, &mockTokenSvc{})
	err := svc.Register(context.Background(), "Alice", "alice@example.com", "password")
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

// ---- Login ----

func TestLogin_UserNotFound(t *testing.T) {
	repo := &mockUserRepo{findErr: errors.New("not found")}
	svc := authuc.NewService(repo, &mockTokenSvc{token: "tok"})
	_, err := svc.Login(context.Background(), "x@x.com", "pass")
	if !errors.Is(err, domain.ErrInvalidCreds) {
		t.Errorf("want ErrInvalidCreds, got %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	repo := &mockUserRepo{findUser: domain.User{ID: 1, Email: "x@x.com", PasswordHash: string(hash)}}
	svc := authuc.NewService(repo, &mockTokenSvc{token: "tok"})
	_, err := svc.Login(context.Background(), "x@x.com", "wrong")
	if !errors.Is(err, domain.ErrInvalidCreds) {
		t.Errorf("want ErrInvalidCreds, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	repo := &mockUserRepo{findUser: domain.User{ID: 1, Email: "x@x.com", PasswordHash: string(hash)}}
	svc := authuc.NewService(repo, &mockTokenSvc{token: "my-token"})
	tok, err := svc.Login(context.Background(), "x@x.com", "secret")
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if tok != "my-token" {
		t.Errorf("want 'my-token', got %q", tok)
	}
}

// ---- Refresh ----

func TestRefresh_Success(t *testing.T) {
	svc := authuc.NewService(&mockUserRepo{}, &mockTokenSvc{token: "refreshed"})
	tok, err := svc.Refresh(context.Background(), 42, "x@x.com")
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if tok != "refreshed" {
		t.Errorf("want 'refreshed', got %q", tok)
	}
}

func TestRefresh_TokenServiceError(t *testing.T) {
	genErr := errors.New("signing failed")
	svc := authuc.NewService(&mockUserRepo{}, &mockTokenSvc{genErr: genErr})
	_, err := svc.Refresh(context.Background(), 1, "x@x.com")
	if !errors.Is(err, genErr) {
		t.Errorf("want signing error, got %v", err)
	}
}
