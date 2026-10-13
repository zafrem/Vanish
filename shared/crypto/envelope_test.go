package crypto

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestPackUnpackRoundTrip(t *testing.T) {
	data := []byte{0x00, '\n', 0xff, 0x00, 'a', '\n'}
	packed, err := PackFile("비밀 문서.bin", "application/octet-stream", data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(packed, []byte(EnvelopeMagic)) {
		t.Fatal("missing magic prefix")
	}
	p, err := Unpack(packed)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != KindFile || p.Name != "비밀 문서.bin" || p.Type != "application/octet-stream" || !bytes.Equal(p.Data, data) {
		t.Fatalf("unexpected payload: %+v", p)
	}
}

func TestPackFileLimits(t *testing.T) {
	if _, err := PackFile("a", "text/plain", nil); !errors.Is(err, ErrFileEmpty) {
		t.Fatalf("empty: want ErrFileEmpty, got %v", err)
	}
	if _, err := PackFile("a", "text/plain", make([]byte, MaxFileBytes)); err != nil {
		t.Fatalf("exactly max should be accepted: %v", err)
	}
	if _, err := PackFile("a", "text/plain", make([]byte, MaxFileBytes+1)); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("max+1: want ErrFileTooLarge, got %v", err)
	}
}

func TestUnpackPlainTextIsText(t *testing.T) {
	for _, s := range []string{"hello", "", "こんにちは 🔒", "multi\nline"} {
		p, err := Unpack([]byte(s))
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if p.Kind != KindText || p.Text != s {
			t.Fatalf("%q: got %+v", s, p)
		}
	}
}

func TestUnpackMagicWithoutValidHeaderIsText(t *testing.T) {
	cases := map[string]string{
		"magic only":        EnvelopeMagic,
		"no newline":        EnvelopeMagic + `{"name":"a","type":"b","size":1}`,
		"not json":          EnvelopeMagic + "not json\nrest",
		"empty object":      EnvelopeMagic + "{}\nx",
		"missing size":      EnvelopeMagic + `{"name":"a","type":"b"}` + "\nx",
		"negative size":     EnvelopeMagic + `{"name":"a","type":"b","size":-1}` + "\nx",
		"header too long":   EnvelopeMagic + strings.Repeat("x", 5000) + "\nrest",
		"name not a string": EnvelopeMagic + `{"name":1,"type":"b","size":1}` + "\nx",
	}
	for name, in := range cases {
		p, err := Unpack([]byte(in))
		if err != nil {
			t.Fatalf("%s: unexpected error %v", name, err)
		}
		if p.Kind != KindText || p.Text != in {
			t.Fatalf("%s: want text %q, got %+v", name, in, p)
		}
	}
}

func TestUnpackSizeMismatchIsError(t *testing.T) {
	in := EnvelopeMagic + `{"name":"a","type":"b","size":5}` + "\nabc"
	if _, err := Unpack([]byte(in)); !errors.Is(err, ErrSizeMismatch) {
		t.Fatalf("want ErrSizeMismatch, got %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{"../../etc/passwd", "passwd"},
		{`C:\Users\bob\secret.txt`, "secret.txt"},
		{`a/b\c.txt`, "c.txt"},
		{`na:me*?.txt`, "name.txt"},
		{"  ..hidden.txt.. ", "hidden.txt"},
		{"line\nbreak\t.txt", "linebreak.txt"},
		{"", "vanish-file"},
		{"...", "vanish-file"},
		{"dir/", "vanish-file"},
		{strings.Repeat("a", 300), strings.Repeat("a", 255)},
		{strings.Repeat("가", 100), strings.Repeat("가", 85)},
	}
	for _, c := range cases {
		if got := SanitizeFilename(c.in); got != c.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
