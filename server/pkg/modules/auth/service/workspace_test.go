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

// wsTestEnv is the workspace service over a database of its own, with the two
// users every sharing rule is written about
type wsTestEnv struct {
	service WorkspaceService

	creator *models.User
	guest   *models.User
	// stranger has nothing shared with them, and proves the refusals
	stranger *models.User
}

func newWorkspaceTestEnv(t *testing.T) *wsTestEnv {
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

	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Workspace{}))

	env := &wsTestEnv{
		service: NewWorkspaceService(
			repository.NewWorkspaceRepository(db),
			repository.NewUserRepository(db),
		),
	}

	for _, u := range []struct {
		username string
		into     **models.User
	}{
		{"creator", &env.creator},
		{"guest", &env.guest},
		{"stranger", &env.stranger},
	} {
		user := &models.User{Username: u.username, Password: "x", Name: u.username}
		require.NoError(t, db.Create(user).Error)
		*u.into = user
	}

	return env
}

// workspace creates one owned by the creator and hands back its id
func (e *wsTestEnv) workspace(t *testing.T, name string) uint {
	t.Helper()

	created, err := e.service.Create(e.creator.ID, &dto.CreateWorkspaceReq{Name: name})
	require.NoError(t, err)

	return created.ID
}

// share grants the guest access to the workspace
func (e *wsTestEnv) share(t *testing.T, workspaceID uint) {
	t.Helper()

	_, err := e.service.Share(workspaceID, e.creator.ID, &dto.ShareWorkspaceReq{
		Username: e.guest.Username,
	})
	require.NoError(t, err)
}

func TestSharingGivesAccessToTheWorkspace(t *testing.T) {
	env := newWorkspaceTestEnv(t)
	id := env.workspace(t, "Kitchen")

	_, err := env.service.Get(id, env.guest.ID)
	require.ErrorIs(t, err, core.ErrNotAllowed, "the guest has no access before it is shared")

	env.share(t, id)

	resp, err := env.service.Get(id, env.guest.ID)
	require.NoError(t, err)
	assert.False(t, resp.IsCreator, "a shared user does not own the workspace")

	owned, err := env.service.Get(id, env.creator.ID)
	require.NoError(t, err)
	assert.True(t, owned.IsCreator)

	// A stranger stays out of it
	_, err = env.service.Get(id, env.stranger.ID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

func TestSharedWorkspacesShowUpInTheGuestListing(t *testing.T) {
	env := newWorkspaceTestEnv(t)
	shared := env.workspace(t, "Kitchen")
	env.workspace(t, "Garage")
	env.share(t, shared)

	guestList, err := env.service.ListForUser(env.guest.ID)
	require.NoError(t, err)
	require.Len(t, guestList.Workspaces, 1, "only the shared one")
	assert.Equal(t, shared, guestList.Workspaces[0].ID)
	assert.False(t, guestList.Workspaces[0].IsCreator)

	creatorList, err := env.service.ListForUser(env.creator.ID)
	require.NoError(t, err)
	assert.Len(t, creatorList.Workspaces, 2, "both of the creator's, listed once each")

	strangerList, err := env.service.ListForUser(env.stranger.ID)
	require.NoError(t, err)
	assert.Empty(t, strangerList.Workspaces)
}

func TestOnlyTheCreatorAdministersTheWorkspace(t *testing.T) {
	env := newWorkspaceTestEnv(t)
	id := env.workspace(t, "Kitchen")
	env.share(t, id)

	_, err := env.service.Update(id, env.guest.ID, &dto.UpdateWorkspaceReq{Name: "Renamed"})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, env.service.Delete(id, env.guest.ID), core.ErrNotAllowed)

	_, err = env.service.Share(id, env.guest.ID, &dto.ShareWorkspaceReq{Username: env.stranger.Username})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, env.service.Revoke(id, env.guest.ID, env.guest.ID), core.ErrNotAllowed)

	// The creator does all of it
	updated, err := env.service.Update(id, env.creator.ID, &dto.UpdateWorkspaceReq{Name: "Renamed"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)
}

func TestSharingRefusesUnknownAndRepeatedUsers(t *testing.T) {
	env := newWorkspaceTestEnv(t)
	id := env.workspace(t, "Kitchen")

	_, err := env.service.Share(id, env.creator.ID, &dto.ShareWorkspaceReq{Username: "nobody"})
	assert.ErrorIs(t, err, core.ErrResourceNotFound)

	_, err = env.service.Share(id, env.creator.ID, &dto.ShareWorkspaceReq{
		Username: env.creator.Username,
	})
	assert.ErrorIs(t, err, core.ErrDuplicatedEntry, "the creator already has access")

	env.share(t, id)

	_, err = env.service.Share(id, env.creator.ID, &dto.ShareWorkspaceReq{
		Username: env.guest.Username,
	})
	assert.ErrorIs(t, err, core.ErrDuplicatedEntry)

	members, err := env.service.Members(id, env.creator.ID)
	require.NoError(t, err)
	require.Len(t, members.Members, 1)
	assert.Equal(t, env.guest.Username, members.Members[0].Username)
}

func TestRevokingAndLeavingTakeTheAccessBack(t *testing.T) {
	env := newWorkspaceTestEnv(t)
	id := env.workspace(t, "Kitchen")
	env.share(t, id)

	require.NoError(t, env.service.Leave(id, env.guest.ID))

	_, err := env.service.Get(id, env.guest.ID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	env.share(t, id)
	require.NoError(t, env.service.Revoke(id, env.creator.ID, env.guest.ID))

	_, err = env.service.Get(id, env.guest.ID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	members, err := env.service.Members(id, env.creator.ID)
	require.NoError(t, err)
	assert.Empty(t, members.Members)
}

func TestTheCreatorCannotLeaveOrBeRevoked(t *testing.T) {
	env := newWorkspaceTestEnv(t)
	id := env.workspace(t, "Kitchen")

	assert.ErrorIs(t, env.service.Leave(id, env.creator.ID), core.ErrNotAllowed)
	assert.ErrorIs(t, env.service.Revoke(id, env.creator.ID, env.creator.ID), core.ErrNotAllowed)

	_, err := env.service.Get(id, env.creator.ID)
	assert.NoError(t, err, "the creator keeps their access")
}

func TestStrangersCannotLeaveOrListMembers(t *testing.T) {
	env := newWorkspaceTestEnv(t)
	id := env.workspace(t, "Kitchen")

	assert.ErrorIs(t, env.service.Leave(id, env.stranger.ID), core.ErrNotAllowed)

	_, err := env.service.Members(id, env.stranger.ID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)
}
