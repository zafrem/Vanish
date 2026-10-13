package crypto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

const vectorPath = "testdata/vector.json"

type sanitizeCase struct {
	In   string `json:"in"`
	Want string `json:"want"`
}

// unpackCase pins how Unpack (Go) and unpackEnvelope (JS) classify raw
// plaintext. Kind is "text", "file" or "error"; the file fields are only
// meaningful for "file".
type unpackCase struct {
	Name     string `json:"name"`
	InputB64 string `json:"input_b64"`
	Kind     string `json:"kind"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	DataB64  string `json:"data_b64"`
}

type vectorFile struct {
	KeyURL         string         `json:"key_url"`
	KeyStd         string         `json:"key_std"`
	IV             string         `json:"iv"`
	FileName       string         `json:"file_name"`
	FileType       string         `json:"file_type"`
	FileDataB64    string         `json:"file_data_b64"`
	FileCiphertext string         `json:"file_ciphertext"`
	TextPlain      string         `json:"text_plain"`
	TextCiphertext string         `json:"text_ciphertext"`
	Sanitize       []sanitizeCase `json:"sanitize"`
	Unpack         []unpackCase   `json:"unpack"`
}

// headerOfLen returns a valid header (name "nnn...", type "t", size 1) that is
// exactly n bytes long.
func headerOfLen(n int) string {
	const empty = `{"name":"","type":"t","size":1}`
	return `{"name":"` + strings.Repeat("n", n-len(empty)) + `","type":"t","size":1}`
}

func unpackCases() []unpackCase {
	const m = EnvelopeMagic
	const bs = "\x5c" // backslash, for JSON escapes inside raw strings
	b := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	text := func(label, in string) unpackCase {
		return unpackCase{Name: label, InputB64: b(in), Kind: "text"}
	}
	file := func(label, in, name, mime, data string) unpackCase {
		return unpackCase{Name: label, InputB64: b(in), Kind: "file", FileName: name, FileType: mime, DataB64: b(data)}
	}
	h4096, h4097 := headerOfLen(4096), headerOfLen(4097)

	return []unpackCase{
		text("plain text", "hello"),
		text("plain text starting with BOM", "\xef\xbb\xbfhello"),
		text("magic only", m),
		text("no newline after header", m+`{"name":"a","type":"t","size":1}`),
		text("not json", m+"not json\nrest"),
		text("empty object", m+"{}\nx"),
		text("missing size", m+`{"name":"a","type":"t"}`+"\nx"),
		text("missing name", m+`{"type":"t","size":1}`+"\nx"),
		text("name not a string", m+`{"name":1,"type":"t","size":1}`+"\nx"),
		text("name null", m+`{"name":null,"type":"t","size":1}`+"\nx"),
		text("type not a string", m+`{"name":"a","type":["t"],"size":1}`+"\nx"),
		text("size as string", m+`{"name":"a","type":"t","size":"1"}`+"\nx"),
		text("size null", m+`{"name":"a","type":"t","size":null}`+"\nx"),
		text("size true", m+`{"name":"a","type":"t","size":true}`+"\nx"),
		text("size -1", m+`{"name":"a","type":"t","size":-1}`+"\nx"),
		text("size 1.5", m+`{"name":"a","type":"t","size":1.5}`+"\nx"),
		text("size 5242881", m+`{"name":"a","type":"t","size":5242881}`+"\nx"),
		text("size 1e300", m+`{"name":"a","type":"t","size":1e300}`+"\nx"),
		text("size 20 digits", m+`{"name":"a","type":"t","size":12345678901234567890}`+"\nx"),
		text("size 1e400 overflows", m+`{"name":"a","type":"t","size":1e400}`+"\nx"),
		file("size 1.0 is integral", m+`{"name":"a","type":"t","size":1.0}`+"\nx", "a", "t", "x"),
		file("size 1e0 is integral", m+`{"name":"a","type":"t","size":1e0}`+"\nx", "a", "t", "x"),
		file("size 0.1e1 is integral", m+`{"name":"a","type":"t","size":0.1e1}`+"\nx", "a", "t", "x"),
		file("size 0 with no data", m+`{"name":"a","type":"t","size":0}`+"\n", "a", "t", ""),
		file("size -0 counts as 0", m+`{"name":"a","type":"t","size":-0}`+"\n", "a", "t", ""),
		text("uppercase keys", m+`{"NAME":"a","Type":"t","SIZE":1}`+"\nx"),
		file("case-variant duplicate key is ignored", m+`{"name":"a","type":"t","size":1,"NAME":"evil.exe"}`+"\nx", "a", "t", "x"),
		file("exact duplicate key: last wins", m+`{"name":"a","type":"t","size":1,"name":"evil.exe"}`+"\nx", "evil.exe", "t", "x"),
		file("duplicate size: last wins", m+`{"name":"a","type":"t","size":5,"size":1}`+"\nx", "a", "t", "x"),
		file("escaped key", m+`{"n`+bs+`u0061me":"a","type":"t","size":1}`+"\nx", "a", "t", "x"),
		file("escaped slash in name", m+`{"name":"a`+bs+`u002fb","type":"t","size":1}`+"\nx", "a/b", "t", "x"),
		file("lone surrogate escape becomes U+FFFD", m+`{"name":"a`+bs+`ud800b","type":"t","size":1}`+"\nx", "a\xef\xbf\xbdb", "t", "x"),
		text("BOM before header", m+"\xef\xbb\xbf"+`{"name":"a","type":"t","size":1}`+"\nx"),
		text("invalid UTF-8 in header", m+`{"name":"a`+"\xff"+`","type":"t","size":1}`+"\nx"),
		text("header is an array", m+"[]\nx"),
		text("header is null", m+"null\nx"),
		text("header is a number", m+"1\nx"),
		text("header is a string", m+`"s"`+"\nx"),
		text("trailing garbage after header object", m+`{"name":"a","type":"t","size":1} x`+"\nx"),
		file("whitespace around header", m+` {"name":"a","type":"t","size":1}`+"\r\nx", "a", "t", "x"),
		file("unknown keys are ignored", m+`{"name":"a","type":"t","size":1,"extra":{"k":[1]}}`+"\nx", "a", "t", "x"),
		file("header exactly 4096 bytes", m+h4096+"\nx", strings.Repeat("n", 4096-len(`{"name":"","type":"t","size":1}`)), "t", "x"),
		text("header 4097 bytes", m+h4097+"\nx"),
		file("binary data with NUL and newlines", m+`{"name":"x.bin","type":"application/octet-stream","size":4}`+"\n\x00\n\xff\n", "x.bin", "application/octet-stream", "\x00\n\xff\n"),
		{Name: "size mismatch", InputB64: b(m + `{"name":"a","type":"t","size":5}` + "\nabc"), Kind: "error"},
	}
}

func vectorInputs() (key, nonce, data []byte) {
	key = make([]byte, 32)
	for i := range key {
		key[i] = byte(0xff - i*5) // first byte 0xff => '/' in std base64, '_' in URL-safe
	}
	nonce = make([]byte, 12)
	for i := range nonce {
		nonce[i] = byte(i + 1)
	}
	data = append([]byte("%PDF-1.4\x00\x01\x02\n"), bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef, 0x00, '\n'}, 60)...)
	return
}

func buildVector(t *testing.T) vectorFile {
	t.Helper()
	key, nonce, data := vectorInputs()
	const name, mime, text = "비밀 문서.bin", "application/octet-stream", "plain secret ✓ — 한글"

	envelope, err := PackFile(name, mime, data)
	if err != nil {
		t.Fatal(err)
	}
	fileEnc, err := seal(key, nonce, envelope)
	if err != nil {
		t.Fatal(err)
	}
	textEnc, err := seal(key, bytesWithLastByteChanged(nonce), []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	keyStd := base64.StdEncoding.EncodeToString(key)
	if !strings.Contains(keyStd, "/") || !strings.Contains(fileEnc.Key, "_") {
		t.Fatal("vector key must exercise the URL-safe alphabet")
	}

	return vectorFile{
		KeyURL:         fileEnc.Key,
		KeyStd:         keyStd,
		IV:             fileEnc.IV,
		FileName:       name,
		FileType:       mime,
		FileDataB64:    base64.StdEncoding.EncodeToString(data),
		FileCiphertext: fileEnc.Ciphertext,
		TextPlain:      text,
		TextCiphertext: textEnc.Ciphertext + "|" + textEnc.IV,
		Sanitize: []sanitizeCase{
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
		},
		Unpack: unpackCases(),
	}
}

// The text message uses a different nonce so (key, nonce) is not reused.
func bytesWithLastByteChanged(n []byte) []byte {
	out := append([]byte(nil), n...)
	out[len(out)-1] ^= 0xff
	return out
}

// TestWriteVector regenerates testdata/vector.json. Run only when the format
// intentionally changes: VANISH_WRITE_VECTOR=1 go test ./crypto -run TestWriteVector
func TestWriteVector(t *testing.T) {
	if os.Getenv("VANISH_WRITE_VECTOR") != "1" {
		t.Skip("set VANISH_WRITE_VECTOR=1 to regenerate the vector")
	}
	b, err := json.MarshalIndent(buildVector(t), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vectorPath, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVector(t *testing.T) {
	raw, err := os.ReadFile(vectorPath)
	if err != nil {
		t.Fatalf("read vector: %v", err)
	}
	var v vectorFile
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}

	// The committed file must equal what the current code produces.
	if want := buildVector(t); want.FileCiphertext != v.FileCiphertext || want.KeyURL != v.KeyURL {
		t.Fatal("vector.json is stale; regenerate with VANISH_WRITE_VECTOR=1")
	}

	data, _ := base64.StdEncoding.DecodeString(v.FileDataB64)
	for _, key := range []string{v.KeyURL, v.KeyStd} {
		plain, err := DecryptBytes(v.FileCiphertext, v.IV, key)
		if err != nil {
			t.Fatalf("decrypt with key %q: %v", key, err)
		}
		p, err := Unpack(plain)
		if err != nil {
			t.Fatal(err)
		}
		if p.Kind != KindFile || p.Name != v.FileName || p.Type != v.FileType || !bytes.Equal(p.Data, data) {
			t.Fatalf("unexpected file payload %+v", p)
		}
	}

	parts := strings.SplitN(v.TextCiphertext, "|", 2)
	plain, err := DecryptBytes(parts[0], parts[1], v.KeyURL)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := Unpack(plain)
	if p.Kind != KindText || p.Text != v.TextPlain {
		t.Fatalf("unexpected text payload %+v", p)
	}

	for _, c := range v.Sanitize {
		if got := SanitizeFilename(c.In); got != c.Want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.In, got, c.Want)
		}
	}

	if len(v.Unpack) == 0 {
		t.Fatal("vector has no unpack cases")
	}
	for _, c := range v.Unpack {
		in, err := base64.StdEncoding.DecodeString(c.InputB64)
		if err != nil {
			t.Fatalf("%s: bad input_b64: %v", c.Name, err)
		}
		p, err := Unpack(in)
		switch c.Kind {
		case "error":
			if !errors.Is(err, ErrSizeMismatch) {
				t.Errorf("%s: want ErrSizeMismatch, got %+v, %v", c.Name, p, err)
			}
		case "text":
			if err != nil || p.Kind != KindText || p.Text != string(in) {
				t.Errorf("%s: want text, got %+v, %v", c.Name, p, err)
			}
		case "file":
			data, derr := base64.StdEncoding.DecodeString(c.DataB64)
			if derr != nil {
				t.Fatalf("%s: bad data_b64: %v", c.Name, derr)
			}
			if err != nil || p.Kind != KindFile || p.Name != c.FileName || p.Type != c.FileType || !bytes.Equal(p.Data, data) {
				t.Errorf("%s: want file %q/%q, got %+v, %v", c.Name, c.FileName, c.FileType, p, err)
			}
		default:
			t.Fatalf("%s: unknown kind %q", c.Name, c.Kind)
		}
	}
}
