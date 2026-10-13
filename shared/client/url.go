package client

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// messageIDPattern matches server message IDs: URL-safe base64, optionally
// padded (the backend uses base64.URLEncoding, so IDs end in "==").
var messageIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+={0,2}$`)

// Errors never include the link: it carries the encryption key and they are
// printed to stderr.
var (
	errInvalidLink = errors.New("invalid link")
	errLinkPath    = errors.New("invalid link: expected .../m/<id>#<key>")
	errLinkKey     = errors.New("invalid link: missing encryption key after '#'")
)

// ParseMessageURL extracts the message ID and encryption key from a Vanish
// link of the form {base}/m/{id}#{key}.
func ParseMessageURL(raw string) (id, key string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", errInvalidLink
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	n := len(segments)
	if n < 2 || segments[n-2] != "m" || !messageIDPattern.MatchString(segments[n-1]) {
		return "", "", errLinkPath
	}
	id = segments[n-1]

	if u.Fragment == "" {
		return "", "", errLinkKey
	}
	return id, u.Fragment, nil
}
