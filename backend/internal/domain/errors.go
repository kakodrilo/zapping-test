package domain

import "errors"

var (
	ErrEmailTaken   = errors.New("email already registered")
	ErrInvalidCreds = errors.New("invalid credentials")
	ErrMissingFields = errors.New("name, email and password are required")
	ErrNotFound     = errors.New("not found")
)
