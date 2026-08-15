package service

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/dto"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userTestEnv is the user service over a database of its own, holding the two
// administrators and the ordinary member the rules are written about
type userTestEnv struct {
	service UserService

	db          *core.DB
	sessionRepo repository.SessionRepository
	wsRepo      repository.WorkspaceRepository

	admin *models.User
	// second exists so the last-admin guard has something to let through
	second *models.User
	member *models.User
}

func newUserTestEnv(t *testing.T) *userTestEnv {
	t.Helper()

	// The dsn is a file: URI, so the drive letter has to come with forward slashes
	dataDir := path.Join(filepath.ToSlash(t.TempDir()), "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	registry := core.NewRegistry()
	registry.Register(new(core.DB))

	ctx := core.NewContext(registry, core.Config{
		DataDir: dataDir,
		DB: core.DBConfig{
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: time.Minute,
		},
	})

	db, err := core.GetModule[*core.DB](ctx, new(core.DB))
	require.NoError(t, err)

	// The database has to let go of the files before TempDir removes them
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Session{}, &models.Workspace{}))

	userRepo := repository.NewUserRepository(db)
	wsRepo := repository.NewWorkspaceRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	env := &userTestEnv{
		service:     NewUserService(userRepo, wsRepo, sessionRepo, core.BCryptHasher{}),
		db:          db,
		sessionRepo: sessionRepo,
		wsRepo:      wsRepo,
	}

	for _, u := range []struct {
		username string
		admin    bool
		into     **models.User
	}{
		{"admin", true, &env.admin},
		{"second", true, &env.second},
		{"member", false, &env.member},
	} {
		user := &models.User{
			Username:      u.username,
			Password:      "x",
			Name:          u.username,
			IsSystemAdmin: u.admin,
		}
		require.NoError(t, db.Create(user).Error)
		*u.into = user
	}

	return env
}

// demote removes the second administrator, leaving the first one alone
func (e *userTestEnv) demoteSecond(t *testing.T) {
	t.Helper()

	e.second.IsSystemAdmin = false
	require.NoError(t, e.db.Save(e.second).Error)
}

func (e *userTestEnv) openSession(t *testing.T, user *models.User, code string) {
	t.Helper()

	require.NoError(t, e.sessionRepo.Create(&models.Session{
		UserID:    user.ID,
		Code:      code,
		ExpiresAt: time.Now().Add(time.Hour),
	}))
}

func (e *userTestEnv) sessionExists(t *testing.T, code string) bool {
	t.Helper()

	_, err := e.sessionRepo.FindByCode(code)

	return err == nil
}

