package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zafrem/vanish/shared/config"
	"github.com/zafrem/vanish/shared/crypto"
)

// The decryption key must only appear in the returned link's fragment and
// never in the request sent to the server.
func TestSendMessageDoesNotSendKey(t *testing.T) {
	const key = "SuperSecretKeyMaterial0123456789abcdefABCDE"
	var body string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc123","expires_at":"2030-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := NewClient(&config.Config{BaseURL: srv.URL, Token: "t"})
	url, _, err := c.SendMessage(2, &crypto.EncryptedMessage{Ciphertext: "Y2lwaGVy", IV: "aXY=", Key: key}, 3600)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if strings.Contains(body, key) || strings.Contains(body, "encryption_key") {
		t.Fatalf("request body leaked the key: %s", body)
	}
	if want := srv.URL + "/m/abc123#" + key; url != want {
		t.Fatalf("url = %q, want %q", url, want)
	}
}
