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
