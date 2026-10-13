# File Sharing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a sender attach a file (≤ 5 MiB) to a Vanish message from the web UI or the CLI, with the same end-to-end encryption and burn-on-read behavior as text.

**Architecture:** A file message is an ordinary message whose plaintext is an envelope (`VNSHFILE1\n` + JSON header + raw bytes) that is encrypted with the existing AES-256-GCM scheme. The backend API, `Message` struct and Redis schema do not change; the backend only gains a request-body size limit. `shared/crypto` (Go) and `frontend/src/lib` (JS) implement the same envelope and are kept honest by one shared test vector.

**Tech Stack:** Go (`shared/`, `cli/`, `backend/` with Gin + testify), React 18 + Vite + Vitest (jsdom), Web Crypto API, nginx.

**Spec:** `docs/superpowers/specs/2026-10-13-file-sharing-design.md`

## Global Constraints

- Max file size: 5 MiB = 5,242,880 bytes; empty files are rejected.
- Envelope magic is exactly `VNSHFILE1\n`; header is one line of JSON `{"name","type","size"}` followed by `\n`; header limit 4096 bytes.
- AES-256-GCM, 96-bit IV, base64 (standard alphabet) ciphertext and IV. Go-generated keys stay `base64.URLEncoding`; JS-generated keys stay standard base64.
- The encryption key stays in the URL fragment. Never move it to path, query or body.
- Retrieved text is never rendered into the DOM; decrypted file bytes never enter React state or the DOM.
- Text messages are unchanged byte-for-byte (old links and old clients keep working).
- No middleware may log request bodies. No filename, MIME type or content may reach PostgreSQL metadata, notifications or logs.
- Module path prefixes differ: `backend`/`cli` use `github.com/milkiss/vanish/...`, `shared` uses `github.com/zafrem/vanish/shared`. Work inside one module directory at a time (no `go.work`).
- Backend create-request body limit: 8 MiB (`8 << 20`); nginx `/api/` limit: `8m`.
- Work on a feature branch (e.g. `feat/file-sharing`), not on `main`. Commit per task as shown; do not push or open a PR without the repo owner's confirmation. The working tree has an unrelated uncommitted `.gitignore` change: never `git add -A`, stage only the files each task names.

## Review Focus

1. A text message that starts with `VNSHFILE1\n` but has no valid header must be shown as text, not error (Task 2, Task 8).
2. A malicious filename in the envelope (`../../evil`, `C:\x\y`, control characters, 300 characters, empty) must never write outside the destination directory or produce an empty/oversized name (Tasks 2, 5, 8, 10).
3. Boundary sizes: 0 bytes rejected, exactly 5 MiB accepted, 5 MiB + 1 rejected, in Go, JS and the UI (Tasks 2, 4, 8, 9).
4. A CLI-generated link whose key contains `-`/`_` must open in the browser (Tasks 7, 8).
5. A failed save after the message is burned must not silently lose the file: CLI pre-checks the destination and never overwrites by accident; the web UI offers "Save again" (Tasks 5, 10).

---

### Task 1: Byte-oriented crypto in `shared/crypto`

**Files:**
- Modify: `shared/crypto/crypto.go` (full rewrite below)
- Test: `shared/crypto/crypto_test.go` (append)

**Interfaces:**
- Produces:
  - `func EncryptBytes(plaintext []byte) (*EncryptedMessage, error)`
  - `func DecryptBytes(ciphertext, iv, keyStr string) ([]byte, error)`
  - `func seal(key, nonce, plaintext []byte) (*EncryptedMessage, error)` (unexported; used by the vector test in Task 2)
  - `EncryptMessage` / `DecryptMessage` keep their signatures and behavior.

- [ ] **Step 1: Write the failing test** — append to `shared/crypto/crypto_test.go`:

```go
func TestEncryptDecryptBytesRoundTrip(t *testing.T) {
	plain := []byte{0x00, 0x01, 0xff, '\n', 0x00, 0xde, 0xad, 0xbe, 0xef}

	enc, err := EncryptBytes(plain)
	if err != nil {
		t.Fatalf("EncryptBytes: %v", err)
	}
	got, err := DecryptBytes(enc.Ciphertext, enc.IV, enc.Key)
	if err != nil {
		t.Fatalf("DecryptBytes: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("round trip mismatch: got %x want %x", got, plain)
	}
}

func TestDecryptBytesWrongKeyFails(t *testing.T) {
	a, _ := EncryptBytes([]byte("secret"))
	b, _ := EncryptBytes([]byte("other"))
	if _, err := DecryptBytes(a.Ciphertext, a.IV, b.Key); err == nil {
		t.Fatal("expected error decrypting with the wrong key")
	}
}

func TestTextWrappersMatchBytes(t *testing.T) {
	enc, err := EncryptMessage("héllo 🔒")
	if err != nil {
		t.Fatal(err)
	}
	b, err := DecryptBytes(enc.Ciphertext, enc.IV, enc.Key)
	if err != nil || string(b) != "héllo 🔒" {
		t.Fatalf("bytes decrypt of text message: %q, %v", b, err)
	}
	s, err := DecryptMessage(enc.Ciphertext, enc.IV, enc.Key)
	if err != nil || s != "héllo 🔒" {
		t.Fatalf("DecryptMessage: %q, %v", s, err)
	}
}
```

Add `"bytes"` to the import block of `crypto_test.go` (it currently imports `"encoding/base64"` and `"testing"`).

- [ ] **Step 2: Run to verify it fails**

Run (from `shared/`): `go test ./crypto/ -run 'Bytes|TextWrappers' -v`
Expected: FAIL — `undefined: EncryptBytes` / `undefined: DecryptBytes`.

- [ ] **Step 3: Replace `shared/crypto/crypto.go` with:**

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const (
	keySize   = 32 // AES-256
	nonceSize = 12 // 96-bit IV
)

// EncryptedMessage represents the encrypted data structure
type EncryptedMessage struct {
	Ciphertext string
	IV         string
	Key        string
}

// EncryptMessage encrypts a plaintext message using AES-256-GCM
// Returns the encrypted message with ciphertext, IV, and key
func EncryptMessage(plaintext string) (*EncryptedMessage, error) {
	return EncryptBytes([]byte(plaintext))
}

// EncryptBytes encrypts arbitrary bytes using AES-256-GCM with a fresh random
// key and IV. Ciphertext and IV are standard base64; the key is URL-safe base64.
func EncryptBytes(plaintext []byte) (*EncryptedMessage, error) {
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return seal(key, nonce, plaintext)
}

// seal encrypts with a caller-supplied key and nonce. Callers must never reuse
// a (key, nonce) pair; it exists so tests can build deterministic vectors.
func seal(key, nonce, plaintext []byte) (*EncryptedMessage, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, fmt.Errorf("invalid nonce size %d", len(nonce))
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	return &EncryptedMessage{
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		IV:         base64.StdEncoding.EncodeToString(nonce),
		Key:        base64.URLEncoding.EncodeToString(key),
	}, nil
}

