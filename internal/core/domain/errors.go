package domain

import "errors"

// Common domain errors
var (
	ErrNotFound          = errors.New("resource not found")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInternalServer    = errors.New("internal server error")
	ErrDuplicateEntry    = errors.New("duplicate entry")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired      = errors.New("token expired")
	ErrTokenInvalid      = errors.New("token invalid")
)

// UserErrors
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrMemberNotFound    = errors.New("member not found in flommast")
)

// LoanErrors (Phase 4)
var (
	ErrLoanNotFound      = errors.New("loan not found")
	ErrLoanAlreadyExists = errors.New("loan already exists")
	ErrInvalidLoanStatus = errors.New("invalid loan status")
)
