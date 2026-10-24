package loadtest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateFixture_SmallFollowsFixedCountsAndRelations(t *testing.T) {
	fixture, err := GenerateFixture("small")
	require.NoError(t, err)

	require.Equal(t, Counts{Users: 32, Posts: 100, Comments: 1000, PostLikes: 200, CommentLikes: 2000}, fixture.Counts())
	require.Equal(t, "lt_user_0001", fixture.Users[0].Username)
	require.Equal(t, "lt_user_0032", fixture.Users[31].Username)
	// 32人に対し33件目のPostは作者indexが剰余で先頭へ戻る。
	require.Equal(t, fixture.Users[0].ID, fixture.Posts[32].AuthorID)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), fixture.Users[0].CreatedAt)
	firstPost := fixture.Posts[0]
	require.Len(t, firstPost.Title, 32)
	require.Len(t, firstPost.Content, 256)
	parents, replies := 0, 0
	for _, comment := range fixture.Comments[:10] {
		require.Equal(t, firstPost.ID, comment.PostID)
		require.Len(t, comment.Content, 64)
		if comment.ParentID == nil {
			parents++
		} else {
			replies++
		}
	}
	require.Equal(t, 5, parents)
	require.Equal(t, 5, replies)
	require.Equal(t, fixture.Comments[0].ID, *fixture.Comments[1].ParentID)
	// Like Userは作者index+1、+2で、作者自身や同一Userの重複を作らない。
	require.Equal(t, fixture.Users[1].ID, fixture.PostLikes[0].UserID)
	require.Equal(t, fixture.Users[2].ID, fixture.PostLikes[1].UserID)
	require.Equal(t, firstPost.ID, fixture.PostLikes[1].PostID)
}

func TestGenerateFixture_KeepsIDsAndLikePairsUniqueForEveryDataset(t *testing.T) {
	cases := []struct {
		name    string
		dataset string
		want    Counts
	}{
		{name: "mediumは仕様の件数", dataset: "medium", want: Counts{Users: 64, Posts: 1000, Comments: 10000, PostLikes: 2000, CommentLikes: 20000}},
		{name: "largeは仕様の件数", dataset: "large", want: Counts{Users: 128, Posts: 10000, Comments: 100000, PostLikes: 20000, CommentLikes: 200000}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fixture, err := GenerateFixture(tt.dataset)
			require.NoError(t, err)
			require.Equal(t, tt.want, fixture.Counts())
			ids := map[string]bool{}
			for _, user := range fixture.Users {
				ids[string(user.ID)] = true
			}
			for _, post := range fixture.Posts {
				ids[string(post.ID)] = true
			}
			for _, comment := range fixture.Comments {
				ids[string(comment.ID)] = true
			}
			require.Len(t, ids, tt.want.Users+tt.want.Posts+tt.want.Comments)
			postLikes := map[string]bool{}
			for _, like := range fixture.PostLikes {
				postLikes[string(like.UserID)+string(like.PostID)] = true
			}
			require.Len(t, postLikes, tt.want.PostLikes)
			commentLikes := map[string]bool{}
			for _, like := range fixture.CommentLikes {
				commentLikes[string(like.UserID)+string(like.CommentID)] = true
			}
			require.Len(t, commentLikes, tt.want.CommentLikes)
		})
	}
}

func TestGenerateFixture_RejectsUnknownDataset(t *testing.T) {
	_, err := GenerateFixture("huge")
	require.EqualError(t, err, "invalid_input")
}

func TestFixtureChecksum_IsDeterministicAndCoversLogicalData(t *testing.T) {
	first, err := GenerateFixture("small")
	require.NoError(t, err)
	second, err := GenerateFixture("small")
	require.NoError(t, err)
	require.Equal(t, first.Checksum(), second.Checksum())
	require.Regexp(t, "^sha256:[0-9a-f]{64}$", first.Checksum())

	// password hashはランダムsaltのため対象外とし、本文・関連・時刻の変更は検出する。
	second.Users[0].PasswordHash = "random-salted-hash"
	require.Equal(t, first.Checksum(), second.Checksum())
	second.Comments[3].Content = "changed"
	require.NotEqual(t, first.Checksum(), second.Checksum())
}

func TestFixtureChecksum_MatchesRecordedSmallFixtureV1(t *testing.T) {
	// fixture v1の生成規則を変えた場合は版を上げ、過去の結果と比較しないため固定値で検出する。
	fixture, err := GenerateFixture("small")
	require.NoError(t, err)
	require.Equal(t, "sha256:052c0dd27e04cfbafa26f75a49cf3e9a5574c2cac98ba18d81746423e3e18eb7", fixture.Checksum())
}
