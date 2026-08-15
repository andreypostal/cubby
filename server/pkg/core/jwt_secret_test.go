package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newJWTIn builds the provider against a data folder, which is what decides
// where a generated secret is kept
func newJWTIn(t *testing.T, dataDir, secret string) *JWT {
	t.Helper()

	j := new(JWT)
	require.NoError(t, j.Initialize(NewContext(NewRegistry(), Config{
		DataDir: dataDir,
		JWT:     JWTConfig{Secret: secret, ExpiresIn: time.Hour},
	})))

	return j
}

// A secret somebody configured is theirs to manage, the server neither replaces
// it nor writes anything of its own
func TestAConfiguredSecretIsUsedAsItIs(t *testing.T) {
	dataDir := t.TempDir()

	j := newJWTIn(t, dataDir, "a-secret-i-manage-myself")

	assert.Equal(t, "a-secret-i-manage-myself", j.secret)
	assert.NoFileExists(t, filepath.Join(dataDir, JWTSecretFile))
}

// A deployment that configured nothing still has to sign with something nobody
// else can guess, so the server generates one and keeps it
func TestAnEmptySecretIsGeneratedIntoTheDataFolder(t *testing.T) {
	dataDir := t.TempDir()

	j := newJWTIn(t, dataDir, "")

	path := filepath.Join(dataDir, JWTSecretFile)
	require.FileExists(t, path)

	stored, err := os.ReadFile(path)
	require.NoError(t, err)

	// 32 random bytes written as hex
	assert.Len(t, string(stored), secretBytes*2)
	assert.Equal(t, string(stored), j.secret)
}

// The file holds the key to every session on the server, nobody else on the box
// has any business reading it
func TestTheGeneratedSecretIsNotReadableByAnyoneElse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits do not carry over to windows")
	}

	dataDir := t.TempDir()
	newJWTIn(t, dataDir, "")

	info, err := os.Stat(filepath.Join(dataDir, JWTSecretFile))
	require.NoError(t, err)

	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

// Restarting a server must not sign everyone out, so the stored secret is read
// back rather than rolled
func TestARestartReusesTheStoredSecret(t *testing.T) {
	dataDir := t.TempDir()

	first := newJWTIn(t, dataDir, "")
	second := newJWTIn(t, dataDir, "")

	assert.Equal(t, first.secret, second.secret)

	// A token minted before the restart still verifies after it
	token, _, err := first.NewToken("code", "alice", 1)
	require.NoError(t, err)

	_, err = second.VerifyToken(token)
	assert.NoError(t, err)
}

// A file left empty by a failed write earlier signs nothing, it gets replaced
func TestAnEmptySecretFileIsReplaced(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, JWTSecretFile)

	require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o600))

	j := newJWTIn(t, dataDir, "")

	assert.Len(t, j.secret, secretBytes*2)
}

// Two servers generating their own secrets cannot be talked into accepting each
// other's tokens
func TestEachServerGeneratesItsOwnSecret(t *testing.T) {
	one := newJWTIn(t, t.TempDir(), "")
	other := newJWTIn(t, t.TempDir(), "")

	require.NotEqual(t, one.secret, other.secret)

	token, _, err := one.NewToken("code", "alice", 1)
	require.NoError(t, err)

	_, err = other.VerifyToken(token)
	assert.Error(t, err)
}