// DecryptMessage decrypts a message encrypted with EncryptMessage
// This is provided for completeness but may not be used by CLI/MCP
// (decryption typically happens in the frontend)
func DecryptMessage(ciphertext, iv, keyStr string) (string, error) {
	plaintext, err := DecryptBytes(ciphertext, iv, keyStr)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// DecryptBytes decrypts a message encrypted with EncryptBytes (or by the
// frontend) and returns the raw plaintext bytes. The key may be URL-safe or
// standard base64.
func DecryptBytes(ciphertext, iv, keyStr string) ([]byte, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	ivBytes, err := base64.StdEncoding.DecodeString(iv)
	if err != nil {
		return nil, fmt.Errorf("failed to decode IV: %w", err)
	}

	// Key might be base64url encoded (URL-safe)
	keyBytes, err := base64.URLEncoding.DecodeString(keyStr)
	if err != nil {
		// Try standard base64 if URL encoding fails
		keyBytes, err = base64.StdEncoding.DecodeString(keyStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %w", err)
		}
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, ivBytes, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}
```

- [ ] **Step 4: Run all crypto tests**

Run (from `shared/`): `go test ./crypto/ -v`
Expected: PASS (new and pre-existing tests).

- [ ] **Step 5: Commit**

```bash
git add shared/crypto/crypto.go shared/crypto/crypto_test.go
git commit -m "feat(shared): add byte-oriented AES-GCM encrypt/decrypt"
```

---

### Task 2: Envelope, filename sanitizing and the shared test vector

**Files:**
- Create: `shared/crypto/envelope.go`
- Create: `shared/crypto/envelope_test.go`
- Create: `shared/crypto/vector_test.go`
- Create (generated, committed): `shared/crypto/testdata/vector.json`

**Interfaces:**
- Consumes: `seal`, `DecryptBytes` from Task 1.
- Produces:
  - `const EnvelopeMagic = "VNSHFILE1\n"`, `const MaxFileBytes = 5 * 1024 * 1024`
  - `var ErrFileEmpty, ErrFileTooLarge, ErrSizeMismatch error`
  - `type Kind string` with `KindText`, `KindFile`
  - `type Payload struct { Kind Kind; Name, Type string; Data []byte; Text string }`
  - `func PackFile(name, mime string, data []byte) ([]byte, error)`
  - `func Unpack(plain []byte) (*Payload, error)`
  - `func SanitizeFilename(name string) string`
  - `testdata/vector.json` fields: `key_url`, `key_std`, `iv`, `file_name`, `file_type`, `file_data_b64`, `file_ciphertext`, `text_plain`, `text_ciphertext`, `sanitize: [{in, want}]`. Consumed by the frontend tests in Tasks 7–8.

- [ ] **Step 1: Write the failing tests** — `shared/crypto/envelope_test.go`:

```go
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
		"magic only":          EnvelopeMagic,
		"no newline":          EnvelopeMagic + `{"name":"a","type":"b","size":1}`,
		"not json":            EnvelopeMagic + "not json\nrest",
		"empty object":        EnvelopeMagic + "{}\nx",
		"missing size":        EnvelopeMagic + `{"name":"a","type":"b"}` + "\nx",
		"negative size":       EnvelopeMagic + `{"name":"a","type":"b","size":-1}` + "\nx",
		"header too long":     EnvelopeMagic + strings.Repeat("x", 5000) + "\nrest",
		"name not a string":   EnvelopeMagic + `{"name":1,"type":"b","size":1}` + "\nx",
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
```

`shared/crypto/vector_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run (from `shared/`): `go test ./crypto/ 2>&1 | head`
Expected: build FAIL — `undefined: PackFile`, `Unpack`, `SanitizeFilename`, etc.

- [ ] **Step 3: Create `shared/crypto/envelope.go`:**

```go
package crypto

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	// EnvelopeMagic prefixes the plaintext of a file message.
	EnvelopeMagic = "VNSHFILE1\n"
	// MaxFileBytes is the largest file that can be sent (5 MiB).
	MaxFileBytes = 5 * 1024 * 1024

	maxHeaderBytes   = 4096
	maxFilenameBytes = 255
	defaultFilename  = "vanish-file"
)

var (
	ErrFileEmpty    = errors.New("file is empty")
	ErrFileTooLarge = errors.New("file is too large (max 5 MiB)")
	ErrSizeMismatch = errors.New("corrupt file: size does not match header")
)

// Kind says whether a decrypted message is plain text or a file.
type Kind string

const (
	KindText Kind = "text"
	KindFile Kind = "file"
)

// Payload is a decoded message. Text is set for KindText; Name, Type and Data
// for KindFile. Name is the sender-supplied name and MUST go through
// SanitizeFilename before it touches the filesystem.
type Payload struct {
	Kind Kind
	Name string
	Type string
	Data []byte
	Text string
}

type fileHeader struct {
	Name *string `json:"name"`
	Type *string `json:"type"`
	Size *int    `json:"size"`
}

type fileHeaderOut struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

// PackFile builds the plaintext envelope for a file message.
func PackFile(name, mime string, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrFileEmpty
	}
	if len(data) > MaxFileBytes {
		return nil, ErrFileTooLarge
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	hdr, err := json.Marshal(fileHeaderOut{Name: name, Type: mime, Size: len(data)})
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(EnvelopeMagic)+len(hdr)+1+len(data))
	out = append(out, EnvelopeMagic...)
	out = append(out, hdr...)
	out = append(out, '\n')
	out = append(out, data...)
	return out, nil
}

// Unpack decodes decrypted plaintext. Anything that is not a well-formed
// envelope header is treated as plain text. A well-formed header whose size
// does not match the data is an error.
func Unpack(plain []byte) (*Payload, error) {
	text := &Payload{Kind: KindText, Text: string(plain)}

	if !bytes.HasPrefix(plain, []byte(EnvelopeMagic)) {
		return text, nil
	}
	rest := plain[len(EnvelopeMagic):]
	nl := bytes.IndexByte(rest, '\n')
	if nl < 0 || nl > maxHeaderBytes {
		return text, nil
	}

	var h fileHeader
	if err := json.Unmarshal(rest[:nl], &h); err != nil {
		return text, nil
	}
	if h.Name == nil || h.Type == nil || h.Size == nil || *h.Size < 0 {
		return text, nil
	}

	data := rest[nl+1:]
	if len(data) != *h.Size {
		return nil, ErrSizeMismatch
	}
	return &Payload{Kind: KindFile, Name: *h.Name, Type: *h.Type, Data: data}, nil
}

// SanitizeFilename reduces a sender-supplied name to a safe base name.
// Identical rules exist in frontend/src/lib/fileEnvelope.js.
func SanitizeFilename(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}

	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f || r == utf8.RuneError || strings.ContainsRune(`:*?"<>|`, r) {
			continue
		}
		b.WriteRune(r)
	}

	s := strings.Trim(b.String(), " .")
	if len(s) > maxFilenameBytes {
		// Keep whole runes only, up to maxFilenameBytes bytes.
		cut := 0
		for _, r := range s {
			size := utf8.RuneLen(r)
			if cut+size > maxFilenameBytes {
				break
			}
			cut += size
		}
		s = strings.Trim(s[:cut], " .")
	}
	if s == "" {
		return defaultFilename
	}
	return s
}
```

- [ ] **Step 4: Generate the vector and run all tests**

Run (from `shared/`):
```bash
VANISH_WRITE_VECTOR=1 go test ./crypto/ -run TestWriteVector -v
go test ./crypto/ -v
```
Expected: `testdata/vector.json` is created; all tests PASS. If `TestSanitizeFilename` fails on the 255-byte cases, fix the truncation loop until both the `a`×300 and `가`×100 cases pass (85 × 3 bytes = 255 exactly). Regenerate the vector only if you change the format intentionally.

- [ ] **Step 5: Commit**

```bash
git add shared/crypto/envelope.go shared/crypto/envelope_test.go shared/crypto/vector_test.go shared/crypto/testdata/vector.json
git commit -m "feat(shared): add file envelope, filename sanitizing and shared test vector"
```

---

### Task 3: `ParseMessageURL` in `shared/client`

**Files:**
- Create: `shared/client/url.go`
- Test: `shared/client/url_test.go`

**Interfaces:**
- Produces: `func ParseMessageURL(raw string) (id, key string, err error)` — accepts `http(s)://host[:port][/prefix]/m/<id>#<key>`.

- [ ] **Step 1: Write the failing test** — `shared/client/url_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run (from `shared/`): `go test ./client/ -run ParseMessageURL -v`
Expected: FAIL — `undefined: ParseMessageURL`.

- [ ] **Step 3: Create `shared/client/url.go`:**

```go
package client

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseMessageURL extracts the message ID and encryption key from a Vanish
// link of the form {base}/m/{id}#{key}.
func ParseMessageURL(raw string) (id, key string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("invalid link: %w", err)
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if n := len(segments); n >= 2 && segments[n-2] == "m" && segments[n-1] != "" {
		id = segments[n-1]
	} else {
		return "", "", fmt.Errorf("invalid link: expected .../m/<id>#<key>")
	}

	if u.Fragment == "" {
		return "", "", fmt.Errorf("invalid link: missing encryption key after '#'")
	}
	return id, u.Fragment, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run (from `shared/`): `go test ./client/ -v && go vet ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add shared/client/url.go shared/client/url_test.go
git commit -m "feat(shared): add ParseMessageURL"
```

---

### Task 4: `vanish send --file`

**Files:**
- Create: `cli/filesend.go`
- Create: `cli/filesend_test.go`
- Modify: `cli/main.go` (flag parsing + `runSend`)

**Interfaces:**
- Consumes: `crypto.PackFile`, `crypto.EncryptBytes`, `crypto.MaxFileBytes` (Tasks 1–2).
- Produces:
  - `func loadFileEnvelope(path string) ([]byte, error)` — validates, reads and packs a file.
  - `func parseInterspersed(fs *flag.FlagSet, args []string, firstOnly bool) ([]string, error)` — also used by Task 5.
  - CLI: `vanish send [flags] <email> [flags] [message...]` with `-file <path>`.

- [ ] **Step 1: Write the failing test** — `cli/filesend_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run (from `cli/`): `go test -run 'LoadFileEnvelope|ParseInterspersed' ./... -v`
Expected: build FAIL — `undefined: loadFileEnvelope`, `parseInterspersed`.

- [ ] **Step 3: Create `cli/filesend.go`:**

```go
package main

import (
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/zafrem/vanish/shared/crypto"
)

// loadFileEnvelope validates the file (non-empty, regular, within the size
// limit) before reading it, and returns the packed plaintext envelope.
func loadFileEnvelope(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", path)
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("%q is empty", path)
	}
	if info.Size() > crypto.MaxFileBytes {
		return nil, fmt.Errorf("%q is %d bytes; the limit is %d bytes (5 MiB)", path, info.Size(), crypto.MaxFileBytes)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q: %w", path, err)
	}

	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return crypto.PackFile(filepath.Base(path), mimeType, data)
}

