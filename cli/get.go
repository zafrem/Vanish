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
	// Validate the key before fetching: fetching burns the message.
	if _, err := crypto.DecodeKey(key); err != nil {
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
