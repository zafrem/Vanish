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