// parseInterspersed parses flags that may appear before or between
// positional arguments. With firstOnly, flags are parsed up to and just after
// the first positional argument (the recipient) and everything else is kept as
// positional text, so a free-form message is not mistaken for flags.
func parseInterspersed(fs *flag.FlagSet, args []string, firstOnly bool) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
		if firstOnly && len(positional) == 1 {
			if err := fs.Parse(args); err != nil {
				return nil, err
			}
			return append(positional, fs.Args()...), nil
		}
	}
}
```

- [ ] **Step 4: Wire into `cli/main.go`.**

Replace the flag setup and switch in `main()`:

```go
func main() {
	configCmd := flag.NewFlagSet("config", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)

	// Send flags
	ttl := sendCmd.Int64("ttl", 86400, "Time to live in seconds (default 24h)")
	filePath := sendCmd.String("file", "", "Send this file (max 5 MiB) instead of a text message")

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "config":
		configCmd.Parse(os.Args[2:])
		runConfig()
	case "send":
		args, _ := parseInterspersed(sendCmd, os.Args[2:], true)
		runSend(args, *ttl, *filePath)
	default:
		printHelp()
		os.Exit(1)
	}
}
```

Update `printHelp`:

```go
func printHelp() {
	fmt.Println("Vanish CLI Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  vanish config                         Configure the CLI (interactive)")
	fmt.Println("  vanish send <email> [msg]             Send a secret to a user")
	fmt.Println("  vanish send <email> -file <path>      Send a file (max 5 MiB)")
	fmt.Println()
	fmt.Println("Flags for send:")
	fmt.Println("  -ttl <seconds>                        Expiration time (default 86400)")
	fmt.Println("  -file <path>                          File to send instead of a message")
}
```

Replace `runSend` with:

```go
func runSend(args []string, ttl int64, filePath string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\nRun 'vanish config' first.\n", err)
		os.Exit(1)
	}

	if len(args) < 1 {
		fmt.Println("Usage: vanish send <email> [message]  |  vanish send <email> -file <path>")
		os.Exit(1)
	}

	recipientEmail := args[0]
	var payload []byte

	if filePath != "" {
		if len(args) > 1 {
			fmt.Println("Error: -file cannot be combined with an inline message")
			os.Exit(1)
		}
		payload, err = loadFileEnvelope(filePath)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		var secret string
		if len(args) > 1 {
			secret = strings.Join(args[1:], " ")
		} else {
			// Read from stdin
			info, err := os.Stdin.Stat()
			if err != nil {
				fmt.Printf("Error checking stdin: %v\n", err)
				os.Exit(1)
			}

			if (info.Mode() & os.ModeCharDevice) == 0 {
				// Data is being piped
				bytes, err := io.ReadAll(os.Stdin)
				if err != nil {
					fmt.Printf("Error reading stdin: %v\n", err)
					os.Exit(1)
				}
				secret = string(bytes)
			} else {
				// Prompt user
				fmt.Print("Enter secret: ")
				reader := bufio.NewReader(os.Stdin)
				secret, _ = reader.ReadString('\n')
			}
		}
		secret = strings.TrimSpace(secret)
		if secret == "" {
			fmt.Println("Error: Secret message cannot be empty")
			os.Exit(1)
		}
		payload = []byte(secret)
	}

	// Create API client
	apiClient := client.NewClient(cfg)

	// 1. Find User ID
	recipientID, err := apiClient.FindUserByEmail(recipientEmail)
	if err != nil {
		fmt.Printf("Error finding user: %v\n", err)
		os.Exit(1)
	}

	// 2. Encrypt Message
	encrypted, err := crypto.EncryptBytes(payload)
	if err != nil {
		fmt.Printf("Error encrypting message: %v\n", err)
		os.Exit(1)
	}

	// 3. Send to API
	url, _, err := apiClient.SendMessage(recipientID, encrypted, ttl)
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
		os.Exit(1)
	}

	// 4. Notify
	fmt.Println("✓ Secret created successfully!")
	fmt.Printf("🔗 %s\n", url)

	fmt.Println("\nAttempting to send Slack notification...")
	if err := apiClient.SendSlackNotification(recipientID, url); err != nil {
		fmt.Printf("Could not auto-send Slack notification: %v\n", err)
	} else {
		fmt.Println("✓ Notification sent via Slack")
	}
}
```

- [ ] **Step 5: Run the full CLI checks**

Run (from `cli/`): `go build ./... && go vet ./... && go test ./... -v`
Expected: PASS, including pre-existing `api_test.go`.

- [ ] **Step 6: Commit**

```bash
git add cli/filesend.go cli/filesend_test.go cli/main.go
git commit -m "feat(cli): send files with -file"
```

---

### Task 5: `vanish get`

**Files:**
- Create: `cli/get.go`
- Create: `cli/get_test.go`
- Modify: `cli/main.go` (new `get` subcommand + help text)

**Interfaces:**
- Consumes: `client.ParseMessageURL` (Task 3), `crypto.DecryptBytes`, `crypto.Unpack`, `crypto.SanitizeFilename` (Tasks 1–2), `parseInterspersed` (Task 4), `client.Client.GetMessage`.
- Produces: `type getOptions struct { URL, Out string; Force bool }` and `func runGet(api *client.Client, opts getOptions, stdout io.Writer) (savedPath string, err error)`; CLI `vanish get [-o path] [-force] <url>`.
- Behavior: text → written to `stdout`, `savedPath == ""`. File → written with mode `0600`. Explicit `-o` is validated **before** the message is fetched (burned). Without `-o` the sanitized name in the current directory is used; if it exists and `-force` is not set, a free name `name-1.ext`, `name-2.ext`… is chosen so the file is never lost or overwritten.

- [ ] **Step 1: Write the failing tests** — `cli/get_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	if info, _ := os.Stat(out); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
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

func TestGetBadLink(t *testing.T) {
	c, _, hits := fakeServer(t, []byte("x"))
	if _, err := runGet(c, getOptions{URL: "https://vanish.example/m/m1"}, &bytes.Buffer{}); err == nil {
		t.Fatal("link without key must fail")
	}
	if *hits != 0 {
		t.Fatal("must not fetch with an invalid link")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run (from `cli/`): `go test -run TestGet ./... -v`
Expected: build FAIL — `undefined: runGet`, `getOptions`.

- [ ] **Step 3: Create `cli/get.go`:**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zafrem/vanish/shared/client"
	"github.com/zafrem/vanish/shared/crypto"
)

type getOptions struct {
	URL   string
	Out   string
	Force bool
}

// checkOutputPath validates an explicit output path BEFORE the message is
// fetched, because fetching burns the message.
func checkOutputPath(out string, force bool) error {
	if out == "" {
		return nil
	}
	dir := filepath.Dir(out)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("output directory %q does not exist", dir)
	}
	if info, err := os.Stat(out); err == nil {
		if info.IsDir() {
			return fmt.Errorf("output path %q is a directory", out)
		}
		if !force {
			return fmt.Errorf("%q already exists (use -force to overwrite)", out)
		}
	}
	return nil
}

func writeFile(path string, data []byte, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// freePath returns path if it does not exist, otherwise name-1.ext, name-2.ext...
func freePath(path string) string {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// runGet fetches (and thereby burns) a message, decrypts it and either prints
// the text to stdout or saves the file. It returns the saved path for files.
func runGet(api *client.Client, opts getOptions, stdout io.Writer) (string, error) {
	id, key, err := client.ParseMessageURL(opts.URL)
	if err != nil {
		return "", err
	}
	if err := checkOutputPath(opts.Out, opts.Force); err != nil {
		return "", err
	}

	msg, err := api.GetMessage(id)
	if err != nil {
		return "", err
	}
	plain, err := crypto.DecryptBytes(msg.Ciphertext, msg.IV, key)
	if err != nil {
		return "", err
	}
	payload, err := crypto.Unpack(plain)
	if err != nil {
		return "", err
	}

	if payload.Kind == crypto.KindText {
		_, err := io.WriteString(stdout, payload.Text)
		return "", err
	}

	target := opts.Out
	if target == "" {
		target = crypto.SanitizeFilename(payload.Name)
		if !opts.Force {
			target = freePath(target)
		}
	}
	if err := writeFile(target, payload.Data, opts.Force); err != nil {
		return "", fmt.Errorf("the message was already burned but writing %q failed: %w", target, err)
	}
	return target, nil
}
```

- [ ] **Step 4: Run the `get` tests**

Run (from `cli/`): `go test -run TestGet ./... -v`
Expected: PASS.

- [ ] **Step 5: Add the subcommand to `cli/main.go`.**

In `main()`, after `filePath := ...`, add:

```go
	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	getOut := getCmd.String("o", "", "Write a received file to this path")
	getForce := getCmd.Bool("force", false, "Overwrite the output file if it exists")
```

Add this case to the `switch`:

```go
	case "get":
		args, _ := parseInterspersed(getCmd, os.Args[2:], false)
		runGetCommand(args, *getOut, *getForce)
```

Append to `main.go`:

```go
func runGetCommand(args []string, out string, force bool) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\nRun 'vanish config' first.\n", err)
		os.Exit(1)
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: vanish get [-o path] [-force] <link>")
		os.Exit(1)
	}

	saved, err := runGet(client.NewClient(cfg), getOptions{URL: args[0], Out: out, Force: force}, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if saved != "" {
		fmt.Fprintf(os.Stderr, "✓ Saved %s (the message has been destroyed)\n", saved)
	}
}
```

Extend `printHelp` with these lines (after the `send` lines, before the flags section):

```go
	fmt.Println("  vanish get <link>                     Read a secret (text to stdout, files saved)")
	fmt.Println("  vanish get -o <path> [-force] <link>  Save a received file to <path>")
```

- [ ] **Step 6: Full CLI checks**

Run (from `cli/`): `go build ./... && go vet ./... && go test ./... -race`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add cli/get.go cli/get_test.go cli/main.go
git commit -m "feat(cli): add 'get' command for receiving text and files"
```

---

### Task 6: Backend request-body limit (+ nginx)

**Files:**
- Modify: `backend/internal/api/middleware.go` (add `BodyLimit`, `IsBodyTooLarge`, constant)
- Modify: `backend/internal/api/routes.go:76` (apply to `POST /messages`)
- Modify: `backend/internal/api/handlers.go:42-47` (map body-too-large to 413)
- Modify: `frontend/nginx.conf` (`/api/` location)
- Test: `backend/tests/unit/bodylimit_test.go`

**Interfaces:**
- Produces: `const MaxMessageBodyBytes = 8 << 20`, `func BodyLimit(max int64) gin.HandlerFunc`, `func IsBodyTooLarge(err error) bool` in package `api`.

- [ ] **Step 1: Write the failing test** — `backend/tests/unit/bodylimit_test.go`:

```go
package unit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/milkiss/vanish/backend/internal/api"
	"github.com/stretchr/testify/assert"
)

func bodyLimitRouter(called *bool, tooLarge *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/m", api.BodyLimit(1024), func(c *gin.Context) {
		*called = true
		var v struct {
			A string `json:"a"`
		}
		if err := c.ShouldBindJSON(&v); err != nil {
			*tooLarge = api.IsBodyTooLarge(err)
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusCreated)
	})
	return r
}

func TestBodyLimitRejectsOversizedContentLength(t *testing.T) {
	var called, tooLarge bool
	r := bodyLimitRouter(&called, &tooLarge)

	body := `{"a":"` + strings.Repeat("x", 2000) + `"}`
	req := httptest.NewRequest("POST", "/m", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.False(t, called, "handler must not run")
}

func TestBodyLimitStopsUnknownLengthBodies(t *testing.T) {
	var called, tooLarge bool
	r := bodyLimitRouter(&called, &tooLarge)

	body := io.MultiReader(strings.NewReader(`{"a":"`), strings.NewReader(strings.Repeat("x", 5000)), strings.NewReader(`"}`))
	req := httptest.NewRequest("POST", "/m", body) // ContentLength == -1
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.True(t, called)
	assert.True(t, tooLarge, "bind error must be recognised as body-too-large")
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestBodyLimitAllowsSmallBodies(t *testing.T) {
	var called, tooLarge bool
	r := bodyLimitRouter(&called, &tooLarge)

	req := httptest.NewRequest("POST", "/m", strings.NewReader(`{"a":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.False(t, tooLarge)
}

func TestMaxMessageBodyBytes(t *testing.T) {
	// 5 MiB file => ~7.0 MB of base64 JSON; the limit must leave headroom.
	assert.Equal(t, int64(8<<20), int64(api.MaxMessageBodyBytes))
}
```

- [ ] **Step 2: Run to verify it fails**

Run (from `backend/`): `go test ./tests/unit/ -run 'BodyLimit|MaxMessageBodyBytes' -v`
Expected: build FAIL — `undefined: api.BodyLimit`.

- [ ] **Step 3: Implement.** In `backend/internal/api/middleware.go` extend the imports with `"errors"`, `"net/http"` and `"github.com/milkiss/vanish/backend/internal/models"`, then append:

```go
// MaxMessageBodyBytes caps the create-message request body. A 5 MiB file
// becomes ~7 MB after AES-GCM and base64, so 8 MiB leaves headroom.
const MaxMessageBodyBytes = 8 << 20

// BodyLimit rejects requests whose body exceeds max bytes. It checks the
// declared Content-Length up front and also caps bodies of unknown length.
// It never reads or logs the body.
func BodyLimit(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > max {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, models.ErrorResponse{
				Error: "Message too large",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max)
		c.Next()
	}
}

// IsBodyTooLarge reports whether err came from exceeding a BodyLimit.
func IsBodyTooLarge(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}
```

In `backend/internal/api/routes.go`, change line 76 to:

```go
				messages.POST("", BodyLimit(MaxMessageBodyBytes), messageHandler.CreateMessage)
```

In `backend/internal/api/handlers.go`, replace the bind-error block in `CreateMessage` (lines 42–47):

```go
	if err := c.ShouldBindJSON(&req); err != nil {
		if IsBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, models.ErrorResponse{
				Error: "Message too large",
			})
			return
		}
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
		})
		return
	}
