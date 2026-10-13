package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zafrem/vanish/shared/client"
	"github.com/zafrem/vanish/shared/config"
	"github.com/zafrem/vanish/shared/crypto"
)

// fakeServer serves one encrypted message at /api/messages/m1 and counts GETs.
func fakeServer(t *testing.T, plaintext []byte) (*client.Client, string, *int32) {
	t.Helper()
	enc, err := crypto.EncryptBytes(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/messages/m1" {
			atomic.AddInt32(&hits, 1)
			json.NewEncoder(w).Encode(map[string]string{"ciphertext": enc.Ciphertext, "iv": enc.IV})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	c := client.NewClient(&config.Config{BaseURL: srv.URL, Token: "t"})
	return c, "https://vanish.example/m/m1#" + enc.Key, &hits
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func envelope(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	p, err := crypto.PackFile(name, "application/octet-stream", data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGetFileToExplicitPath(t *testing.T) {
	data := []byte{0, 1, 2, '\n', 0xff}
	c, link, _ := fakeServer(t, envelope(t, "a.bin", data))
	out := filepath.Join(t.TempDir(), "saved.bin")

	saved, err := runGet(c, getOptions{URL: link, Out: out}, &bytes.Buffer{})
	if err != nil || saved != out {
		t.Fatalf("saved=%q err=%v", saved, err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, data) {
		t.Fatalf("content mismatch: %x", got)
	}
	// Windows has no Unix permission bits (files report 0666).
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(out); info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
		}
	}
}

func TestGetTextGoesToStdout(t *testing.T) {
	c, link, _ := fakeServer(t, []byte("my secret"))
	var stdout bytes.Buffer
	saved, err := runGet(c, getOptions{URL: link}, &stdout)
	if err != nil || saved != "" || stdout.String() != "my secret" {
		t.Fatalf("saved=%q out=%q err=%v", saved, stdout.String(), err)
	}
}

func TestGetUsesSanitizedNameInCwd(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	c, link, _ := fakeServer(t, envelope(t, "../../evil.txt", []byte("x")))

	saved, err := runGet(c, getOptions{URL: link}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if saved != "evil.txt" {
		t.Fatalf("saved = %q, want evil.txt", saved)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestGetExplicitPathExistsRefusedBeforeBurn(t *testing.T) {
	out := filepath.Join(t.TempDir(), "taken.bin")
	os.WriteFile(out, []byte("old"), 0o600)
	c, link, hits := fakeServer(t, envelope(t, "a.bin", []byte("new")))

	if _, err := runGet(c, getOptions{URL: link, Out: out}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected refusal to overwrite")
	}
	if *hits != 0 {
		t.Fatal("message must not be fetched (burned) when the destination is unusable")
	}
	if b, _ := os.ReadFile(out); string(b) != "old" {
		t.Fatal("existing file was modified")
	}
}

func TestGetForceOverwrites(t *testing.T) {
	out := filepath.Join(t.TempDir(), "taken.bin")
	os.WriteFile(out, []byte("old"), 0o600)
	c, link, _ := fakeServer(t, envelope(t, "a.bin", []byte("new")))

	if _, err := runGet(c, getOptions{URL: link, Out: out, Force: true}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != "new" {
		t.Fatalf("content = %q", b)
	}
}

func TestGetMissingDirectoryRefusedBeforeBurn(t *testing.T) {
	c, link, hits := fakeServer(t, envelope(t, "a.bin", []byte("x")))
	out := filepath.Join(t.TempDir(), "nope", "a.bin")
	if _, err := runGet(c, getOptions{URL: link, Out: out}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected error")
	}
	if *hits != 0 {
		t.Fatal("message must not be fetched when the directory is missing")
	}
}

func TestGetDefaultNameCollisionPicksFreeName(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	os.WriteFile("report.pdf", []byte("old"), 0o600)
	c, link, _ := fakeServer(t, envelope(t, "report.pdf", []byte("new")))

	saved, err := runGet(c, getOptions{URL: link}, &bytes.Buffer{})
	if err != nil || saved != "report-1.pdf" {
		t.Fatalf("saved=%q err=%v", saved, err)
	}
	if b, _ := os.ReadFile("report.pdf"); string(b) != "old" {
		t.Fatal("existing file was modified")
	}
}

func TestGetBadKeyRefusedBeforeBurn(t *testing.T) {
	c, link, hits := fakeServer(t, []byte("x"))
	truncated := link[:len(link)-6]
	_, err := runGet(c, getOptions{URL: truncated}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("a truncated key must fail")
	}
	if *hits != 0 {
		t.Fatal("message must not be fetched (burned) when the key is invalid")
	}
	if frag := truncated[strings.Index(truncated, "#")+1:]; strings.Contains(err.Error(), frag) {
		t.Fatalf("error leaks the key: %v", err)
	}
}

func TestGetUnpaddedKeyWorks(t *testing.T) {
	c, link, _ := fakeServer(t, []byte("my secret"))
	var stdout bytes.Buffer
	if _, err := runGet(c, getOptions{URL: strings.TrimRight(link, "=")}, &stdout); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "my secret" {
		t.Fatalf("out = %q", stdout.String())
	}
}

func TestGetBadLink(t *testing.T) {
	c, _, hits := fakeServer(t, []byte("x"))
	if _, err := runGet(c, getOptions{URL: "https://vanish.example/m/m1"}, &bytes.Buffer{}); err == nil {
		t.Fatal("link without key must fail")
	}
	if *hits != 0 {
		t.Fatal("must not fetch with an invalid link")
	}
}
