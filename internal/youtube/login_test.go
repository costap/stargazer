package youtube

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// tokenServer is Google's token endpoint, reduced to what the login needs to prove:
// it checks the code and the PKCE verifier, and hands back a token.
type tokenServer struct {
	t         *testing.T
	challenge string // from the consent URL, S256
	refresh   string
	calls     int
}

func (s *tokenServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.calls++
	_ = r.ParseForm()
	if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "the-code" {
		s.t.Errorf("unexpected exchange: %v", r.Form)
	}
	sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != s.challenge {
		s.t.Errorf("the code_verifier does not match the S256 challenge from the consent URL")
	}
	w.Header().Set("Content-Type", "application/json")
	body := `{"access_token":"acc","token_type":"Bearer","expires_in":3600`
	if s.refresh != "" {
		body += `,"refresh_token":"` + s.refresh + `"`
	}
	_, _ = w.Write([]byte(body + "}"))
}

func loginConfig(tokenURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID: "id", ClientSecret: "sec", Scopes: []string{Scope},
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.example.invalid/auth", TokenURL: tokenURL},
	}
}

// browser plays the user: it reads the consent URL and calls the loopback redirect.
func browser(t *testing.T, ts *tokenServer, mutate func(q url.Values)) func(string) error {
	return func(consent string) error {
		u, err := url.Parse(consent)
		if err != nil {
			return err
		}
		q := u.Query()
		ts.challenge = q.Get("code_challenge")
		if q.Get("code_challenge_method") != "S256" || q.Get("access_type") != "offline" || q.Get("prompt") != "consent" {
			t.Errorf("consent URL must ask for S256, offline access and a consent prompt: %v", q)
		}
		if q.Get("scope") != Scope {
			t.Errorf("scope: %q", q.Get("scope"))
		}
		redirect, _ := url.Parse(q.Get("redirect_uri"))
		if !strings.HasPrefix(redirect.Host, "127.0.0.1:") {
			t.Errorf("the redirect must be on the loopback address only: %s", redirect)
		}
		back := url.Values{"code": {"the-code"}, "state": {q.Get("state")}}
		if mutate != nil {
			mutate(back)
		}
		go func() {
			resp, err := http.Get(redirect.String() + "?" + back.Encode())
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
}

func TestLoginExchangesTheCodeWithPKCEAndKeepsTheRefreshToken(t *testing.T) {
	ts := &tokenServer{t: t, refresh: "r1"}
	srv := httptest.NewServer(ts)
	defer srv.Close()
	var out bytes.Buffer
	tok, err := Login(context.Background(), LoginOptions{Config: loginConfig(srv.URL), Open: browser(t, ts, nil), Out: &out, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "acc" || tok.RefreshToken != "r1" {
		t.Errorf("token: %+v", tok)
	}
	if ts.calls != 1 {
		t.Errorf("one exchange, got %d", ts.calls)
	}
	if !strings.Contains(out.String(), "https://accounts.example.invalid/auth") {
		t.Errorf("the consent address must be printed in case no browser opens:\n%s", out.String())
	}
}

func TestLoginRefusesAForeignState(t *testing.T) {
	ts := &tokenServer{t: t, refresh: "r1"}
	srv := httptest.NewServer(ts)
	defer srv.Close()
	_, err := Login(context.Background(), LoginOptions{
		Config: loginConfig(srv.URL), Timeout: 5 * time.Second,
		Open: browser(t, ts, func(q url.Values) { q.Set("state", "attacker") }),
	})
	if err == nil || !strings.Contains(err.Error(), "state") {
		t.Errorf("want a state refusal, got %v", err)
	}
	if ts.calls != 0 {
		t.Error("a code that arrives with the wrong state must never be exchanged")
	}
}

func TestLoginReportsADeniedConsent(t *testing.T) {
	ts := &tokenServer{t: t}
	srv := httptest.NewServer(ts)
	defer srv.Close()
	_, err := Login(context.Background(), LoginOptions{
		Config: loginConfig(srv.URL), Timeout: 5 * time.Second,
		Open: browser(t, ts, func(q url.Values) { q.Del("code"); q.Set("error", "access_denied") }),
	})
	if err == nil || !strings.Contains(err.Error(), "access_denied") || ts.calls != 0 {
		t.Errorf("want access_denied and no exchange, got %v (calls %d)", err, ts.calls)
	}
}

func TestLoginRequiresARefreshToken(t *testing.T) {
	ts := &tokenServer{t: t} // no refresh token in the answer
	srv := httptest.NewServer(ts)
	defer srv.Close()
	_, err := Login(context.Background(), LoginOptions{Config: loginConfig(srv.URL), Open: browser(t, ts, nil), Timeout: 5 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "refresh token") {
		t.Errorf("want a refresh-token error, got %v", err)
	}
}

func TestLoginTimesOutWhenTheBrowserNeverReturns(t *testing.T) {
	_, err := Login(context.Background(), LoginOptions{Config: loginConfig("https://x.invalid/token"), Timeout: 80 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "no answer") {
		t.Errorf("want a timeout, got %v", err)
	}
}

func TestWhoamiNamesTheChannelOrExplainsItHasNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/channels" || r.URL.Query().Get("mine") != "true" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if r.Header.Get("X-Has-Channel") == "no" {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"snippet":{"title":"Pedro"}}]}`))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), BaseURL: srv.URL}
	if name, err := c.Whoami(context.Background()); err != nil || name != "Pedro" {
		t.Fatalf("%q %v", name, err)
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/channels?mine=true", nil)
	req.Header.Set("X-Has-Channel", "no")
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.Header.Set("X-Has-Channel", "no")
		return http.DefaultTransport.RoundTrip(r)
	})}
	if _, err := (&Client{HTTP: hc, BaseURL: srv.URL}).Whoami(context.Background()); err == nil || !strings.Contains(err.Error(), "no YouTube channel") {
		t.Errorf("want a no-channel explanation, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
