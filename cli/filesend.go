package main

import (
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

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

// isKnownFlag reports whether arg looks like -name, --name or -name=value for
// a flag defined on fs.
func isKnownFlag(fs *flag.FlagSet, arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
	if i := strings.Index(name, "="); i >= 0 {
		name = name[:i]
	}
	return name != "" && fs.Lookup(name) != nil
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
			// Only treat what follows as flags if it starts with a flag this
			// FlagSet defines; otherwise it is message text (e.g. "-hunter2").
			if len(args) == 0 || !isKnownFlag(fs, args[0]) {
				return append(positional, args...), nil
			}
			if err := fs.Parse(args); err != nil {
				return nil, err
			}
			return append(positional, fs.Args()...), nil
		}
	}
}
