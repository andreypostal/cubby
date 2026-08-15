package core

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

var (
	ErrDuplicatedEntry  = errors.New("duplicated entry")
	ErrResourceNotFound = errors.New("resource not found")
	ErrForbidden        = errors.New("forbidden")
	ErrBadRequest       = errors.New("bad request")
	ErrInvalidLogin     = errors.New("invalid username or password")
	ErrNotAllowed       = errors.New("operation not allowed for this user")
)

type ValidatorProvider struct{}

func (ValidatorProvider) Id() string {
	return "api-validator"
}

func (ValidatorProvider) New() any {
	return validator.New()
}
