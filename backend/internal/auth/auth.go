// Package auth implements NextChapter's password hashing, opaque-token
// mint/verify, and the gin middleware that resolves a session cookie or
// bearer token to a *User. See ADR-0001.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/enable-it/nextchapter/backend/constants"
)

// SessionDuration is the sliding expiry window for session cookies.
// Stays here because it's not consumed cross-package.
const SessionDuration = 30 * 24 * time.Hour

// ErrInvalidCredentials is returned by VerifyPassword on a mismatch.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrTokenNotFound is returned by Resolve when no matching auth_token row exists
// or the row is expired.
var ErrTokenNotFound = errors.New("auth: token not found or expired")

// HashPassword bcrypt-hashes the given password using the default cost.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash password: %w", err)
	}
	return string(h), nil
}

// VerifyPassword returns nil if the password matches the stored hash,
// [ErrInvalidCredentials] otherwise.
func VerifyPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidCredentials
		}
		return fmt.Errorf("auth: compare password: %w", err)
	}
	return nil
}

// MintToken returns a fresh opaque token suitable for storing as kind.
// The returned token already carries its prefix.
func MintToken(kind string) (string, error) {
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

// HashToken returns the lowercase hex SHA-256 of token. We store only the
// hash; the raw token is shown to the user exactly once.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeCompare exposes subtle.ConstantTimeCompare for callers that
// want to avoid timing leaks on hex-token comparisons.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
