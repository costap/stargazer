package youtube

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
)

// LoginOptions controls one interactive login.
type LoginOptions struct {
	// Config is the OAuth client (from OAuthConfig). Its RedirectURL is set here.
	Config *oauth2.Config
	// Open shows the consent URL to the user. When nil, the URL is only printed.
	Open func(url string) error
	// Timeout bounds the wait for the browser to come back. Zero means five minutes.
	Timeout time.Duration
	// Out receives the instructions.
	Out io.Writer
}

// Login runs the installed-app OAuth flow: a loopback listener on 127.0.0.1, PKCE (S256)
// and a random state. It returns a token that carries a refresh token, or an error.
// Nothing is written to disk here; the caller saves the token.
func Login(ctx context.Context, o LoginOptions) (*oauth2.Token, error) {
	if o.Config == nil {
		return nil, errors.New("no OAuth client configured")
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	out := o.Out
	if out == nil {
		out = io.Discard
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("unable to listen for the browser's return: %w", err)
	}
	defer ln.Close()

	cfg := *o.Config
	cfg.RedirectURL = fmt.Sprintf("http://%s/callback", ln.Addr().String())
	state := oauth2.GenerateVerifier() // random, URL-safe
	verifier := oauth2.GenerateVerifier()

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var res result
		switch {
		case q.Get("state") != state:
			res.err = errors.New("the browser came back with a different state: login refused")
			http.Error(w, "state mismatch", http.StatusBadRequest)
		case q.Get("error") != "":
			res.err = fmt.Errorf("Google reported %q: %s", q.Get("error"), q.Get("error_description"))
			fmt.Fprintf(w, "<p>Login failed: %s</p>", html.EscapeString(q.Get("error")))
		case q.Get("code") == "":
			res.err = errors.New("the browser came back without a code")
			http.Error(w, "no code", http.StatusBadRequest)
		default:
			res.code = q.Get("code")
			fmt.Fprint(w, "<p>Stargazer is logged in to YouTube. You can close this tab.</p>")
		}
		select {
		case done <- res:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()

	authURL := cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"), // without it Google may omit the refresh token
		oauth2.S256ChallengeOption(verifier),
	)
	fmt.Fprintf(out, "Open this address to log in to YouTube:\n\n  %s\n\n", authURL)
	if o.Open != nil {
		if err := o.Open(authURL); err != nil {
			fmt.Fprintf(out, "(could not open a browser: %v; open the address by hand)\n", err)
		}
	}
	fmt.Fprintf(out, "Waiting up to %s for the browser to come back...\n", timeout.Round(time.Second))

	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var res result
	select {
	case res = <-done:
	case <-wctx.Done():
		if errors.Is(wctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("no answer from the browser within %s", timeout.Round(time.Second))
		}
		return nil, wctx.Err()
	}
	if res.err != nil {
		return nil, res.err
	}

	tok, err := cfg.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("Google refused the login code: %w", err)
	}
	if tok.RefreshToken == "" {
		return nil, errors.New("Google returned no refresh token, so the login could not be kept: remove Stargazer under https://myaccount.google.com/permissions and log in again")
	}
	return tok, nil
}

// OpenBrowser opens a URL in the user's default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// Whoami returns the title of the signed-in account's YouTube channel (one quota unit).
func (c *Client) Whoami(ctx context.Context) (string, error) {
	var out struct {
		Items []struct {
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}
	q := map[string][]string{"part": {"snippet"}, "mine": {"true"}}
	if err := c.do(ctx, "who am i", http.MethodGet, "/channels", q, nil, &out); err != nil {
		return "", err
	}
	if len(out.Items) == 0 {
		return "", errors.New("this Google account has no YouTube channel: create one at youtube.com, then log in again")
	}
	return out.Items[0].Snippet.Title, nil
}
