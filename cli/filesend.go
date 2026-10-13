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
