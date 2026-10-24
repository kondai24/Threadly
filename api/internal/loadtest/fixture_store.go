package loadtest

import (
	"context"
	"errors"
	"fmt"

	"Threadly/internal/domain/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const fixtureBatchSize = 1000

// ErrAlreadyInitializedは、投入先DBに既存行があり初期化済みと判断したことを表す。
var ErrAlreadyInitialized = errors.New("already_initialized")

// ErrFixtureMismatchは、投入後にDBから読み戻した論理データが生成値と一致しないことを表す。
var ErrFixtureMismatch = errors.New("fixture_mismatch")

// FixtureSummaryは、投入済みfixtureを比較するためのhashを含まないmetadataである。
type FixtureSummary struct {
	Version  string `json:"version"`
	Dataset  string `json:"dataset"`
	Counts   Counts `json:"counts"`
	Checksum string `json:"checksum"`
}

// passwordHasherは、通常の認証と同じArgon2id設定でfixture Userのhashを作る。
type passwordHasher interface {
	Hash(password string) (string, error)
}

// LoadFixtureは、空の専用DBへfixtureを1つのtransactionで投入する。
// 投入後にDBから読み戻したchecksumが生成値と一致した場合だけcommitし、不完全な状態を準備完了にしない。
func LoadFixture(ctx context.Context, db *gorm.DB, fixture Fixture, hasher passwordHasher, password string) (FixtureSummary, error) {
	users := append([]models.User(nil), fixture.Users...)
	for i := range users {
		hash, err := hasher.Hash(password)
		if err != nil {
			return FixtureSummary{}, errors.New("hash_failed")
		}
		users[i].PasswordHash = hash
	}
	expected := fixture.Checksum()
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		counts, err := countFixtureRows(tx)
		if err != nil {
			return err
		}
		if counts != (Counts{}) {
			return ErrAlreadyInitialized
		}
		// 関連の自動保存を止め、生成済みの外部キーだけを投入する。
		tx = tx.Omit(clause.Associations)
		for _, rows := range []any{users, fixture.Posts, fixture.Comments, fixture.PostLikes, fixture.CommentLikes} {
			if err := tx.CreateInBatches(rows, fixtureBatchSize).Error; err != nil {
				return fmt.Errorf("insert fixture: %w", err)
			}
		}
		actual, err := ChecksumFromDB(ctx, tx)
		if err != nil {
			return err
		}
		if actual != expected {
			return ErrFixtureMismatch
		}
		return nil
	})
	if err != nil {
		return FixtureSummary{}, err
	}
	return FixtureSummary{Version: FixtureVersion, Dataset: fixture.Dataset, Counts: fixture.Counts(), Checksum: expected}, nil
}

// ChecksumFromDBは、DBに保存された論理データからFixture.Checksumと同じ規則で値を作る。
func ChecksumFromDB(ctx context.Context, db *gorm.DB) (string, error) {
	db = db.WithContext(ctx)
	var (
		users        []models.User
		posts        []models.Post
		comments     []models.Comment
		postLikes    []models.PostLike
		commentLikes []models.CommentLike
	)
	for _, rows := range []any{&users, &posts, &comments, &postLikes, &commentLikes} {
		if err := db.Unscoped().Find(rows).Error; err != nil {
			return "", fmt.Errorf("read fixture: %w", err)
		}
	}
	stored := Fixture{Users: users, Posts: posts, Comments: comments, PostLikes: postLikes, CommentLikes: commentLikes}
	return stored.Checksum(), nil
}

func countFixtureRows(db *gorm.DB) (Counts, error) {
	var counts [5]int64
	for i, model := range []any{&models.User{}, &models.Post{}, &models.Comment{}, &models.PostLike{}, &models.CommentLike{}} {
		if err := db.Unscoped().Model(model).Count(&counts[i]).Error; err != nil {
			return Counts{}, fmt.Errorf("count fixture: %w", err)
		}
	}
	return Counts{Users: int(counts[0]), Posts: int(counts[1]), Comments: int(counts[2]), PostLikes: int(counts[3]), CommentLikes: int(counts[4])}, nil
}
