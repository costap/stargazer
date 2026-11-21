package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/costap/stargazer/internal/app"
	"github.com/costap/stargazer/internal/config"
	"github.com/costap/stargazer/internal/youtube"
	"github.com/spf13/cobra"
)

var (
	playlistDryRun    bool
	playlistNoBrowser bool
)

func init() {
	rootCmd.AddCommand(playlistCmd)
	playlistCmd.AddCommand(playlistSyncCmd, playlistLoginCmd)
	playlistLoginCmd.Flags().BoolVar(&playlistNoBrowser, "no-browser", false, "print the login address instead of opening a browser")
	playlistSyncCmd.Flags().BoolVar(&playlistDryRun, "dry-run", false, "print what would change and write nothing (needs no login before the first sync)")
}

var playlistCmd = &cobra.Command{
	Use:   "playlist",
	Short: "Keep a private YouTube playlist in step with the audio queue",
	Long: `The audio queue (YouTube talks from the signals and digests) can be mirrored into a
private playlist that Stargazer creates and owns, so you can press play on your phone.`,
}

var playlistSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Create or update the playlist so it matches the queue",
	Long: `Builds the wanted list from the signals (newest spotted first, then each digest's
"Watch These First" links), skips anything under heard: in the state file, cuts it to
youtube.playlist.cap, and adds, removes and reorders items in the playlist Stargazer owns.
It never deletes the playlist and only removes items it finds in it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		var overridden []string
		for flag, key := range flagKeys {
			if cmd.Flags().Changed(flag) {
				overridden = append(overridden, key)
			}
		}
		cfg, err := config.Load(GetViper(), overridden...)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		if syncSignalsSource != "" {
			cfg.Signals.Source = syncSignalsSource
		}

		newAPI := func() (app.PlaylistAPI, error) {
			hc, err := youtube.NewHTTPClient(cmd.Context(), cfg.YouTube.ClientFile, cfg.YouTube.TokenFile)
			if err != nil {
				return nil, err
			}
			return &youtube.Client{HTTP: hc}, nil
		}
		_, err = app.NewEngine(cfg).SyncPlaylist(cmd.Context(), app.PlaylistOptions{DryRun: playlistDryRun}, newAPI, os.Stdout)
		if err != nil {
			return fmt.Errorf("playlist sync failed: %w", err)
		}
		return nil
	},
}

var playlistLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to YouTube once, so the playlist can be kept in sync",
	Long: `Opens your browser to Google's consent page, waits for it to come back on a
loopback address (127.0.0.1, a random port, PKCE) and stores the login at
youtube.token_file with owner-only permissions. It needs youtube.client_file: the
Desktop-app OAuth client JSON downloaded from Google Cloud.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		cfg, err := config.Load(GetViper())
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		oc, err := youtube.OAuthConfig(cfg.YouTube.ClientFile)
		if err != nil {
			return err
		}
		opts := youtube.LoginOptions{Config: oc, Out: os.Stdout, Timeout: 5 * time.Minute}
		if !playlistNoBrowser {
			opts.Open = youtube.OpenBrowser
		}
		tok, err := youtube.Login(cmd.Context(), opts)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		if err := youtube.SaveToken(cfg.YouTube.TokenFile, tok); err != nil {
			return fmt.Errorf("the login worked but the token could not be saved: %w", err)
		}
		fmt.Printf("Saved the login to %s\n", youtube.ExpandHome(cfg.YouTube.TokenFile))

		hc, err := youtube.NewHTTPClient(cmd.Context(), cfg.YouTube.ClientFile, cfg.YouTube.TokenFile)
		if err != nil {
			return err
		}
		name, err := (&youtube.Client{HTTP: hc}).Whoami(cmd.Context())
		if err != nil {
			return fmt.Errorf("logged in, but YouTube would not answer for this account (is the YouTube Data API v3 enabled in the project?): %w", err)
		}
		fmt.Printf("Logged in to YouTube as %q. Run `stargazer playlist sync --dry-run` next.\n", name)
		return nil
	},
}
