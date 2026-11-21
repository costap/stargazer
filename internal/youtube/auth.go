package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

// Scope lets Stargazer manage the playlists of the signed-in account.
const Scope = "https://www.googleapis.com/auth/youtube"

// ExpandHome turns a leading "~/" into the user's home directory.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// OAuthConfig reads the client JSON that Google Cloud lets you download. Both the
// "installed" (Desktop app) and "web" layouts are accepted.
func OAuthConfig(clientFile string) (*oauth2.Config, error) {
	if clientFile == "" {
		return nil, errors.New("no YouTube client file: set youtube.client_file to the OAuth client JSON downloaded from Google Cloud")
	}
	b, err := os.ReadFile(ExpandHome(clientFile))
	if err != nil {
		return nil, fmt.Errorf("unable to read the YouTube client file: %w", err)
	}
	var f struct {
		Installed *clientJSON `json:"installed"`
		Web       *clientJSON `json:"web"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", clientFile, err)
	}
	c := f.Installed
	if c == nil {
		c = f.Web
	}
	if c == nil || c.ClientID == "" || c.TokenURI == "" {
		return nil, fmt.Errorf("%s has no installed or web OAuth client (download the Desktop app client JSON)", clientFile)
	}
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: c.AuthURI, TokenURL: c.TokenURI},
		Scopes:       []string{Scope},
	}, nil
}

type clientJSON struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	AuthURI      string `json:"auth_uri"`
	TokenURI     string `json:"token_uri"`
}

// LoadToken reads the saved login. A missing file says how to log in.
func LoadToken(tokenFile string) (*oauth2.Token, error) {
	path := ExpandHome(tokenFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("no YouTube login at %s: run `stargazer playlist login` first", path)
		}
		return nil, fmt.Errorf("unable to read the YouTube token: %w", err)
	}
	var t oauth2.Token
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s is not a valid token file: %w", path, err)
	}
	if t.RefreshToken == "" && !t.Valid() {
		return nil, fmt.Errorf("the token at %s has expired and carries no refresh token: run `stargazer playlist login` again", path)
	}
	return &t, nil
}

// SaveToken writes the token with owner-only permissions, creating its directory.
func SaveToken(tokenFile string, t *oauth2.Token) error {
	path := ExpandHome(tokenFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// savingSource persists a refreshed token so the next run starts from it.
type savingSource struct {
	mu   sync.Mutex
	src  oauth2.TokenSource
	file string
	last string
}

func (s *savingSource) Token() (*oauth2.Token, error) {
	t, err := s.src.Token()
	if err != nil {
		return nil, fmt.Errorf("YouTube login failed (revoked, or expired in Testing status): run `stargazer playlist login` again: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.AccessToken != s.last {
		s.last = t.AccessToken
		if err := SaveToken(s.file, t); err != nil {
			return nil, fmt.Errorf("unable to save the refreshed token: %w", err)
		}
	}
	return t, nil
}

// NewHTTPClient returns an http.Client that signs requests with the saved login and
// refreshes and re-saves it when it expires.
func NewHTTPClient(ctx context.Context, clientFile, tokenFile string) (*http.Client, error) {
	cfg, err := OAuthConfig(clientFile)
	if err != nil {
		return nil, err
	}
	tok, err := LoadToken(tokenFile)
	if err != nil {
		return nil, err
	}
	src := &savingSource{src: cfg.TokenSource(ctx, tok), file: tokenFile, last: tok.AccessToken}
	return oauth2.NewClient(ctx, oauth2.ReuseTokenSource(tok, src)), nil
}
