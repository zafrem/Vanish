package client

import "testing"

func TestParseMessageURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantID  string
		wantKey string
		wantErr bool
	}{
		{"basic", "https://vanish.example.com/m/abc123#KEY_-=", "abc123", "KEY_-=", false},
		{"standard base64 key", "http://localhost:3000/m/xyz#a+b/c==", "xyz", "a+b/c==", false},
		{"with path prefix", "https://h.example/app/m/id9#k", "id9", "k", false},
		{"missing fragment", "https://h.example/m/abc", "", "", true},
		{"empty fragment", "https://h.example/m/abc#", "", "", true},
		{"not a message path", "https://h.example/x/abc#k", "", "", true},
		{"garbage", "::::", "", "", true},
		{"empty", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, key, err := ParseMessageURL(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if id != tt.wantID || key != tt.wantKey {
				t.Fatalf("got (%q, %q), want (%q, %q)", id, key, tt.wantID, tt.wantKey)
			}
		})
	}
}