```

In `frontend/nginx.conf`, add inside `location /api/ { ... }` (first line of the block):

```nginx
        client_max_body_size 8m;
```

- [ ] **Step 4: Run tests and static checks**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./tests/unit/ -run 'BodyLimit|MaxMessageBodyBytes|Middleware' -v`
Expected: PASS. Then: `grep -n "client_max_body_size" ../frontend/nginx.conf` → one match inside `location /api/`.

(Other tests in `tests/unit` need Redis/PostgreSQL; do not treat their absence as a regression.)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/api/middleware.go backend/internal/api/routes.go backend/internal/api/handlers.go backend/tests/unit/bodylimit_test.go frontend/nginx.conf
git commit -m "feat(backend): cap create-message body at 8 MiB and return 413"
```

---

### Task 7: Frontend crypto — bytes API, chunked base64, URL-safe keys

**Files:**
- Modify: `frontend/src/lib/crypto.js`
- Test: `frontend/tests/cryptoBytes.test.js` (new)

**Interfaces:**
- Consumes: `shared/crypto/testdata/vector.json` (Task 2).
- Produces: `encryptBytes(bytes: Uint8Array, key: CryptoKey) -> Promise<{ciphertext, iv}>`, `decryptBytes(ciphertextB64, ivB64, key) -> Promise<Uint8Array>`. `encrypt`/`decrypt` keep their signatures. `importKey` accepts URL-safe and standard base64, padded or not.

> `frontend/node_modules` is not installed in a fresh checkout: run `npm install` in `frontend/` first. `tests/setup.js` replaces `crypto` with a mock, so tests that need real crypto use the `node` environment and `vi.stubGlobal('crypto', webcrypto)`.

- [ ] **Step 1: Write the failing test** — `frontend/tests/cryptoBytes.test.js`:

```js
// @vitest-environment node
import { describe, test, expect, beforeAll, afterAll, vi } from 'vitest';
import { webcrypto } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
  generateKey, exportKey, importKey, encrypt, decrypt, encryptBytes, decryptBytes,
} from '../src/lib/crypto';

const vector = JSON.parse(
  readFileSync(fileURLToPath(new URL('../../shared/crypto/testdata/vector.json', import.meta.url)), 'utf8')
);

beforeAll(() => vi.stubGlobal('crypto', webcrypto));
afterAll(() => vi.unstubAllGlobals());

describe('byte encryption', () => {
  test('round-trips arbitrary bytes including NUL and newlines', async () => {
    const key = await generateKey();
    const data = new Uint8Array([0, 1, 2, 10, 255, 0, 10]);
    const { ciphertext, iv } = await encryptBytes(data, key);
    const out = await decryptBytes(ciphertext, iv, key);
    expect(Array.from(out)).toEqual(Array.from(data));
  });

  test('round-trips a 5 MiB buffer (chunked base64)', async () => {
    const key = await generateKey();
    const data = new Uint8Array(5 * 1024 * 1024).map((_, i) => i % 251);
    const { ciphertext, iv } = await encryptBytes(data, key);
    const out = await decryptBytes(ciphertext, iv, key);
    expect(out.length).toBe(data.length);
    expect(out[123456]).toBe(data[123456]);
  });

  test('text encrypt/decrypt still work', async () => {
    const key = await generateKey();
    const { ciphertext, iv } = await encrypt('héllo 🔒', key);
    expect(await decrypt(ciphertext, iv, key)).toBe('héllo 🔒');
  });

  test('wrong key fails with the existing message', async () => {
    const k1 = await generateKey();
    const k2 = await generateKey();
    const { ciphertext, iv } = await encryptBytes(new Uint8Array([1, 2, 3]), k1);
    await expect(decryptBytes(ciphertext, iv, k2)).rejects.toThrow(/Decryption failed/);
  });
});

describe('key import', () => {
  test('imports a standard-base64 key exported by the browser', async () => {
    const key = await generateKey();
    const again = await importKey(await exportKey(key));
    const { ciphertext, iv } = await encryptBytes(new Uint8Array([9]), key);
    expect(Array.from(await decryptBytes(ciphertext, iv, again))).toEqual([9]);
  });

  test('imports the URL-safe key generated by the Go CLI (contains - or _)', async () => {
    expect(vector.key_url).toMatch(/[-_]/);
    const key = await importKey(vector.key_url);
    const plain = await decryptBytes(vector.file_ciphertext, vector.iv, key);
    expect(new TextDecoder().decode(plain.subarray(0, 10))).toBe('VNSHFILE1\n');
  });

  test('standard and URL-safe encodings of the same key both work', async () => {
    const a = await importKey(vector.key_std);
    const b = await importKey(vector.key_url);
    const pa = await decryptBytes(vector.file_ciphertext, vector.iv, a);
    const pb = await decryptBytes(vector.file_ciphertext, vector.iv, b);
    expect(Array.from(pa)).toEqual(Array.from(pb));
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm install && npx vitest run tests/cryptoBytes.test.js`
Expected: FAIL — `encryptBytes` is not exported; URL-safe import throws (`atob` invalid character).

- [ ] **Step 3: Edit `frontend/src/lib/crypto.js`.**

Replace `encrypt` and `decrypt` (keeping their JSDoc) and add the byte functions:

```js
export async function encrypt(plaintext, key) {
  return encryptBytes(new TextEncoder().encode(plaintext), key);
}

/**
 * Encrypt raw bytes using AES-256-GCM
 * @param {Uint8Array} data - The bytes to encrypt
 * @param {CryptoKey} key - The encryption key
 * @returns {Promise<{ciphertext: string, iv: string}>} Encrypted data and IV
 */
export async function encryptBytes(data, key) {
  // Generate random 96-bit IV (recommended for GCM)
  const iv = crypto.getRandomValues(new Uint8Array(12));

  const ciphertext = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv },
    key,
    data
  );

  return {
    ciphertext: arrayBufferToBase64(ciphertext),
    iv: arrayBufferToBase64(iv)
  };
}

export async function decrypt(ciphertextBase64, ivBase64, key) {
  const bytes = await decryptBytes(ciphertextBase64, ivBase64, key);
  return new TextDecoder().decode(bytes);
}

/**
 * Decrypt ciphertext to raw bytes using AES-256-GCM
 * @param {string} ciphertextBase64 - Base64-encoded ciphertext
 * @param {string} ivBase64 - Base64-encoded IV
 * @param {CryptoKey} key - The decryption key
 * @returns {Promise<Uint8Array>} Decrypted bytes
 */
export async function decryptBytes(ciphertextBase64, ivBase64, key) {
  const ciphertext = base64ToArrayBuffer(ciphertextBase64);
  const iv = base64ToArrayBuffer(ivBase64);

  try {
    const decrypted = await crypto.subtle.decrypt(
      { name: 'AES-GCM', iv },
      key,
      ciphertext
    );
    return new Uint8Array(decrypted);
  } catch (error) {
    throw new Error('Decryption failed. The key may be incorrect or the message corrupted.');
  }
}
```

Replace the two helpers at the bottom:

```js
function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer);
  const CHUNK = 0x8000;
  let binary = '';
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode.apply(null, bytes.subarray(i, i + CHUNK));
  }
  return btoa(binary);
}

