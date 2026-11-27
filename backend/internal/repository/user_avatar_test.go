package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAvatarIndexAndPhotoPrecedence(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)
	applyMigrationsUpTo(t, pool, "9999_sentinel_all_migrations.up.sql")
	repo := New(pool)

	var id string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (name, username, role, status) VALUES ('Peserta', 'avatar-user', 'student', 'active') RETURNING id`,
	).Scan(&id))

	stored := func() (photo *string, idx *int16) {
		require.NoError(t, pool.QueryRow(ctx, `SELECT photo_url, avatar_index FROM users WHERE id = $1`, id).Scan(&photo, &idx))
		return
	}

	require.NoError(t, repo.UpdateUserPhoto(ctx, id, "/files/avatars/a.png"))
	photo, idx := stored()
	require.Equal(t, "/files/avatars/a.png", *photo)
	require.Nil(t, idx)

	seven := 7
	require.NoError(t, repo.UpdateUserAvatarIndex(ctx, id, &seven))
	photo, idx = stored()
	require.NotNil(t, photo, "choosing a sprite keeps the uploaded photo")
	require.Equal(t, int16(7), *idx)

	require.NoError(t, repo.UpdateUserPhoto(ctx, id, "/files/avatars/b.png"))
	photo, idx = stored()
	require.Equal(t, "/files/avatars/b.png", *photo)
	require.Nil(t, idx, "a photo upload clears the sprite choice")

	require.NoError(t, repo.UpdateUserAvatarIndex(ctx, id, &seven))
	require.NoError(t, repo.UpdateUserAvatarIndex(ctx, id, nil))
	photo, idx = stored()
	require.NotNil(t, photo)
	require.Nil(t, idx)
}
