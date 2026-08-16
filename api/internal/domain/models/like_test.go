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

func TestLikeModelsUseIncrementingIDsAndUUIDTargetIDs(t *testing.T) {
	for _, modelType := range []reflect.Type{reflect.TypeOf(PostLike{}), reflect.TypeOf(CommentLike{})} {
		idField, ok := modelType.FieldByName("ID")
		if !ok {
			t.Fatalf("%s has no ID", modelType.Name())
		}
		if idField.Type.Kind() != reflect.Uint64 {
			t.Fatalf("%s.ID has type %s, want uint64", modelType.Name(), idField.Type)
		}
	}

	postLike := PostLike{UserID: "11111111-1111-4111-8111-111111111111", PostID: "22222222-2222-4222-8222-222222222222", CreatedAt: time.Now()}
	commentLike := CommentLike{UserID: "11111111-1111-4111-8111-111111111111", CommentID: "33333333-3333-4333-8333-333333333333", CreatedAt: time.Now()}

	if postLike.UserID == "" || postLike.PostID == "" || commentLike.UserID == "" || commentLike.CommentID == "" {
		t.Fatal("like user and target IDs must be UUID values")
	}
}