/**
 * Convert a base64 or base64url string (padded or not) to ArrayBuffer.
 * The Go CLI generates URL-safe keys, so '-' and '_' must be accepted.
 * @param {string} base64
 * @returns {ArrayBuffer}
 */
function base64ToArrayBuffer(base64) {
  let normalized = base64.replace(/-/g, '+').replace(/_/g, '/');
  while (normalized.length % 4 !== 0) normalized += '=';
  const binary = atob(normalized);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}
```

- [ ] **Step 4: Run all frontend crypto tests**

Run (from `frontend/`): `npx vitest run tests/cryptoBytes.test.js tests/crypto.test.js`
Expected: PASS (both files; the old suite uses the mocked crypto and must still pass).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/crypto.js frontend/tests/cryptoBytes.test.js
git commit -m "feat(frontend): byte encryption, chunked base64, accept URL-safe keys"
```

---

### Task 8: `fileEnvelope.js` and `download.js`

**Files:**
- Create: `frontend/src/lib/fileEnvelope.js`
- Create: `frontend/src/lib/download.js`
- Test: `frontend/tests/fileEnvelope.test.js`, `frontend/tests/download.test.js`

**Interfaces:**
- Consumes: `decryptBytes`, `importKey` (Task 7); `vector.json` (Task 2).
- Produces:
  - `MAX_FILE_BYTES`, `ENVELOPE_MAGIC`
  - `sanitizeFilename(name: string) -> string`
  - `packEnvelope(name: string, type: string, bytes: Uint8Array) -> Uint8Array` (throws `Error('File is empty')` / `Error('File is too large (max 5 MB)')`)
  - `packFile(file: File) -> Promise<Uint8Array>`
  - `unpackEnvelope(bytes: Uint8Array) -> {kind:'file',name,type,data} | {kind:'text',text}` (throws `Error('Corrupt file: size does not match header')` on size mismatch)
  - `validateFileForSend(file: {size:number}) -> string | null` (an error message, or null when OK)
  - `formatBytes(n: number) -> string`
  - `saveBlob(data: Uint8Array, name: string, type?: string) -> () => void` (the returned function saves again)

- [ ] **Step 1: Write the failing tests.**

`frontend/tests/fileEnvelope.test.js`:

```js
// @vitest-environment node
import { describe, test, expect, beforeAll, afterAll, vi } from 'vitest';
import { webcrypto } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { importKey, decryptBytes } from '../src/lib/crypto';
import {
  MAX_FILE_BYTES, sanitizeFilename, packEnvelope, packFile, unpackEnvelope,
  validateFileForSend, formatBytes,
} from '../src/lib/fileEnvelope';

const vector = JSON.parse(
  readFileSync(fileURLToPath(new URL('../../shared/crypto/testdata/vector.json', import.meta.url)), 'utf8')
);

beforeAll(() => vi.stubGlobal('crypto', webcrypto));
afterAll(() => vi.unstubAllGlobals());

const b64 = (s) => Uint8Array.from(Buffer.from(s, 'base64'));
const enc = (s) => new TextEncoder().encode(s);

describe('envelope', () => {
  test('round-trips binary data with NUL and newlines', () => {
    const data = new Uint8Array([0, 10, 255, 0, 10]);
    const packed = packEnvelope('비밀 문서.bin', 'application/octet-stream', data);
    const out = unpackEnvelope(packed);
    expect(out.kind).toBe('file');
    expect(out.name).toBe('비밀 문서.bin');
    expect(out.type).toBe('application/octet-stream');
    expect(Array.from(out.data)).toEqual(Array.from(data));
  });

  test('empty type falls back to application/octet-stream', () => {
    const out = unpackEnvelope(packEnvelope('a', '', new Uint8Array([1])));
    expect(out.type).toBe('application/octet-stream');
  });

  test('size limits: 0 rejected, exactly max accepted, max+1 rejected', () => {
    expect(() => packEnvelope('a', 't', new Uint8Array(0))).toThrow(/empty/i);
    expect(() => packEnvelope('a', 't', new Uint8Array(MAX_FILE_BYTES))).not.toThrow();
    expect(() => packEnvelope('a', 't', new Uint8Array(MAX_FILE_BYTES + 1))).toThrow(/too large/i);
  });

  test('plain text is returned as text', () => {
    for (const s of ['hello', '', 'こんにちは 🔒', 'multi\nline']) {
      expect(unpackEnvelope(enc(s))).toEqual({ kind: 'text', text: s });
    }
  });

  test('magic prefix without a valid header is treated as text', () => {
    const magic = 'VNSHFILE1\n';
    const cases = [
      magic,
      magic + '{"name":"a","type":"b","size":1}',
      magic + 'not json\nrest',
      magic + '{}\nx',
      magic + '{"name":"a","type":"b"}\nx',
      magic + '{"name":"a","type":"b","size":-1}\nx',
      magic + 'x'.repeat(5000) + '\nrest',
      magic + '{"name":1,"type":"b","size":1}\nx',
    ];
    for (const s of cases) {
      expect(unpackEnvelope(enc(s))).toEqual({ kind: 'text', text: s });
    }
  });

  test('valid header with wrong size is an error', () => {
    const bad = enc('VNSHFILE1\n{"name":"a","type":"b","size":5}\nabc');
    expect(() => unpackEnvelope(bad)).toThrow(/size/i);
  });

  test('packFile reads a File', async () => {
    const file = new File([new Uint8Array([1, 2, 3])], 'x.bin', { type: 'application/x-test' });
    const out = unpackEnvelope(await packFile(file));
    expect(out.name).toBe('x.bin');
    expect(out.type).toBe('application/x-test');
    expect(Array.from(out.data)).toEqual([1, 2, 3]);
  });
});

describe('validateFileForSend / formatBytes', () => {
  test('validation messages', () => {
    expect(validateFileForSend({ size: 0 })).toMatch(/empty/i);
    expect(validateFileForSend({ size: MAX_FILE_BYTES + 1 })).toMatch(/5 MB/);
    expect(validateFileForSend({ size: MAX_FILE_BYTES })).toBeNull();
    expect(validateFileForSend({ size: 1 })).toBeNull();
  });

  test('formatBytes', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB');
  });
});

describe('sanitizeFilename (shared cases)', () => {
  test.each(vector.sanitize.map((c) => [c.in, c.want]))('%j -> %j', (input, want) => {
    expect(sanitizeFilename(input)).toBe(want);
  });

  test('never returns path separators or control characters', () => {
    for (const s of ['..\\..\\x', '/etc/passwd', 'a\u0000b', '\u202e', '.', ' ']) {
      const out = sanitizeFilename(s);
      expect(out).not.toMatch(/[\\/\u0000-\u001f]/);
      expect(out.length).toBeGreaterThan(0);
    }
  });
});

describe('Go interoperability (shared vector)', () => {
  test('decrypts and unpacks the file message produced by Go', async () => {
    const key = await importKey(vector.key_url);
    const plain = await decryptBytes(vector.file_ciphertext, vector.iv, key);
    const out = unpackEnvelope(plain);
    expect(out.kind).toBe('file');
    expect(out.name).toBe(vector.file_name);
    expect(out.type).toBe(vector.file_type);
    expect(Array.from(out.data)).toEqual(Array.from(b64(vector.file_data_b64)));
  });

  test('decrypts the text message produced by Go', async () => {
    const [ct, iv] = vector.text_ciphertext.split('|');
    const key = await importKey(vector.key_url);
    const out = unpackEnvelope(await decryptBytes(ct, iv, key));
    expect(out).toEqual({ kind: 'text', text: vector.text_plain });
  });
});
```

`frontend/tests/download.test.js`:

```js
import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import { saveBlob } from '../src/lib/download';

describe('saveBlob', () => {
  let click;
  beforeEach(() => {
    vi.useFakeTimers();
    URL.createObjectURL = vi.fn(() => 'blob:mock');
    URL.revokeObjectURL = vi.fn();
    click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      this.__clicked = { href: this.href, download: this.download };
    });
  });
  afterEach(() => {
    vi.useRealTimers();
    click.mockRestore();
  });

  test('triggers a download with the given name and revokes the URL', () => {
    saveBlob(new Uint8Array([1, 2, 3]), 'a.bin', 'application/octet-stream');
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
    expect(click).toHaveBeenCalledTimes(1);
    expect(click.mock.instances[0].__clicked.download).toBe('a.bin');
    vi.advanceTimersByTime(1500);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock');
    expect(document.querySelectorAll('a[download]').length).toBe(0);
  });

  test('returned function saves again', () => {
    const again = saveBlob(new Uint8Array([1]), 'a.bin');
    again();
    expect(click).toHaveBeenCalledTimes(2);
    expect(URL.createObjectURL).toHaveBeenCalledTimes(2);
  });
});
```

