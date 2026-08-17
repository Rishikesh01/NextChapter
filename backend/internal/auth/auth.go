// Package auth implements NextChapter's password hashing, opaque-token
// mint/verify, and the gin middleware that resolves a session cookie or
// bearer token to a *User.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/enable-it/nextchapter/backend/constants"
)

// sessionDuration is the sliding expiry window for session cookies.
const sessionDuration = 30 * 24 * time.Hour

// ErrTokenNotFound is returned by Resolve when no matching auth_token row exists
// or the row is expired.
var ErrTokenNotFound = errors.New("auth: token not found or expired")

// mintToken returns a fresh opaque token suitable for storing as kind.
// The returned token already carries its prefix.
func mintToken(kind string) (string, error) {
	var prefix string
	switch kind {
	case constants.TokenKindSession:
		prefix = constants.TokenPrefixSession
	case constants.TokenKindAPI:
		prefix = constants.TokenPrefixAPI
	default:
		return "", fmt.Errorf("auth: unknown token kind %q", kind)
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("auth: read random bytes: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// hashToken returns the lowercase hex SHA-256 of token. We store only
// the hash; the raw token is shown to the user exactly once. Exposed
// to integration tests via [AssertSessionGone] / [AssertSessionExists]
// rather than directly so the storage-hash format stays inside the
// package.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
