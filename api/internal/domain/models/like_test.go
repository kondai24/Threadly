package models

import (
	"reflect"
	"testing"
	"time"
)

func TestLikeModelsDoNotHaveSoftDeleteFields(t *testing.T) {
	for _, modelType := range []reflect.Type{reflect.TypeOf(PostLike{}), reflect.TypeOf(CommentLike{})} {
		if _, ok := modelType.FieldByName("DeletedAt"); ok {
			t.Fatalf("%s has DeletedAt, want physical deletion", modelType.Name())
		}
		if _, ok := modelType.FieldByName("UpdatedAt"); ok {
			t.Fatalf("%s has UpdatedAt, want only created_at", modelType.Name())
		}
		if _, ok := modelType.FieldByName("CreatedAt"); !ok {
			t.Fatalf("%s has no CreatedAt", modelType.Name())
		}
	}
}

func TestLikeModelsUseUUIDIDs(t *testing.T) {
	postLike := PostLike{UserID: "11111111-1111-4111-8111-111111111111", PostID: "22222222-2222-4222-8222-222222222222", CreatedAt: time.Now()}
	commentLike := CommentLike{UserID: "11111111-1111-4111-8111-111111111111", CommentID: "33333333-3333-4333-8333-333333333333", CreatedAt: time.Now()}

	if postLike.UserID == "" || postLike.PostID == "" || commentLike.UserID == "" || commentLike.CommentID == "" {
		t.Fatal("like target IDs must be UUID values")
	}
}