- [ ] **Step 2: Run to verify they fail**

Run (from `frontend/`): `npx vitest run tests/fileEnvelope.test.js tests/download.test.js`
Expected: FAIL — modules not found.

- [ ] **Step 3: Create `frontend/src/lib/fileEnvelope.js`:**

```js
/**
 * File envelope: how a file is carried inside an ordinary encrypted message.
 *
 *   VNSHFILE1\n  {"name":..,"type":..,"size":..}\n  <raw bytes>
 *
 * Mirrors shared/crypto/envelope.go - keep the two in sync. The envelope is
 * the plaintext that gets encrypted, so the server never sees name or type.
 */

export const MAX_FILE_BYTES = 5 * 1024 * 1024;
export const ENVELOPE_MAGIC = 'VNSHFILE1\n';

const MAGIC_BYTES = new TextEncoder().encode(ENVELOPE_MAGIC);
const MAX_HEADER_BYTES = 4096;
const MAX_FILENAME_BYTES = 255;
const DEFAULT_FILENAME = 'vanish-file';
const DEFAULT_TYPE = 'application/octet-stream';

/**
 * Reduce a sender-supplied name to a safe base name (same rules as Go).
 * @param {string} name
 * @returns {string}
 */
export function sanitizeFilename(name) {
  let s = String(name ?? '');
  const slash = Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'));
  if (slash >= 0) s = s.slice(slash + 1);

  s = Array.from(s)
    .filter((ch) => {
      const c = ch.codePointAt(0);
      const surrogate = c >= 0xd800 && c <= 0xdfff;
      return !(c < 0x20 || c === 0x7f || c === 0xfffd || surrogate || ':*?"<>|'.includes(ch));
    })
    .join('');

  const trim = (v) => v.replace(/^[ .]+|[ .]+$/g, '');
  s = trim(s);
  s = trim(truncateUtf8(s, MAX_FILENAME_BYTES));
  return s || DEFAULT_FILENAME;
}

function truncateUtf8(s, maxBytes) {
  const encoder = new TextEncoder();
  let out = '';
  let used = 0;
  for (const ch of s) {
    const len = encoder.encode(ch).length;
    if (used + len > maxBytes) break;
    out += ch;
    used += len;
  }
  return out;
}

/**
 * Message for an unusable file, or null when it can be sent.
 * @param {{size: number}} file
 * @returns {string|null}
 */
export function validateFileForSend(file) {
  if (!file || file.size === 0) return 'File is empty';
  if (file.size > MAX_FILE_BYTES) return 'File is too large (max 5 MB)';
  return null;
}

export function formatBytes(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * Build the plaintext envelope for a file.
 * @param {string} name
 * @param {string} type
 * @param {Uint8Array} bytes
 * @returns {Uint8Array}
 */
export function packEnvelope(name, type, bytes) {
  const problem = validateFileForSend({ size: bytes.length });
  if (problem) throw new Error(problem);

  const header = new TextEncoder().encode(
    JSON.stringify({ name, type: type || DEFAULT_TYPE, size: bytes.length }) + '\n'
  );
  const out = new Uint8Array(MAGIC_BYTES.length + header.length + bytes.length);
  out.set(MAGIC_BYTES, 0);
  out.set(header, MAGIC_BYTES.length);
  out.set(bytes, MAGIC_BYTES.length + header.length);
  return out;
}

/**
 * @param {File} file
 * @returns {Promise<Uint8Array>}
 */
export async function packFile(file) {
  const problem = validateFileForSend(file);
  if (problem) throw new Error(problem);
  const bytes = new Uint8Array(await file.arrayBuffer());
  return packEnvelope(file.name, file.type, bytes);
}

function startsWithMagic(bytes) {
  if (bytes.length < MAGIC_BYTES.length) return false;
  for (let i = 0; i < MAGIC_BYTES.length; i++) {
    if (bytes[i] !== MAGIC_BYTES[i]) return false;
  }
  return true;
}

function parseHeader(rest) {
  const nl = rest.indexOf(10);
  if (nl < 0 || nl > MAX_HEADER_BYTES) return null;
  try {
    const h = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(rest.subarray(0, nl)));
    if (
      h && typeof h.name === 'string' && typeof h.type === 'string' &&
      Number.isInteger(h.size) && h.size >= 0
    ) {
      return { header: h, dataStart: nl + 1 };
    }
  } catch {
    // not an envelope header
  }
  return null;
}

/**
 * Decode decrypted plaintext. Anything that is not a well-formed envelope is
 * text. A well-formed header whose size does not match the data is an error.
 * `name` is sender-supplied: run it through sanitizeFilename before use.
 * @param {Uint8Array} bytes
 * @returns {{kind:'file',name:string,type:string,data:Uint8Array}|{kind:'text',text:string}}
 */
export function unpackEnvelope(bytes) {
  if (startsWithMagic(bytes)) {
    const rest = bytes.subarray(MAGIC_BYTES.length);
    const parsed = parseHeader(rest);
    if (parsed) {
      const data = rest.subarray(parsed.dataStart);
      if (data.length !== parsed.header.size) {
        throw new Error('Corrupt file: size does not match header');
      }
      return { kind: 'file', name: parsed.header.name, type: parsed.header.type, data };
    }
  }
  return { kind: 'text', text: new TextDecoder().decode(bytes) };
}
```

- [ ] **Step 4: Create `frontend/src/lib/download.js`:**

```js
/**
 * Save bytes as a file download without rendering them anywhere.
 * Returns a function that triggers the same download again (useful when a
 * browser blocked the first attempt because it ran after an await).
 * @param {Uint8Array} data
 * @param {string} name - already sanitized
 * @param {string} [type]
 * @returns {() => void}
 */
export function saveBlob(data, name, type = 'application/octet-stream') {
  const blob = new Blob([data], { type });

  const save = () => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    a.style.display = 'none';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  save();
  return save;
}
```

- [ ] **Step 5: Run the tests**

Run (from `frontend/`): `npx vitest run tests/fileEnvelope.test.js tests/download.test.js tests/cryptoBytes.test.js`
Expected: PASS. If a sanitize case differs from Go, fix the JS (the Go side and `vector.json` are the reference).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/fileEnvelope.js frontend/src/lib/download.js frontend/tests/fileEnvelope.test.js frontend/tests/download.test.js
git commit -m "feat(frontend): file envelope, filename sanitizing and download helper"
```

---

### Task 9: Create page — File mode

**Files:**
- Modify: `frontend/src/components/CreateMessage.jsx`
- Test: `frontend/tests/components/CreateMessage.test.jsx` (append tests)

**Interfaces:**
- Consumes: `encryptBytes` (Task 7); `packFile`, `validateFileForSend`, `formatBytes` (Task 8).
- Produces: UI only. Text mode keeps its current labels (`Enter password, API key, or sensitive data...` placeholder, `Create Secret Link` button, `Secret Created!` heading) because existing tests rely on them.

- [ ] **Step 1: Write the failing tests** — append inside the existing `describe('CreateMessage Component', ...)` block in `frontend/tests/components/CreateMessage.test.jsx`:

```jsx
  test('can switch to file mode, which hides the textarea and needs a file', async () => {
    renderComponent();
    await waitFor(() => {
      expect(screen.getByPlaceholderText(/Enter password, API key, or sensitive data/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /^File$/ }));

    expect(screen.queryByPlaceholderText(/Enter password, API key, or sensitive data/i)).not.toBeInTheDocument();
    expect(screen.getByLabelText(/choose a file/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Create Secret Link/i })).toBeDisabled();
  });

  test('rejects an oversized file with a clear error', async () => {
    renderComponent();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^File$/ })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /^File$/ }));

    const input = screen.getByLabelText(/choose a file/i);
    const big = new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'big.bin');
    fireEvent.change(input, { target: { files: [big] } });

    await waitFor(() => {
      expect(screen.getByText(/too large \(max 5 MB\)/i)).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Create Secret Link/i })).toBeDisabled();
  });

  test('shows the selected file name and size, and can remove it', async () => {
    renderComponent();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^File$/ })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /^File$/ }));

    const input = screen.getByLabelText(/choose a file/i);
    fireEvent.change(input, { target: { files: [new File([new Uint8Array(2048)], 'notes.txt')] } });

    expect(await screen.findByText(/notes\.txt/)).toBeInTheDocument();
    expect(screen.getByText(/2\.0 KB/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /remove file/i }));
    expect(screen.queryByText(/notes\.txt/)).not.toBeInTheDocument();
  });
```

- [ ] **Step 2: Run to verify they fail**

Run (from `frontend/`): `npx vitest run tests/components/CreateMessage.test.jsx`
Expected: the 3 new tests FAIL (no File button); pre-existing tests still PASS.

- [ ] **Step 3: Edit `frontend/src/components/CreateMessage.jsx`.**

Imports (lines 1–2):

```jsx
import React, { useState, useEffect, useRef } from 'react';
import { generateKey, exportKey, encrypt, encryptBytes } from '../lib/crypto';
import { packFile, validateFileForSend, formatBytes } from '../lib/fileEnvelope';
```

State (after `const [secretText, ...]`):

```jsx
  const [mode, setMode] = useState('text'); // 'text' | 'file'
  const [file, setFile] = useState(null);
  const [isDragging, setIsDragging] = useState(false);
  const fileInputRef = useRef(null);
