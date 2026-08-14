package models

import "testing"

func TestNewUUID(t *testing.T) {
	id := NewUUID()
	parsed, err := ParseUUID(string(id))
	if err != nil {
		t.Fatalf("ParseUUID() error = %v", err)
	}
	if parsed != id {
		t.Fatalf("parsed UUID = %s, want %s", parsed, id)
	}
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    UUID
		wantErr bool
	}{
		{
			name:  "正規形式のUUIDを受け入れる",
			input: "11111111-1111-4111-8111-111111111111",
			want:  "11111111-1111-4111-8111-111111111111",
		},
		{
			name:  "大文字UUIDを小文字へ正規化する",
			input: "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA",
			want:  "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		},
		{
			name:    "UUIDではない値を拒否する",
			input:   "42",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUUID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ParseUUID() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUUID() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ParseUUID() = %s, want %s", got, tt.want)
			}
		})
	}
}
