package core

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newJWT builds the provider the way the module system does, through Initialize
func newJWT(t *testing.T, secret string, expiresIn time.Duration) *JWT {
	t.Helper()

	j := new(JWT)
	require.NoError(t, j.Initialize(NewContext(NewRegistry(), Config{
		JWT: JWTConfig{Secret: secret, ExpiresIn: expiresIn},
	})))

	return j
}

func TestTokenCarriesTheSessionBackToTheVerifier(t *testing.T) {
	j := newJWT(t, "a-secret", time.Hour)

	before := time.Now()
	token, expiresAt, err := j.NewToken("session-code", "alice", 42)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// The expiry handed back is what the caller stores on the session row, so it
	// has to match the one signed into the token
	assert.WithinDuration(t, before.Add(time.Hour), expiresAt, 5*time.Second)

	claims, err := j.VerifyToken(token)
	require.NoError(t, err)

	assert.Equal(t, "session-code", claims["sub"])
	assert.Equal(t, "alice", claims["username"])
	// Numbers come back through JSON, so the uid is a float on the way out
	assert.Equal(t, float64(42), claims["uid"])
	assert.Equal(t, float64(expiresAt.Unix()), claims["exp"])
}

func TestATokenSignedWithAnotherSecretIsRefused(t *testing.T) {
	token, _, err := newJWT(t, "the-real-secret", time.Hour).NewToken("code", "alice", 1)
	require.NoError(t, err)

	_, err = newJWT(t, "a-different-secret", time.Hour).VerifyToken(token)
	assert.ErrorIs(t, err, ErrFailedToParseToken)
}

func TestAnExpiredTokenIsRefused(t *testing.T) {
	j := newJWT(t, "a-secret", -time.Minute)

	token, expiresAt, err := j.NewToken("code", "alice", 1)
	require.NoError(t, err)
	assert.True(t, expiresAt.Before(time.Now()))

	_, err = j.VerifyToken(token)
	assert.Error(t, err)
}

// An unsigned token must never be accepted, otherwise anyone could mint claims
func TestAnUnsignedTokenIsRefused(t *testing.T) {
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"uid":      float64(1),
		"username": "alice",
		"sub":      "code",
		"exp":      time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = newJWT(t, "a-secret", time.Hour).VerifyToken(unsigned)
	assert.ErrorIs(t, err, ErrFailedToParseToken)
}

func TestGarbageIsRefusedInsteadOfPanicking(t *testing.T) {
	j := newJWT(t, "a-secret", time.Hour)

	for _, in := range []string{"", "not.a.token", "Bearer something"} {
		claims, err := j.VerifyToken(in)
		assert.ErrorIs(t, err, ErrFailedToParseToken, "%q should not have parsed", in)
		assert.Nil(t, claims)
	}
}

// A tampered payload breaks the signature, which is the whole point of signing it
func TestATamperedTokenIsRefused(t *testing.T) {
	j := newJWT(t, "a-secret", time.Hour)

	token, _, err := j.NewToken("code", "alice", 1)
	require.NoError(t, err)

	tampered := token[:len(token)-2] + "xy"
	_, err = j.VerifyToken(tampered)
	assert.Error(t, err)
}

func TestHashedPasswordsVerifyOnlyAgainstThemselves(t *testing.T) {
	hasher := BCryptHasher{}

	hash, err := hasher.Hash("correct horse battery staple")
	require.NoError(t, err)

	// The hash is never the password itself
	assert.NotEqual(t, "correct horse battery staple", hash)
	assert.NoError(t, hasher.Verify("correct horse battery staple", hash))
	assert.Error(t, hasher.Verify("Correct horse battery staple", hash))
	assert.Error(t, hasher.Verify("", hash))
}

// bcrypt salts every hash, so the same password never produces the same string
func TestTheSamePasswordHashesDifferentlyEveryTime(t *testing.T) {
	hasher := BCryptHasher{}

	first, err := hasher.Hash("hunter2")
	require.NoError(t, err)

	second, err := hasher.Hash("hunter2")
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.NoError(t, hasher.Verify("hunter2", first))
	assert.NoError(t, hasher.Verify("hunter2", second))
}