```

Add handlers before `handleCreate`:

```jsx
  const acceptFile = (candidate) => {
    if (!candidate) return;
    const problem = validateFileForSend(candidate);
    if (problem) {
      setFile(null);
      setError(problem);
      return;
    }
    setError(null);
    setFile(candidate);
  };

  const handleFileSelected = (e) => acceptFile(e.target.files && e.target.files[0]);

  const handleDrop = (e) => {
    e.preventDefault();
    setIsDragging(false);
    if (isCreating) return;
    acceptFile(e.dataTransfer.files && e.dataTransfer.files[0]);
  };

  const handleRemoveFile = () => {
    setFile(null);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const switchMode = (next) => {
    setMode(next);
    setError(null);
  };
```

In `handleCreate`, replace the validation + encryption part (the lines from `if (!secretText.trim())` through `const { ciphertext, iv } = await encrypt(secretText, key);`) with:

```jsx
      // Validation
      if (mode === 'text' && !secretText.trim()) {
        throw new Error('Please enter a secret message');
      }

      if (mode === 'file' && !file) {
        throw new Error('Please choose a file');
      }

      if (!recipientId) {
        throw new Error('Please select a recipient');
      }

      // Step 1: Generate encryption key (client-side)
      const key = await generateKey();
      const keyString = await exportKey(key);

      // Step 2: Encrypt the message or file (client-side)
      const { ciphertext, iv } = mode === 'file'
        ? await encryptBytes(await packFile(file), key)
        : await encrypt(secretText, key);
```

After `setSecretText('');` in the success path add `setFile(null);`.

In the success screen replace `<h2 ...>Secret Created!</h2>` text only if mode is text? Keep the heading `Secret Created!` (an existing test asserts it) and change only the subtitle line to:

```jsx
            <p className="text-gray-400 text-sm">Share this link securely with the recipient{mode === 'file' ? ' (it opens a file download)' : ''}</p>
```

Replace the "Secret Message" block (the `<div>` containing the `textarea`) with:

```jsx
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="block text-sm font-medium text-gray-300">
                {mode === 'file' ? 'File' : 'Secret Message'}
              </label>
              <div className="inline-flex rounded-lg border border-dark-border overflow-hidden text-sm">
                <button
                  type="button"
                  onClick={() => switchMode('text')}
                  disabled={isCreating}
                  aria-pressed={mode === 'text'}
                  className={`px-3 py-1 ${mode === 'text' ? 'bg-slate-700 text-white' : 'text-gray-400 hover:bg-slate-800'}`}
                >
                  Text
                </button>
                <button
                  type="button"
                  onClick={() => switchMode('file')}
                  disabled={isCreating}
                  aria-pressed={mode === 'file'}
                  className={`px-3 py-1 ${mode === 'file' ? 'bg-slate-700 text-white' : 'text-gray-400 hover:bg-slate-800'}`}
                >
                  File
                </button>
              </div>
            </div>

            {mode === 'text' ? (
              <textarea
                value={secretText}
                onChange={(e) => setSecretText(e.target.value)}
                placeholder="Enter password, API key, or sensitive data..."
                rows={6}
                className="w-full bg-slate-900 border border-dark-border rounded-lg px-4 py-3 text-gray-100 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 resize-none"
                disabled={isCreating}
              />
            ) : (
              <div>
                <input
                  id="secret-file"
                  ref={fileInputRef}
                  type="file"
                  className="sr-only"
                  onChange={handleFileSelected}
                  disabled={isCreating}
                />
                {file ? (
                  <div className="flex items-center justify-between bg-slate-900 border border-dark-border rounded-lg px-4 py-3">
                    <span className="text-gray-100 break-all">
                      {file.name} <span className="text-gray-400 text-sm">({formatBytes(file.size)})</span>
                    </span>
                    <button
                      type="button"
                      onClick={handleRemoveFile}
                      disabled={isCreating}
                      aria-label="Remove file"
                      className="ml-3 text-gray-400 hover:text-gray-200 text-2xl leading-none"
                    >
                      ×
                    </button>
                  </div>
                ) : (
                  <label
                    htmlFor="secret-file"
                    onDragOver={(e) => { e.preventDefault(); setIsDragging(true); }}
                    onDragLeave={() => setIsDragging(false)}
                    onDrop={handleDrop}
                    className={`block cursor-pointer text-center border-2 border-dashed rounded-lg px-4 py-8 text-gray-400 ${isDragging ? 'border-blue-500 bg-slate-800' : 'border-dark-border bg-slate-900'}`}
                  >
                    Drop a file here or click to choose a file (max 5 MB)
                  </label>
                )}
              </div>
            )}
          </div>
```

Change the submit button's `disabled` and add file wording:

```jsx
            disabled={isCreating || !recipientId || (mode === 'file' ? !file : !secretText.trim())}
```

Update the info list: change `🚫 Server never sees your plaintext message` to `🚫 Server never sees your plaintext message or file`.

- [ ] **Step 4: Run the component tests**

Run (from `frontend/`): `npx vitest run tests/components/CreateMessage.test.jsx`
Expected: PASS (new and pre-existing). If `getByLabelText(/choose a file/i)` matches the `<label>` (text contains "choose a file") and resolves to the input via `htmlFor`, it passes; once a file is selected the label is hidden, which the tests account for.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/CreateMessage.jsx frontend/tests/components/CreateMessage.test.jsx
git commit -m "feat(frontend): send files from the create page"
```

---

### Task 10: Retrieve page — download files

**Files:**
- Modify: `frontend/src/components/RetrieveMessage.jsx`
- Test: `frontend/tests/components/RetrieveMessage.test.jsx` (append tests)

**Interfaces:**
- Consumes: `decryptBytes`, `importKey` (Task 7); `unpackEnvelope`, `sanitizeFilename` (Task 8); `saveBlob` (Task 8); `copyToClipboard` (existing).
- Produces: UI only. Decrypted content stays inside the handler closure; React state holds only `{kind, name}`; the "Save again" function is kept in a `useRef`, cleared on unmount.

- [ ] **Step 1: Write the failing tests** — append to the `describe` in `frontend/tests/components/RetrieveMessage.test.jsx`, and add these imports at the top of the file:

```jsx
import * as cryptoLib from '../../src/lib/crypto';
import * as downloadLib from '../../src/lib/download';
import * as clipboardLib from '../../src/lib/clipboard';
import { packEnvelope } from '../../src/lib/fileEnvelope';
```

Tests:

```jsx
  describe('after reveal', () => {
    const reveal = async () => {
      api.checkMessageExists.mockResolvedValue(true);
      api.getMessage.mockResolvedValue({ ciphertext: 'ct', iv: 'iv' });
      vi.spyOn(cryptoLib, 'importKey').mockResolvedValue({});
      renderComponent();
      const button = await screen.findByRole('button', { name: /Destroy Message/i });
      fireEvent.click(button);
    };

    test('downloads a file with a sanitized name and does not touch the clipboard', async () => {
      const bytes = packEnvelope('../../evil.txt', 'text/plain', new TextEncoder().encode('FILE-CONTENT'));
      vi.spyOn(cryptoLib, 'decryptBytes').mockResolvedValue(bytes);
      const save = vi.spyOn(downloadLib, 'saveBlob').mockReturnValue(() => {});
      const copy = vi.spyOn(clipboardLib, 'copyToClipboard');

      await reveal();

      await waitFor(() => expect(save).toHaveBeenCalledTimes(1));
      expect(save.mock.calls[0][1]).toBe('evil.txt');
      expect(copy).not.toHaveBeenCalled();
      expect(await screen.findByText(/evil\.txt/)).toBeInTheDocument();
      expect(screen.queryByText(/FILE-CONTENT/)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: /save again/i })).toBeInTheDocument();
    });

    test('copies text to the clipboard and never renders it', async () => {
      vi.spyOn(cryptoLib, 'decryptBytes').mockResolvedValue(new TextEncoder().encode('TOP-SECRET'));
      const save = vi.spyOn(downloadLib, 'saveBlob');
      const copy = vi.spyOn(clipboardLib, 'copyToClipboard').mockResolvedValue({ success: true });

      await reveal();

      await waitFor(() => expect(copy).toHaveBeenCalledWith('TOP-SECRET'));
      expect(save).not.toHaveBeenCalled();
      expect(screen.queryByText(/TOP-SECRET/)).not.toBeInTheDocument();
      expect(await screen.findByText(/copied to your clipboard/i)).toBeInTheDocument();
    });

    test('a corrupt file shows an error instead of a download', async () => {
      const bad = new TextEncoder().encode('VNSHFILE1\n{"name":"a","type":"b","size":9}\nabc');
      vi.spyOn(cryptoLib, 'decryptBytes').mockResolvedValue(bad);
      const save = vi.spyOn(downloadLib, 'saveBlob');

      await reveal();

      expect(await screen.findByText(/Corrupt file/i)).toBeInTheDocument();
      expect(save).not.toHaveBeenCalled();
    });
  });
```

- [ ] **Step 2: Run to verify they fail**

Run (from `frontend/`): `npx vitest run tests/components/RetrieveMessage.test.jsx`
Expected: new tests FAIL (no "Destroy Message" button wording match / `decryptBytes` not used).

- [ ] **Step 3: Edit `frontend/src/components/RetrieveMessage.jsx`.**

Imports:

```jsx
import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { importKey, decryptBytes } from '../lib/crypto';
import { getMessage, checkMessageExists } from '../lib/api';
import { extractKeyFromURL } from '../utils/urlHelpers';
import { copyToClipboard, clearSensitiveData } from '../lib/clipboard';
import { unpackEnvelope, sanitizeFilename } from '../lib/fileEnvelope';
import { saveBlob } from '../lib/download';
```

State (after `const [isBurned, ...]`):

```jsx
  const [burnedInfo, setBurnedInfo] = useState(null); // { kind: 'text' } | { kind: 'file', name }
  const resaveRef = useRef(null); // closure over the decrypted file; never put in state

  useEffect(() => () => { resaveRef.current = null; }, []);
```

In `handleCopyAndBurn`, replace steps 4–5 and the success part (from `// Step 4` through `setIsBurned(true);`):

```jsx
      // Step 4: Decrypt in memory (NOT in state - stays in closure)
      const bytes = await decryptBytes(ciphertext, iv, encryptionKey);
      const payload = unpackEnvelope(bytes);

      if (payload.kind === 'file') {
        // Step 5a: Save the file straight to disk without rendering it
        const safeName = sanitizeFilename(payload.name);
        resaveRef.current = saveBlob(payload.data, safeName, payload.type);
        setBurnedInfo({ kind: 'file', name: safeName });
      } else {
        // Step 5b: Copy text directly to clipboard without rendering
        decryptedSecret = payload.text;
        const result = await copyToClipboard(decryptedSecret);

        if (!result.success) {
          throw new Error('Failed to copy to clipboard: ' + (result.error || 'Unknown error'));
        }
        setBurnedInfo({ kind: 'text' });
      }

      // Success! Message is burned and delivered
      setIsBurned(true);
```

(Remove the old `decryptedSecret = await decrypt(...)` and `copyToClipboard(decryptedSecret)` lines; keep the surrounding cleanup, `window.history.replaceState`, `catch` and `finally`.)

Burned screen — replace the `<p>` lines and the warning box inside `if (isBurned)`:

```jsx
          <h2 className="text-2xl font-bold text-orange-400 mb-4">Message Burned!</h2>
          {burnedInfo && burnedInfo.kind === 'file' ? (
            <>
              <p className="text-gray-300 mb-2">
                File downloaded as <span className="font-mono break-all">{burnedInfo.name}</span>
              </p>
              <p className="text-sm text-gray-500 mb-6">
                The message has been permanently destroyed
              </p>
              <div className="bg-yellow-900/30 border border-yellow-600 text-yellow-300 px-4 py-3 rounded-lg text-sm mb-4">
                ⚠️ Didn't get the file? Save it now - it can't be fetched again
              </div>
              <button
                onClick={() => resaveRef.current && resaveRef.current()}
                className="w-full bg-slate-700 hover:bg-slate-600 text-white font-semibold py-3 px-6 rounded-lg transition duration-200 mb-3"
              >
                Save again
              </button>
            </>
          ) : (
            <>
              <p className="text-gray-300 mb-2">Secret copied to your clipboard</p>
              <p className="text-sm text-gray-500 mb-6">
                The message has been permanently destroyed
              </p>
              <div className="bg-yellow-900/30 border border-yellow-600 text-yellow-300 px-4 py-3 rounded-lg text-sm mb-4">
                ⚠️ Paste the secret now - it's not stored anywhere
              </div>
            </>
          )}
```

Awaiting screen (the final `return`): change the subtitle and button label and the footer notes:

```jsx
        <h2 className="text-2xl font-bold mb-4">Secret Awaits</h2>
        <p className="text-gray-400 mb-6">
          Click the button below to reveal the secret. Text is copied to your clipboard; a file is downloaded.
        </p>
```
```jsx
            '🔥 Reveal & Destroy Message'
```
```jsx
          <p>🔐 Decryption happens locally in your browser</p>
          <p>👁️ Secrets are NOT displayed on screen</p>
          <p>📋 Text goes to your clipboard; files are saved to your downloads</p>
```
and change the `Copying & Burning...` spinner label to `Decrypting & Burning...`.

Also remove now-unused `clearSensitiveData(encryptionKey)` is still used - keep. `decrypt` import is removed.

- [ ] **Step 4: Run all frontend tests**

Run (from `frontend/`): `npx vitest run`
Expected: PASS for all suites. Fix any pre-existing test that asserted the old label text (`Copy to Clipboard & Destroy Message`, `Secret Message Awaits`) by updating it to the new wording.

- [ ] **Step 5: Production build check**

Run (from `frontend/`): `npm run build`
Expected: build succeeds with no errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/RetrieveMessage.jsx frontend/tests/components/RetrieveMessage.test.jsx
git commit -m "feat(frontend): download files on the retrieve page"
```

---

### Task 11: Docs and end-to-end verification

**Files:**
- Modify: `docs/ARCHITECTURE.md` (append section), `docs/API_REFERENCE.md` (insert before the closing `---` footer), `cli/README.md`

- [ ] **Step 1: Append to `docs/ARCHITECTURE.md`:**

```markdown
## File Messages

Files travel through the normal message path. The sender's client builds an
envelope, encrypts it, and uploads ciphertext + IV like any other message:

```
VNSHFILE1\n
{"name":"<string>","type":"<mime>","size":<int>}\n
<raw file bytes>
```

- The server cannot tell a file from text, and never sees the file name, type
  or content. PostgreSQL metadata, notifications and logs carry none of them.
- Limits: 5 MiB per file (enforced by clients), 8 MiB request body (backend
  `BodyLimit`, nginx `client_max_body_size 8m` on `/api/`). Larger bodies get
  `413`.
- On receipt, plaintext starting with the magic and a valid header is a file;
  anything else is text. File names are sanitized (base name only, no control
  characters or `:*?"<>|`, ≤ 255 bytes) before any write or download.
- Redis holds the ciphertext (up to ~7 MB per file message) until it is read
  or its TTL expires; there is no per-user cap on pending file messages.
- `shared/crypto/envelope.go` and `frontend/src/lib/fileEnvelope.js` must stay
  in sync; `shared/crypto/testdata/vector.json` is read by both test suites.
```

- [ ] **Step 2: Insert into `docs/API_REFERENCE.md`** (above the final `---` / "Generated" footer):

```markdown
## Message Size Limit

`POST /api/messages` accepts a JSON body of at most 8 MiB. A larger body is
rejected with `413 Request Entity Too Large` and `{"error":"Message too large"}`.
A 5 MiB file becomes about 7 MB after encryption and base64 encoding.
```

- [ ] **Step 3: Update `cli/README.md`.** In **Features** add `- 📎 **File transfer** (up to 5 MiB, encrypted end to end)`. Under **Quick Start** add:

```markdown
### 3. Send a File

```bash
vanish send user@example.com -file ./credentials.pdf
```

### 4. Receive a Secret or File

```bash
# Text is printed to stdout; a file is saved to the current directory
vanish get "https://vanish.example.com/m/abc123#KEY"

# Save a file to a specific path (refuses to overwrite unless -force)
vanish get -o ./out.pdf "https://vanish.example.com/m/abc123#KEY"
```

Reading a link destroys the message. An explicit `-o` path is checked before
the message is fetched, so a bad destination does not burn it. Without `-o`
an existing file is never overwritten: `report.pdf` is saved as `report-1.pdf`.
```

and replace the `Usage:` block with the new help text from `printHelp`.

- [ ] **Step 4: Run every suite once**

```bash
(cd shared && go vet ./... && go test ./... -race)
(cd cli && go vet ./... && go test ./... -race)
(cd backend && go vet ./... && go test ./tests/unit/ -run 'BodyLimit|MaxMessageBodyBytes|Middleware' -v)
(cd frontend && npx vitest run && npm run build)
```
Expected: all PASS / build OK.

- [ ] **Step 5: Manual end-to-end (needs Docker; record results in the PR description)**

`./start.sh dev`, register two users, then verify each of: web→web file (binary, ≈4 MB), CLI→web (`vanish send other@x -file`), web→CLI (`vanish get <link>`), a text secret both ways, a 5 MiB + 1 file rejected in the UI and CLI, and that opening a file link a second time reports "Message not found or already burned".

- [ ] **Step 6: Commit**

```bash
git add docs/ARCHITECTURE.md docs/API_REFERENCE.md cli/README.md
git commit -m "docs: document file messages, size limits and CLI file commands"
```

---

## Self-Review (completed)

- **Spec coverage:** wire format (Tasks 1–2, 7–8); limits client/backend/nginx (Tasks 4, 6, 8, 9); filename handling (Tasks 2, 8); JS crypto/base64/URL-safe (Task 7); `fileEnvelope.js`/`download.js` (Task 8); Create/Retrieve UI incl. "Save again" (Tasks 9–10); `shared/crypto`, `ParseMessageURL` (Tasks 1–3); CLI send/get incl. pre-checks, 0600, `-force` (Tasks 4–5); shared vector (Tasks 2, 7, 8); docs (Task 11). Spec's CLI form `vanish send <email> --file` works via interspersed flag parsing (Task 4).
- **Deliberate refinement of the spec:** without `-o`, a name collision picks a free name instead of refusing, because refusing after the burn would lose the file; an explicit `-o` is still refused up front. Single-dash flags (`-file`, `-o`, `-force`) are what Go's `flag` package accepts; `--file` also works.
- **Placeholder scan:** none. Every code step contains the code.
- **Type consistency:** `EncryptBytes/DecryptBytes/PackFile/Unpack/SanitizeFilename/ParseMessageURL/loadFileEnvelope/parseInterspersed/runGet/getOptions` and JS `encryptBytes/decryptBytes/packEnvelope/packFile/unpackEnvelope/validateFileForSend/formatBytes/sanitizeFilename/saveBlob` are used with the same names and signatures throughout.
- **Known caveat for the executor:** the backend's other unit tests need Redis/PostgreSQL; only the new body-limit and middleware tests are expected to pass without them.
