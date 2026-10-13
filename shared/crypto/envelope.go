package crypto

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strconv"
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
	// ErrHeaderTooLarge means the file name or type is too long to send.
	ErrHeaderTooLarge = errors.New("file name or type is too long")
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
	// Receivers treat an over-long header as text, so refuse to build one.
	if len(hdr) > maxHeaderBytes {
		return nil, ErrHeaderTooLarge
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
//
// The rules must stay identical to unpackEnvelope in
// frontend/src/lib/fileEnvelope.js; testdata/vector.json pins them:
//   - the header (bytes between the magic and the first '\n', at most 4096)
//     must be valid UTF-8 and a JSON object (a BOM makes it invalid JSON);
//   - keys match exactly and case-sensitively; a duplicate key's last value wins;
//   - name and type must be JSON strings;
//   - size must be a JSON number with an integral value (1.0 and 1e0 count as
//     1) in [0, MaxFileBytes].
func Unpack(plain []byte) (*Payload, error) {
	text := &Payload{Kind: KindText, Text: string(plain)}

	if !bytes.HasPrefix(plain, []byte(EnvelopeMagic)) {
		return text, nil
	}
	rest := plain[len(EnvelopeMagic):]
	window := rest
	if len(window) > maxHeaderBytes+1 {
		window = window[:maxHeaderBytes+1]
	}
	nl := bytes.IndexByte(window, '\n')
	if nl < 0 {
		return text, nil
	}
	header := rest[:nl]
	if !utf8.Valid(header) {
		return text, nil
	}

	// Map decoding is exact-key and last-wins, like JSON.parse.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(header, &fields); err != nil {
		return text, nil
	}
	name, okName := jsonString(fields["name"])
	mime, okType := jsonString(fields["type"])
	size, okSize := jsonSize(fields["size"])
	if !okName || !okType || !okSize {
		return text, nil
	}

	data := rest[nl+1:]
	if len(data) != size {
		return nil, ErrSizeMismatch
	}
	return &Payload{Kind: KindFile, Name: name, Type: mime, Data: data}, nil
}

// jsonString accepts only a JSON string literal (not null).
func jsonString(raw json.RawMessage) (string, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '"' {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// jsonSize accepts only a JSON number whose value is an integer in
// [0, MaxFileBytes]. It reads the number as float64, as JavaScript does.
func jsonSize(raw json.RawMessage) (int, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || !(raw[0] == '-' || (raw[0] >= '0' && raw[0] <= '9')) {
		return 0, false
	}
	f, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || f != math.Trunc(f) || f < 0 || f > MaxFileBytes {
		return 0, false
	}
	return int(f), true
}

// dropFilenameRune reports whether r is removed from file names: C0 and C1
// controls, DEL, U+FFFD (also what invalid UTF-8 decodes to), bidi marks and
// embeddings/overrides/isolates (which can disguise an extension, e.g.
// "a<RLO>fdp.exe"), and characters Windows forbids.
func dropFilenameRune(r rune) bool {
	switch {
	case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
		return true
	case r == utf8.RuneError:
		return true
	case r == 0x200e, r == 0x200f, r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return strings.ContainsRune(`:*?"<>|`, r)
}

// SanitizeFilename reduces a sender-supplied name to a safe base name.
// Identical rules exist in frontend/src/lib/fileEnvelope.js.
func SanitizeFilename(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}

	var b strings.Builder
	for _, r := range name {
		if dropFilenameRune(r) {
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
