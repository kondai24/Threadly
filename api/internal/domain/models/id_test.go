package models

import "testing"

func TestParseUUIDNormalizesUppercase(t *testing.T) {
	got, err := ParseUUID("AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA")
	if err != nil {
		t.Fatalf("ParseUUID() error = %v", err)
	}
	if got != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("ParseUUID() = %s, want normalized UUID", got)
	}
}
