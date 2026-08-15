package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) error
}

type BCryptHasher struct {
	PasswordHasher
}

func (BCryptHasher) Id() string {
	return "hasher"
}

func (BCryptHasher) New() any {
	return new(BCryptHasher)
}

func (BCryptHasher) Hash(pass string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	return string(h), err
}

func (BCryptHasher) Verify(pass, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass))
}

// -------------------------------------------

var (
	ErrFailedToParseToken = errors.New("failed to parse token")
	ErrInvalidToken       = errors.New("invalid auth token received")
	ErrExpiredToken       = errors.New("auth token is expired")
)

// JWTSecretFile is the file, relative to the data folder, holding the secret
// this server generated for itself. It belongs to the deployment rather than to
// the data, so the backup archives leave it out.
const JWTSecretFile = "jwt.secret"

// secretBytes is how much randomness a generated secret carries
const secretBytes = 32

// resolveSecret answers the secret to sign with: the configured one when there
// is a real one, otherwise the one kept in the data folder, generating and
// storing it the first time.
func resolveSecret(config Config) (string, error) {
	if configured := strings.TrimSpace(config.JWT.Secret); configured != "" {
		return configured, nil
	}

	path := filepath.Join(config.DataDir, JWTSecretFile)

	// #nosec G304 -- the path is the data folder from the config joined with a
	// constant name, nothing a request can reach
	switch stored, err := os.ReadFile(path); {
	case err == nil:
		if secret := strings.TrimSpace(string(stored)); secret != "" {
			return secret, nil
		}
		// An empty file is worthless, it gets overwritten below
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("reading the stored JWT secret %q: %w", path, err)
	}

	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating a JWT secret: %w", err)
	}
	secret := hex.EncodeToString(buf)

	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		return "", fmt.Errorf("storing the JWT secret in %q: %w", path, err)
	}

	slog.Info("Generated a new JWT signing secret",
		slog.String("file", path),
		slog.String("note", "every session issued with a previous secret is now invalid"),
	)

	return secret, nil
}

type JwtProvider interface {
	NewToken(code, username string, userId uint) (string, time.Time, error)
	VerifyToken(tokenString string) (map[string]any, error)
}

type JWT struct {
	expiresIn time.Duration
	secret    string
}

func (j *JWT) Id() string {
	return "jwt"
}

func (j *JWT) New() any {
	return new(JWT)
}

func (j *JWT) Initialize(ctx *Context) error {
	j.expiresIn = ctx.Config.JWT.ExpiresIn

	secret, err := resolveSecret(ctx.Config)
	if err != nil {
		return err
	}
	j.secret = secret

	return nil
}

func (j *JWT) NewToken(code, username string, userId uint) (string, time.Time, error) {
	expiresAt := time.Now().Add(j.expiresIn)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":      userId,
		"username": username,
		"sub":      code,
		"exp":      expiresAt.Unix(),
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(j.secret))
	return tokenString, expiresAt, err
}

// VerifyToken validates signature, expiration date, format and returns a map with claims
func (j *JWT) VerifyToken(tokenString string) (map[string]any, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrFailedToParseToken
		}
		return []byte(j.secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToParseToken, err)
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to map claims: %w", ErrInvalidToken)
	}

	if float64(time.Now().Unix()) > claims["exp"].(float64) {
		return nil, ErrExpiredToken
	}

	return claims, nil
}
