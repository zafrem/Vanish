package crypto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const vectorPath = "testdata/vector.json"

type sanitizeCase struct {
	In   string `json:"in"`
	Want string `json:"want"`
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
}
