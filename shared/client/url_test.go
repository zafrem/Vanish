package client

import (
	"strings"
	"testing"
)

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
		{"real server id with padding", "https://h.example/m/q83v-Ab_Zx0123456789AB==#k", "q83v-Ab_Zx0123456789AB==", "k", false},
		{"missing fragment", "https://h.example/m/abc", "", "", true},
		{"empty fragment", "https://h.example/m/abc#", "", "", true},
		{"not a message path", "https://h.example/x/abc#k", "", "", true},
		{"garbage", "::::", "", "", true},
		{"empty", "", "", "", true},
		{"escaped query in id", "https://h.example/m/abc%3Fx=1#k", "", "", true},
		{"dot-dot id", "https://h.example/m/..#k", "", "", true},
		{"escaped slash in id", "https://h.example/m/a%2Fb#k", "", "", true},
		{"padding in the middle of id", "https://h.example/m/a=b#k", "", "", true},
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

// Errors are printed to stderr, so they must never echo the key.
func TestParseMessageURLErrorsDoNotLeakKey(t *testing.T) {
	const key = "SuperSecretKeyMaterial0123456789abcdefABCDE"
	for _, raw := range []string{
		"https://h.example/m/a b\x7f#" + key,     // url.Parse fails on the control character
		"https://h.example/x/abc#" + key,         // not a message path
		"https://h.example/m/..#" + key,          // invalid id
		"%zz/m/abc#" + key,                       // bad escape
		"https://h.example/m/abc#" + key + "%zz", // bad escape in the fragment: url.Parse echoes the whole link
	} {
		_, _, err := ParseMessageURL(raw)
		if err == nil {
			t.Errorf("%q: expected an error", raw)
			continue
		}
		if strings.Contains(err.Error(), key) {
			t.Errorf("error leaks the key: %v", err)
		}
	}
}