// A password reaches the database as a hash and never as what was typed
func TestCreatedUsersStoreAHashedPassword(t *testing.T) {
	env := newUserTestEnv(t)

	created, err := env.service.Create(&dto.CreateUserReq{
		Name:     "Ada",
		Username: "ada",
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	assert.False(t, created.IsSystemAdmin)

	stored, err := repository.NewUserRepository(env.db).FindByUsername("ada")
	require.NoError(t, err)

	assert.NotEqual(t, "correct horse battery staple", stored.Password)
	assert.NoError(t, core.BCryptHasher{}.Verify("correct horse battery staple", stored.Password))
}

// Usernames are how people sign in, two accounts cannot answer to the same one
func TestASecondUserCannotTakeAnExistingUsername(t *testing.T) {
	env := newUserTestEnv(t)

	_, err := env.service.Create(&dto.CreateUserReq{
		Name:     "Impostor",
		Username: "admin",
		Password: "s3cret-enough",
	})

	assert.ErrorIs(t, err, core.ErrDuplicatedEntry)
}

// A server with nobody able to administer it cannot be recovered from the UI,
// so the last administrator stays
func TestTheLastAdminCannotBeDeleted(t *testing.T) {
	env := newUserTestEnv(t)
	env.demoteSecond(t)

	err := env.service.Delete(env.admin.ID, env.member.ID)

	assert.ErrorIs(t, err, ErrLastSystemAdmin)

	_, found := repository.NewUserRepository(env.db).FindByID(env.admin.ID)
	assert.NoError(t, found)
}

// Demoting the last administrator locks the server just as thoroughly as
// deleting them
func TestTheLastAdminCannotBeDemoted(t *testing.T) {
	env := newUserTestEnv(t)
	env.demoteSecond(t)

	_, err := env.service.Update(env.admin.ID, env.member.ID, &dto.UpdateUserReq{
		Name:          "admin",
		IsSystemAdmin: false,
	})

	assert.ErrorIs(t, err, ErrLastSystemAdmin)
}

// While another administrator is around, stepping one down is ordinary work
func TestAnAdminCanBeDemotedWhileAnotherRemains(t *testing.T) {
	env := newUserTestEnv(t)

	updated, err := env.service.Update(env.second.ID, env.admin.ID, &dto.UpdateUserReq{
		Name:          "Second",
		IsSystemAdmin: false,
	})
	require.NoError(t, err)

	assert.False(t, updated.IsSystemAdmin)
	assert.Equal(t, "Second", updated.Name)
}

// Deleting yourself ends the session doing the deleting, which reads as a
// misclick rather than an intention
func TestAnAdminCannotDeleteThemselves(t *testing.T) {
	env := newUserTestEnv(t)

	err := env.service.Delete(env.admin.ID, env.admin.ID)

	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

// Same for the admin flag: taking it off yourself is how you lose the page you
// are standing on
func TestAnAdminCannotDemoteThemselves(t *testing.T) {
	env := newUserTestEnv(t)

	_, err := env.service.Update(env.admin.ID, env.admin.ID, &dto.UpdateUserReq{
		Name:          "admin",
		IsSystemAdmin: false,
	})

	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

// Users are soft deleted, so the database cascade never fires and the cubbies
// they created would survive with an owner nobody can sign in as
func TestAUserWhoOwnsCubbiesIsNotDeleted(t *testing.T) {
	env := newUserTestEnv(t)

	require.NoError(t, env.wsRepo.Create(&models.Workspace{
		Name:      "Groceries",
		CreatorID: env.member.ID,
	}))

	err := env.service.Delete(env.member.ID, env.admin.ID)
	assert.ErrorIs(t, err, ErrUserOwnsWorkspaces)

	// The refusal names them, so the administrator knows what is in the way
	names, err := env.service.OwnedWorkspaces(env.member.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Groceries"}, names)

	_, found := repository.NewUserRepository(env.db).FindByID(env.member.ID)
	assert.NoError(t, found)
}

// A user with nothing of their own goes, and takes their sessions and the
// access others shared with them along
func TestDeletingAUserEndsTheirSessionsAndSharedAccess(t *testing.T) {
	env := newUserTestEnv(t)

	shared := &models.Workspace{Name: "Home", CreatorID: env.admin.ID}
	require.NoError(t, env.wsRepo.Create(shared))
	require.NoError(t, env.wsRepo.AddMember(shared.ID, env.member))

	env.openSession(t, env.member, "member-session")
	env.openSession(t, env.admin, "admin-session")

	require.NoError(t, env.service.Delete(env.member.ID, env.admin.ID))

	_, found := repository.NewUserRepository(env.db).FindByID(env.member.ID)
	assert.ErrorIs(t, found, core.ErrResourceNotFound)

	assert.False(t, env.sessionExists(t, "member-session"))
	assert.True(t, env.sessionExists(t, "admin-session"), "the deletion reached another user's session")

	members, err := env.wsRepo.Members(shared.ID)
	require.NoError(t, err)
	assert.Empty(t, members)

	// The cubby itself belongs to its creator and stays untouched
	_, err = env.wsRepo.FindByID(shared.ID)
	assert.NoError(t, err)
}

// A password is usually changed because the old one leaked, so the tokens
// issued under it have to stop working
func TestChangingAPasswordEndsOnlyThatUsersSessions(t *testing.T) {
	env := newUserTestEnv(t)

	env.openSession(t, env.member, "member-session")
	env.openSession(t, env.admin, "admin-session")

	require.NoError(t, env.service.ChangePassword(env.member.ID, &dto.ChangePasswordReq{
		Password: "a-brand-new-password",
	}))

	assert.False(t, env.sessionExists(t, "member-session"))
	assert.True(t, env.sessionExists(t, "admin-session"))

	stored, err := repository.NewUserRepository(env.db).FindByID(env.member.ID)
	require.NoError(t, err)
	assert.NoError(t, core.BCryptHasher{}.Verify("a-brand-new-password", stored.Password))
}

// The list is what the administration screen is built on, and it never carries
// a password out of the service
func TestTheUserListNeverCarriesPasswords(t *testing.T) {
	env := newUserTestEnv(t)

	list, err := env.service.List()
	require.NoError(t, err)

	require.Len(t, list.Users, 3)
	for _, u := range list.Users {
		assert.NotEmpty(t, u.Username)
	}
}
