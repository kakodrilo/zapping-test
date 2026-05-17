package jwtadapter

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"zapping-test-service/internal/port"
)

type jwtService struct {
	secret []byte
}

func NewService(secret string) port.TokenService {
	return &jwtService{secret: []byte(secret)}
}

type claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func (s *jwtService) Generate(userID int, email string) (string, error) {
	c := claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
}

func (s *jwtService) Validate(tokenStr string) (int, string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return 0, "", err
	}
	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return 0, "", errors.New("invalid token")
	}
	return c.UserID, c.Email, nil
}
