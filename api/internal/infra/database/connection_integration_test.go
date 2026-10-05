//go:build integration

package database

import (
	"os"
	"testing"

	"Threadly/internal/domain/models"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMigration_PreservesLookupIndexesAndForeignKeys(t *testing.T) {
	db := openIntegrationDB(t)
	migrator := db.Migrator()

	require.True(t, migrator.HasIndex(&models.Comment{}, "idx_comments_post_id"))
	require.True(t, migrator.HasIndex(&models.Comment{}, "idx_comments_author_id"))
	require.True(t, migrator.HasIndex(&models.Comment{}, "idx_comments_parent_id"))
	require.True(t, migrator.HasIndex(&models.PostLike{}, "idx_post_likes_post_id"))
	require.True(t, migrator.HasIndex(&models.CommentLike{}, "idx_comment_likes_comment_id"))
	require.True(t, migrator.HasConstraint(&models.Comment{}, "Post"))
	require.True(t, migrator.HasConstraint(&models.Comment{}, "Author"))
	require.True(t, migrator.HasConstraint(&models.PostLike{}, "User"))
	require.True(t, migrator.HasConstraint(&models.PostLike{}, "Post"))
	require.True(t, migrator.HasConstraint(&models.CommentLike{}, "User"))
	require.True(t, migrator.HasConstraint(&models.CommentLike{}, "Comment"))
}

func TestMigration_RejectsPostWithMissingAuthor(t *testing.T) {
	tx := newIntegrationTransaction(t)
	post := &models.Post{
		AuthorID: "ffffffff-ffff-4fff-8fff-ffffffffffff",
		Title:    "orphan post", Content: "must be rejected",
	}
	require.ErrorIs(t, tx.Create(post).Error, gorm.ErrForeignKeyViolated)
}

func TestMigration_RejectsCommentWithMissingParent(t *testing.T) {
	tx := newIntegrationTransaction(t)
	user := &models.User{Username: "orphan-comment-user", PasswordHash: "hash"}
	require.NoError(t, tx.Create(user).Error)
	post := &models.Post{AuthorID: user.ID, Title: "post", Content: "content"}
	require.NoError(t, tx.Create(post).Error)
	missingParentID := models.UUID("ffffffff-ffff-4fff-8fff-ffffffffffff")
	comment := &models.Comment{
		PostID: post.ID, AuthorID: user.ID, ParentID: &missingParentID, Content: "orphan reply",
	}
	require.ErrorIs(t, tx.Create(comment).Error, gorm.ErrForeignKeyViolated)
}

func openIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_DSN must be set for integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), NewGORMConfig())
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func newIntegrationTransaction(t *testing.T) *gorm.DB {
	t.Helper()

	db := openIntegrationDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() {
		require.NoError(t, tx.Rollback().Error)
	})
	return tx
}
