package main

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zafrem/vanish/shared/crypto"
)

func TestLoadFileEnvelope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("hello\x00world"), 0o600); err != nil {
		t.Fatal(err)
	}

	packed, err := loadFileEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := crypto.Unpack(packed)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != crypto.KindFile || p.Name != "notes.txt" || string(p.Data) != "hello\x00world" {
		t.Fatalf("unexpected payload %+v", p)
	}
	if p.Type == "" {
		t.Fatal("expected a MIME type")
	}
}

func TestLoadFileEnvelopeRejects(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty")
	os.WriteFile(empty, nil, 0o600)
	if _, err := loadFileEnvelope(empty); err == nil {
		t.Fatal("empty file should be rejected")
	}

	big := filepath.Join(dir, "big")
	os.WriteFile(big, make([]byte, crypto.MaxFileBytes+1), 0o600)
	if _, err := loadFileEnvelope(big); err == nil {
		t.Fatal("oversized file should be rejected")
	}

	exact := filepath.Join(dir, "exact")
	os.WriteFile(exact, make([]byte, crypto.MaxFileBytes), 0o600)
	if _, err := loadFileEnvelope(exact); err != nil {
		t.Fatalf("exactly max should be accepted: %v", err)
	}

	if _, err := loadFileEnvelope(dir); err == nil {
		t.Fatal("directory should be rejected")
	}
	if _, err := loadFileEnvelope(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing file should be rejected")
	}
}

func TestParseInterspersed(t *testing.T) {
	newFS := func() (*flag.FlagSet, *string, *int64) {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		file := fs.String("file", "", "")
		ttl := fs.Int64("ttl", 0, "")
		return fs, file, ttl
	}

	fs, file, ttl := newFS()
	got, err := parseInterspersed(fs, []string{"user@x.com", "-file", "a.bin", "-ttl", "60"}, true)
	if err != nil || *file != "a.bin" || *ttl != 60 || !reflect.DeepEqual(got, []string{"user@x.com"}) {
		t.Fatalf("flags after email: got %v %q %d %v", got, *file, *ttl, err)
	}

	fs, file, _ = newFS()
	got, _ = parseInterspersed(fs, []string{"-file", "a.bin", "user@x.com", "hello", "world"}, true)
	if *file != "a.bin" || !reflect.DeepEqual(got, []string{"user@x.com", "hello", "world"}) {
		t.Fatalf("flags before email + message: got %v %q", got, *file)
	}

	fs, _, _ = newFS()
	got, _ = parseInterspersed(fs, []string{"user@x.com", "my", "secret"}, true)
	if !reflect.DeepEqual(got, []string{"user@x.com", "my", "secret"}) {
		t.Fatalf("plain message: got %v", got)
	}
}
