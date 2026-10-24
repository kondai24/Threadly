package loadtest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"Threadly/internal/domain/models"
)

const (
	// FixtureVersionは、生成規則を変えた場合に比較対象を分けるための版である。
	FixtureVersion = "v1"

	commentsPerPost   = 10
	parentsPerPost    = 5
	postTitleLength   = 32
	postContentLength = 256
	commentLength     = 64
)

// fixtureEpochは、全fixtureの作成・更新日時の基準である。連番秒を足して決定的にする。
var fixtureEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Fixtureは、専用DBへ投入する固定データを既存Modelで表す。password hashは投入時に付与する。
type Fixture struct {
	Dataset      string
	Users        []models.User
	Posts        []models.Post
	Comments     []models.Comment
	PostLikes    []models.PostLike
	CommentLikes []models.CommentLike
}

// GenerateFixtureは、datasetごとの件数と関連を、種別と連番だけから決定的に生成する。
func GenerateFixture(dataset string) (Fixture, error) {
	plan, err := NewPlan("browse", dataset)
	if err != nil {
		return Fixture{}, errors.New("invalid_input")
	}
	counts := plan.FixtureCounts
	fixture := Fixture{Dataset: dataset}
	for i := 0; i < counts.Users; i++ {
		fixture.Users = append(fixture.Users, models.User{
			UUIDBaseModel: baseModel(fixtureID(1, i), i),
			Username:      fmt.Sprintf("lt_user_%04d", i+1),
		})
	}
	userAt := func(index int) models.UUID { return fixture.Users[index%counts.Users].ID }
	for p := 0; p < counts.Posts; p++ {
		author := p % counts.Users
		post := models.Post{
			UUIDBaseModel: baseModel(fixtureID(2, p), p),
			AuthorID:      userAt(author),
			Title:         fixedText(fmt.Sprintf("lt post %06d ", p+1), postTitleLength),
			Content:       fixedText(fmt.Sprintf("lt post content %06d ", p+1), postContentLength),
		}
		fixture.Posts = append(fixture.Posts, post)
		fixture.PostLikes = append(fixture.PostLikes, likesFor(author, counts.Users, func(user int) models.PostLike {
			return models.PostLike{UserID: userAt(user), PostID: post.ID, CreatedAt: post.CreatedAt}
		})...)
		// 各Postへ親Comment5件と、それぞれへの返信1件を親→返信の順に並べる。
		for k := 0; k < parentsPerPost; k++ {
			parentSeq := p*commentsPerPost + 2*k
			parent := newComment(parentSeq, post.ID, userAt(parentSeq), nil)
			reply := newComment(parentSeq+1, post.ID, userAt(parentSeq+1), &parent.ID)
			for offset, comment := range []models.Comment{parent, reply} {
				seq := parentSeq + offset
				fixture.Comments = append(fixture.Comments, comment)
				fixture.CommentLikes = append(fixture.CommentLikes, likesFor(seq%counts.Users, counts.Users, func(user int) models.CommentLike {
					return models.CommentLike{UserID: userAt(user), CommentID: comment.ID, CreatedAt: comment.CreatedAt}
				})...)
			}
		}
	}
	return fixture, nil
}

// Countsは、生成済みfixtureの件数を返す。
func (f Fixture) Counts() Counts {
	return Counts{
		Users: len(f.Users), Posts: len(f.Posts), Comments: len(f.Comments),
		PostLikes: len(f.PostLikes), CommentLikes: len(f.CommentLikes),
	}
}

func newComment(seq int, postID, authorID models.UUID, parentID *models.UUID) models.Comment {
	return models.Comment{
		UUIDBaseModel: baseModel(fixtureID(3, seq), seq),
		PostID:        postID,
		AuthorID:      authorID,
		ParentID:      parentID,
		Content:       fixedText(fmt.Sprintf("lt comment %07d ", seq+1), commentLength),
	}
}

// likesForは、作者index+1と+2のUserでLikeを作る。User数は3以上なので作者と重複しない。
func likesFor[T any](author, users int, like func(user int) T) []T {
	return []T{like((author + 1) % users), like((author + 2) % users)}
}

func baseModel(id models.UUID, seq int) models.UUIDBaseModel {
	at := fixtureEpoch.Add(time.Duration(seq) * time.Second)
	return models.UUIDBaseModel{ID: id, CreatedAt: at, UpdatedAt: at}
}

// fixtureIDは、種別を先頭、連番を末尾に置いたUUID形式の固定IDを作る。
func fixtureID(kind, index int) models.UUID {
	return models.UUID(fmt.Sprintf("%08x-0000-4000-8000-%012x", kind, index))
}

func fixedText(prefix string, length int) string {
	return prefix + strings.Repeat("x", length-len(prefix))
}

// Checksumは、hashを除く論理データ（ID・関連・本文・時刻）から順序に依存しない値を作る。
// 生成直後と投入後のDB読み戻しの両方で同じ規則を使い、再現性を比較する。
func (f Fixture) Checksum() string {
	return checksumLines(f.logicalLines())
}

func (f Fixture) logicalLines() []string {
	lines := make([]string, 0, len(f.Users)+len(f.Posts)+len(f.Comments)+len(f.PostLikes)+len(f.CommentLikes))
	for _, user := range f.Users {
		lines = append(lines, userLine(user))
	}
	for _, post := range f.Posts {
		lines = append(lines, postLine(post))
	}
	for _, comment := range f.Comments {
		lines = append(lines, commentLine(comment))
	}
	for _, like := range f.PostLikes {
		lines = append(lines, postLikeLine(like))
	}
	for _, like := range f.CommentLikes {
		lines = append(lines, commentLikeLine(like))
	}
	return lines
}

func userLine(user models.User) string {
	return strings.Join([]string{"user", string(user.ID), user.Username, timeText(user.CreatedAt), timeText(user.UpdatedAt)}, "|")
}

func postLine(post models.Post) string {
	return strings.Join([]string{"post", string(post.ID), string(post.AuthorID), post.Title, post.Content, timeText(post.CreatedAt), timeText(post.UpdatedAt)}, "|")
}

func commentLine(comment models.Comment) string {
	parent := ""
	if comment.ParentID != nil {
		parent = string(*comment.ParentID)
	}
	return strings.Join([]string{"comment", string(comment.ID), string(comment.PostID), string(comment.AuthorID), parent, comment.Content, timeText(comment.CreatedAt), timeText(comment.UpdatedAt)}, "|")
}

func postLikeLine(like models.PostLike) string {
	return strings.Join([]string{"post_like", string(like.UserID), string(like.PostID), timeText(like.CreatedAt)}, "|")
}

func commentLikeLine(like models.CommentLike) string {
	return strings.Join([]string{"comment_like", string(like.UserID), string(like.CommentID), timeText(like.CreatedAt)}, "|")
}

func timeText(at time.Time) string { return at.UTC().Format(time.RFC3339Nano) }

func checksumLines(lines []string) string {
	sorted := append([]string(nil), lines...)
	sort.Strings(sorted)
	hash := sha256.New()
	for _, line := range sorted {
		hash.Write([]byte(line))
		hash.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
