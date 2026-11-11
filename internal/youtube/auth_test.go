package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeClient(t *testing.T, dir, tokenURL string) string {
	t.Helper()
	p := filepath.Join(dir, "client.json")
	body := `{"installed":{"client_id":"id","client_secret":"sec","auth_uri":"https://example.invalid/auth","token_uri":"` + tokenURL + `"}}`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOAuthConfigRejectsAFileWithoutAClient(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "c.json")
	_ = os.WriteFile(p, []byte(`{"something":"else"}`), 0o600)
	if _, err := OAuthConfig(p); err == nil {
		t.Error("a file with no installed or web client must be refused")
	}
	if _, err := OAuthConfig(""); err == nil {
		t.Error("no path must be refused with a hint")
	}
}

func TestMissingTokenSaysHowToLogIn(t *testing.T) {
	dir := t.TempDir()
	_, err := NewHTTPClient(context.Background(), writeClient(t, dir, "https://example.invalid/token"), filepath.Join(dir, "none.json"))
	if err == nil || !strings.Contains(err.Error(), "playlist login") {
		t.Errorf("want a hint to run playlist login, got %v", err)
	}
}

func TestAnExpiredTokenIsRefreshedAndSaved(t *testing.T) {
	var refreshes int
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshes++
		_ = r.ParseForm()
		if r.Form.Get("refresh_token") != "refresh-1" || r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("unexpected refresh request: %v", r.Form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fresh","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer fresh" {
			t.Errorf("request must carry the refreshed token, got %q", got)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer apiSrv.Close()

	dir := t.TempDir()
	client := writeClient(t, dir, tokenSrv.URL)
	tokenFile := filepath.Join(dir, "sub", "token.json")
	if err := SaveToken(tokenFile, testToken{Access: "stale", Refresh: "refresh-1", Expiry: time.Now().Add(-time.Hour)}.oauth()); err != nil {
		t.Fatal(err)
	}

	hc, err := NewHTTPClient(context.Background(), client, tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := hc.Get(apiSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if refreshes != 1 {
		t.Errorf("want one refresh, got %d", refreshes)
	}

	b, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.Unmarshal(b, &saved)
	if saved.AccessToken != "fresh" || saved.RefreshToken != "refresh-1" {
		t.Errorf("the refreshed token must be saved and keep its refresh token: %+v", saved)
	}
	info, _ := os.Stat(tokenFile)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("the token file must be owner-only, is %v", info.Mode().Perm())
	}
}
