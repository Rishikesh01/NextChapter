package auth_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/enable-it/nextchapter/backend/constants"
	"github.com/enable-it/nextchapter/backend/internal/auth"
)

func TestMintTokenPrefixes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind   string
		prefix string
	}{
		{constants.TokenKindSession, constants.TokenPrefixSession},
		{constants.TokenKindAPI, constants.TokenPrefixAPI},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			t.Parallel()
			r := require.New(t)
			tok, err := auth.MintToken(c.kind)
			r.NoError(err)
			r.True(strings.HasPrefix(tok, c.prefix), "token %q missing prefix %q", tok, c.prefix)
			// The body is 32 bytes base64url-encoded => 43 chars.
			body := strings.TrimPrefix(tok, c.prefix)
			r.Equal(43, len(body))
		})
	}
}

func TestMintTokenUnknownKind(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	_, err := auth.MintToken("bogus")
	r.Error(err)
}

func TestHashTokenStable(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	a := assert.New(t)
	h1 := auth.HashToken("ncs_abc")
	h2 := auth.HashToken("ncs_abc")
	r.Equal(h1, h2)
	a.NotEqual(auth.HashToken("ncs_abc"), auth.HashToken("ncs_abd"))
	r.Equal(64, len(h1))
}

func TestHashPasswordRoundTrip(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	const pw = "correct horse battery staple"
	hash, err := auth.HashPassword(pw)
	r.NoError(err)
	r.NoError(auth.VerifyPassword(hash, pw))
	r.Error(auth.VerifyPassword(hash, "wrong password"))
}
