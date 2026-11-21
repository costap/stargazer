package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type ADRGroupConfig struct {
	Namespace string `mapstructure:"namespace"` // e.g. "PLT", "SB"
	Dir       string `mapstructure:"dir"`       // e.g. "docs/adrs/platform"
}

type Config struct {
	ADR struct {
		WIPLimit int              `mapstructure:"wip_limit"`
		Groups   []ADRGroupConfig `mapstructure:"groups"`
	} `mapstructure:"adr"`

	Fronts struct {
		// Limit caps the non-ADR fronts in flight (state file). Zero means no limit.
		Limit int `mapstructure:"limit"`
	} `mapstructure:"fronts"`

	Candidates struct {
		// Dir holds NNN-*.md ADR candidate files. Unset means candidates are not read.
		Dir string `mapstructure:"dir"`
	} `mapstructure:"candidates"`

	YouTube struct {
		// ClientFile is the OAuth client JSON downloaded from Google Cloud.
		ClientFile string `mapstructure:"client_file"`
		// TokenFile holds the login token. Keep it outside any repository.
		TokenFile string `mapstructure:"token_file"`
		// PlaylistFile records the playlist Stargazer created (machine-owned).
		PlaylistFile string `mapstructure:"playlist_file"`
		Playlist     struct {
			Title string `mapstructure:"title"`
			Cap   int    `mapstructure:"cap"`
		} `mapstructure:"playlist"`
	} `mapstructure:"youtube"`

	Roadmap struct {
		File string `mapstructure:"file"`

		// Explicit is true when the roadmap path was set on purpose (config file,
		// flag or environment) rather than left at the default. A missing roadmap is
		// normal in the second case and an error in the first.
		Explicit bool `mapstructure:"-"`
	} `mapstructure:"roadmap"`

	State struct {
		// File records which Accepted ADRs have been dispatched. A missing file at
		// the default path means nothing is dispatched yet.
		File     string `mapstructure:"file"`
		Explicit bool   `mapstructure:"-"`
	} `mapstructure:"state"`

	Signals struct {
		Source      string   `mapstructure:"source"`
		FilterRings []string `mapstructure:"filter_rings"`
		FilterTags  []string `mapstructure:"filter_tags"`
	} `mapstructure:"signals"`

	Dispatch struct {
		Output string `mapstructure:"output"`
		Silent bool   `mapstructure:"silent"`
	} `mapstructure:"dispatch"`
}

// Load maps Viper onto Config and resolves relative paths.
//
// A path the config file sets is resolved against the config file's directory, so
// the same file behaves the same from any working directory. A path that came from
// somewhere with higher precedence stays relative to the working directory, which is
// what a shell user typing it expects. overridden lists the Viper keys that a command
// line flag set; environment overrides are detected here.
func Load(v *viper.Viper, overridden ...string) (*Config, error) {
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	flagSet := map[string]bool{}
	for _, k := range overridden {
		flagSet[k] = true
	}
	cfg.Roadmap.Explicit = flagSet["roadmap.file"] || envSet("roadmap.file") || v.InConfig("roadmap.file")

	cfg.State.Explicit = envSet("state.file") || v.InConfig("state.file")

	configDir := ""
	if used := v.ConfigFileUsed(); used != "" {
		if abs, err := filepath.Abs(filepath.Dir(used)); err == nil {
			configDir = abs
		}
	}
	fromConfig := func(key string) bool {
		return configDir != "" && v.InConfig(key) && !flagSet[key] && !envSet(key)
	}

	if fromConfig("roadmap.file") {
		cfg.Roadmap.File = resolve(configDir, cfg.Roadmap.File)
	}
	if fromConfig("state.file") {
		cfg.State.File = resolve(configDir, cfg.State.File)
	}
	if fromConfig("candidates.dir") {
		cfg.Candidates.Dir = resolve(configDir, cfg.Candidates.Dir)
	}
	if fromConfig("youtube.client_file") && !strings.HasPrefix(cfg.YouTube.ClientFile, "~") {
		cfg.YouTube.ClientFile = resolve(configDir, cfg.YouTube.ClientFile)
	}
	if fromConfig("youtube.token_file") && !strings.HasPrefix(cfg.YouTube.TokenFile, "~") {
		cfg.YouTube.TokenFile = resolve(configDir, cfg.YouTube.TokenFile)
	}
	// The playlist record is what stops a second playlist being created, so its default
	// location must not depend on the directory the command is run from: with a config
	// file, it sits beside that file.
	if configDir != "" && !flagSet["youtube.playlist_file"] && !envSet("youtube.playlist_file") {
		cfg.YouTube.PlaylistFile = resolve(configDir, cfg.YouTube.PlaylistFile)
	}
	if fromConfig("dispatch.output") {
		cfg.Dispatch.Output = resolve(configDir, cfg.Dispatch.Output)
	}
	if fromConfig("signals.source") && !strings.Contains(cfg.Signals.Source, "://") {
		cfg.Signals.Source = resolve(configDir, cfg.Signals.Source)
	}
	if configDir != "" && v.InConfig("adr.groups") {
		for i := range cfg.ADR.Groups {
			cfg.ADR.Groups[i].Dir = resolve(configDir, cfg.ADR.Groups[i].Dir)
		}
	}

	// Fallback default: if no groups configured, create standard default
	if len(cfg.ADR.Groups) == 0 {
		cfg.ADR.Groups = []ADRGroupConfig{
			{Namespace: "ADR", Dir: "docs/adrs"},
		}
	}

	return &cfg, nil
}

// resolve joins a relative path onto base and leaves empty and absolute paths alone.
func resolve(base, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(base, p)
}

// envSet reports whether the STARGAZER_ environment variable for a Viper key is set.
func envSet(key string) bool {
	name := "STARGAZER_" + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
	_, ok := os.LookupEnv(name)
	return ok
}
