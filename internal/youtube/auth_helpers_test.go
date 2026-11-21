package youtube

import (
	"time"

	"golang.org/x/oauth2"
)

type testToken struct {
	Access, Refresh string
	Expiry          time.Time
}

func (t testToken) oauth() *oauth2.Token {
	return &oauth2.Token{AccessToken: t.Access, RefreshToken: t.Refresh, TokenType: "Bearer", Expiry: t.Expiry}
}
