package core

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// Error is a custom error that exposes information in the API
type Error struct {
	Message    string         `json:"message"`
	Details    map[string]any `json:"details,omitempty"`
	httpStatus int
}

func NewError(status int, msg string, details map[string]any) Error {
	return Error{
		Message:    msg,
		Details:    details,
		httpStatus: status,
	}
}

func RespondError(c *gin.Context, err Error) {
	c.JSON(err.httpStatus, err)
}

// ------------------------------------

type contextKey string

const (
	profileKey      contextKey = "profile"
	organizationKey contextKey = "organization"
	loggerKey       contextKey = "logger"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

type ProfileUser struct {
	ID            uint
	Username      string
	Name          string
	IsSystemAdmin bool
}

type ProfileWorkspace struct {
	ID   uint
	Name string
	// IsCreator separates the user who owns the workspace from the ones it was
	// shared with
	IsCreator bool
}

type Profile struct {
	SessionCode string
	User        ProfileUser
	Workspace   *ProfileWorkspace
}

func GetProfile(c *gin.Context) (Profile, bool) {
	p, ok := c.Get(profileKey)
	if !ok {
		return Profile{}, false
	}
	return p.(Profile), true
}

func SetProfile(c *gin.Context, profile Profile) {
	c.Set(profileKey, profile)
}

func FromStringToUint(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 0)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}

func FromUintToString(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}

// ----------------------------------------------
// ValidationError maps go-playground/validator field errors into a core.Error
// with a details map so the client knows exactly which fields failed.
func ValidationError(err error) Error {
	details := make(map[string]any)

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			details[fe.Field()] = fieldMessage(fe)
		}
	}

	return NewError(http.StatusUnprocessableEntity, "validation failed", details)
}

// fieldMessage returns a human-readable message for a validation failure.
func fieldMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fe.Field() + " is required"
	case "email":
		return fe.Field() + " must be a valid email address"
	case "min":
		return fe.Field() + " must be at least " + fe.Param() + " characters"
	case "max":
		return fe.Field() + " must be at most " + fe.Param() + " characters"
	case "url":
		return fe.Field() + " must be a valid URL"
	case "uuid":
		return fe.Field() + " must be a valid UUID"
	case "alphanum":
		return fe.Field() + " must contain only letters and numbers"
	case "oneof":
		return fe.Field() + " must be one of: " + fe.Param()
	default:
		return fe.Field() + " is invalid"
	}
}
