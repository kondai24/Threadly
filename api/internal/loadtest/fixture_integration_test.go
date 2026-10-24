//go:build integration

package loadtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"Threadly/internal/domain/models"
	"Threadly/internal/infra/auth"
	"Threadly/internal/infra/database"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const fixtureTestPassword = "loadtest-fixture-only-password"

// openScratchDBは、TEST_DATABASE_DSNのMySQL serverへ使い捨てschemaを作り、既存migrationを順に適用する。
// 既存test-dbのschemaやデータには触れず、テスト終了時にschemaごと削除する。
func openScratchDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_DSN must be set for integration tests")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	require.NoError(t, err)
	suffix := make([]byte, 6)
	_, err = rand.Read(suffix)
	require.NoError(t, err)
	schema := "threadly_loadtest_fixture_" + hex.EncodeToString(suffix)

	server, err := gorm.Open(mysql.Open(dsn), database.NewGORMConfig())
	require.NoError(t, err)
	require.NoError(t, server.Exec("CREATE DATABASE `"+schema+"`").Error)
	t.Cleanup(func() {
		require.NoError(t, server.Exec("DROP DATABASE `"+schema+"`").Error)
		sqlDB, err := server.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	// 本番のprepareと同じく、既存Atlas migrationをatlas.sumの検証込みで明示適用する。
	atlasURL := fmt.Sprintf("mysql://%s:%s@%s/%s", url.PathEscape(config.User), url.PathEscape(config.Passwd), config.Addr, schema)
	atlas := exec.Command("atlas", "migrate", "apply", "--url", atlasURL, "--dir", "file://../../migrations")
	output, err := atlas.CombinedOutput()
	require.NoError(t, err, "atlas migrate apply failed: %s", strings.ReplaceAll(string(output), config.Passwd, "***"))

	config.DBName = schema
	db, err := gorm.Open(mysql.Open(config.FormatDSN()), database.NewGORMConfig())
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func countRows(t *testing.T, db *gorm.DB) Counts {
	t.Helper()
	var counts [5]int64
	for i, model := range []any{&models.User{}, &models.Post{}, &models.Comment{}, &models.PostLike{}, &models.CommentLike{}} {
		require.NoError(t, db.Model(model).Count(&counts[i]).Error)
	}
	return Counts{Users: int(counts[0]), Posts: int(counts[1]), Comments: int(counts[2]), PostLikes: int(counts[3]), CommentLikes: int(counts[4])}
}

func TestLoadFixture_InsertsSmallFixtureAndRecomputesChecksumFromDB(t *testing.T) {
	db := openScratchDB(t)
	fixture, err := GenerateFixture("small")
	require.NoError(t, err)

	summary, err := LoadFixture(context.Background(), db, fixture, auth.NewArgon2idHasher(), fixtureTestPassword)

	require.NoError(t, err)
	require.Equal(t, FixtureVersion, summary.Version)
	require.Equal(t, "small", summary.Dataset)
	require.Equal(t, fixture.Counts(), summary.Counts)
	require.Equal(t, fixture.Checksum(), summary.Checksum)
	require.Equal(t, fixture.Counts(), countRows(t, db))
	dbChecksum, err := ChecksumFromDB(context.Background(), db)
	require.NoError(t, err)
	require.Equal(t, fixture.Checksum(), dbChecksum)
	var user models.User
	require.NoError(t, db.First(&user, "username = ?", "lt_user_0001").Error)
	require.NoError(t, auth.NewArgon2idHasher().Compare(user.PasswordHash, fixtureTestPassword))
}

func TestLoadFixture_RefusesInitializedDatabaseWithoutChangingIt(t *testing.T) {
	db := openScratchDB(t)
	fixture, err := GenerateFixture("small")
	require.NoError(t, err)
	_, err = LoadFixture(context.Background(), db, fixture, auth.NewArgon2idHasher(), fixtureTestPassword)
	require.NoError(t, err)

	_, err = LoadFixture(context.Background(), db, fixture, auth.NewArgon2idHasher(), fixtureTestPassword)

	require.EqualError(t, err, "already_initialized")
	require.Equal(t, fixture.Counts(), countRows(t, db))
}

func TestLoadFixture_RollsBackEveryRowWhenInsertFailsMidway(t *testing.T) {
	db := openScratchDB(t)
	fixture, err := GenerateFixture("small")
	require.NoError(t, err)
	injected := errors.New("injected comment like failure")
	// 最後に投入するCommentLikeで失敗させ、先に投入したUser〜PostLikeも残らないことを確かめる。
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail-comment-likes", func(tx *gorm.DB) {
		if tx.Statement.Table == "comment_likes" {
			_ = tx.AddError(injected)
		}
	}))

	_, err = LoadFixture(context.Background(), db, fixture, auth.NewArgon2idHasher(), fixtureTestPassword)

	require.Error(t, err)
	require.Equal(t, Counts{}, countRows(t, db))
}

func TestLoadFixture_SchemaRejectsDuplicatesAndBrokenReferences(t *testing.T) {
	db := openScratchDB(t)
	fixture, err := GenerateFixture("small")
	require.NoError(t, err)
	_, err = LoadFixture(context.Background(), db, fixture, auth.NewArgon2idHasher(), fixtureTestPassword)
	require.NoError(t, err)
	missing := models.UUID("ffffffff-ffff-4fff-8fff-ffffffffffff")

	// 投入したfixtureの上でも、既存migrationのunique/FK制約が有効なままであることを確かめる。
	duplicateUser := fixture.Users[0]
	duplicateUser.ID = missing
	duplicateUser.PasswordHash = "hash"
	require.ErrorIs(t, db.Omit(clause.Associations).Create(&duplicateUser).Error, gorm.ErrDuplicatedKey)
	postLike := fixture.PostLikes[0]
	require.ErrorIs(t, db.Omit(clause.Associations).Create(&postLike).Error, gorm.ErrDuplicatedKey)
	commentLike := fixture.CommentLikes[0]
	require.ErrorIs(t, db.Omit(clause.Associations).Create(&commentLike).Error, gorm.ErrDuplicatedKey)
	orphanReply := fixture.Comments[1]
	orphanReply.ID = missing
	missingParent := models.UUID("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	orphanReply.ParentID = &missingParent
	require.ErrorIs(t, db.Omit(clause.Associations).Create(&orphanReply).Error, gorm.ErrForeignKeyViolated)
	require.Equal(t, fixture.Counts(), countRows(t, db))
}
