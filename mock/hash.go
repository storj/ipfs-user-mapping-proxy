package mock

import (
	"crypto/sha256"
	"encoding/base64"
)

// Hash returns a base64-encoded sha256 hash of name.
func Hash(name string) string {
	hasher := sha256.New()
	_, err := hasher.Write([]byte(name))
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}
