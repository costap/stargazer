package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// newViper returns a Viper wired like the CLI: a config file, the STARGAZER_ env
// prefix and the defaults root.go sets.
func newViper(t *testing.T, yaml string) (*viper.Viper, string) {
	t.Helper()
	dir := t.TempDir()
	v := viper.New()
	v.SetDefault("roadmap.file", "roadmap.yaml")
	v.SetDefault("dispatch.output", "ARCH_DISPATCH.md")
	v.SetEnvPrefix("STARGAZER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	if yaml != "" {
		path := filepath.Join(dir, ".stargazer.yaml")
		if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
	}
	return v, dir
}

const relativeConfig = `
adr:
  groups:
    - namespace: "2-product"
      dir: "adrs/product"
    - namespace: "3-platform"
      dir: "/abs/platform"
roadmap:
  file: "inventory/roadmap.yaml"
dispatch:
  output: "out/ARCH_DISPATCH.md"
signals:
  source: "data/signals.json"
`

func TestLoadResolvesConfigPathsAgainstTheConfigFileDirectory(t *testing.T) {
	v, dir := newViper(t, relativeConfig)
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"group 0": filepath.Join(dir, "adrs/product"),
		"group 1": "/abs/platform", // absolute stays as written
		"roadmap": filepath.Join(dir, "inventory/roadmap.yaml"),
		"output":  filepath.Join(dir, "out/ARCH_DISPATCH.md"),
		"signals": filepath.Join(dir, "data/signals.json"),
	}
	got := map[string]string{
		"group 0": cfg.ADR.Groups[0].Dir,
		"group 1": cfg.ADR.Groups[1].Dir,
		"roadmap": cfg.Roadmap.File,
		"output":  cfg.Dispatch.Output,
		"signals": cfg.Signals.Source,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q, want %q", k, got[k], w)
		}
	}
}

func TestLoadResultIsIndependentOfTheWorkingDirectory(t *testing.T) {
	v, _ := newViper(t, relativeConfig)
	first, _ := Load(v)
	t.Chdir(t.TempDir())
	second, _ := Load(v)
	if !filepath.IsAbs(first.Roadmap.File) || !filepath.IsAbs(first.ADR.Groups[0].Dir) {
		t.Errorf("config-file paths should resolve to absolute: %q %q", first.Roadmap.File, first.ADR.Groups[0].Dir)
	}
	if first.Roadmap.File != second.Roadmap.File || first.ADR.Groups[0].Dir != second.ADR.Groups[0].Dir {
		t.Errorf("paths changed with the working directory: %q vs %q", first.Roadmap.File, second.Roadmap.File)
	}
}

func TestLoadFlagOverrideStaysRelativeToTheWorkingDirectory(t *testing.T) {
	v, _ := newViper(t, relativeConfig)
	v.Set("roadmap.file", "from-flag.yaml") // what a bound flag does to Viper
	cfg, _ := Load(v, "roadmap.file")
	if cfg.Roadmap.File != "from-flag.yaml" {
		t.Errorf("flag path was rewritten: %q", cfg.Roadmap.File)
	}
	if !cfg.Roadmap.Explicit {
		t.Error("a flag-set roadmap must count as explicit")
	}
}

func TestLoadEnvOverrideStaysRelativeToTheWorkingDirectory(t *testing.T) {
	t.Setenv("STARGAZER_ROADMAP_FILE", "from-env.yaml")
	v, _ := newViper(t, relativeConfig)
	cfg, _ := Load(v)
	if cfg.Roadmap.File != "from-env.yaml" {
		t.Errorf("env path was rewritten: %q", cfg.Roadmap.File)
	}
}

func TestLoadLeavesSignalsURLsAlone(t *testing.T) {
	v, _ := newViper(t, "signals:\n  source: \"https://example.test/signals.json\"\n")
	cfg, _ := Load(v)
	if cfg.Signals.Source != "https://example.test/signals.json" {
		t.Errorf("URL was rewritten: %q", cfg.Signals.Source)
	}
}

func TestLoadWithoutAConfigFileKeepsDefaultsRelativeAndRoadmapImplicit(t *testing.T) {
	v, _ := newViper(t, "")
	cfg, _ := Load(v)
	if cfg.Roadmap.File != "roadmap.yaml" || cfg.Dispatch.Output != "ARCH_DISPATCH.md" {
		t.Errorf("defaults were rewritten: %q %q", cfg.Roadmap.File, cfg.Dispatch.Output)
	}
	if cfg.ADR.Groups[0].Dir != "docs/adrs" {
		t.Errorf("fallback group dir = %q", cfg.ADR.Groups[0].Dir)
	}
	if cfg.Roadmap.Explicit {
		t.Error("a defaulted roadmap path must not count as explicit")
	}
}

func TestLoadConfigFileThatOmitsAKeyDoesNotResolveItsDefault(t *testing.T) {
	// A config under $HOME/.config must not drag the default roadmap.yaml in there.
	v, _ := newViper(t, "adr:\n  wip_limit: 3\n")
	cfg, _ := Load(v)
	if cfg.Roadmap.File != "roadmap.yaml" {
		t.Errorf("default was resolved against the config directory: %q", cfg.Roadmap.File)
	}
	if cfg.Roadmap.Explicit {
		t.Error("an omitted roadmap path must not count as explicit")
	}
}

func TestLoadRoadmapSetInConfigIsExplicit(t *testing.T) {
	v, _ := newViper(t, "roadmap:\n  file: \"r.yaml\"\n")
	cfg, _ := Load(v)
	if !cfg.Roadmap.Explicit {
		t.Error("a roadmap set in the config file must count as explicit")
	}
}

func TestStateFileResolvesAgainstTheConfigDirectoryAndIsExplicit(t *testing.T) {
	v, dir := newViper(t, "state:\n  file: \"records/state.yaml\"\n")
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "records/state.yaml"); cfg.State.File != want {
		t.Errorf("state file = %q, want %q", cfg.State.File, want)
	}
	if !cfg.State.Explicit {
		t.Error("a state file named in the config is explicit: if it is missing that is an error")
	}
}

func TestStateFileDefaultIsNotExplicitAndStaysRelative(t *testing.T) {
	v, _ := newViper(t, "adr:\n  wip_limit: 3\n")
	v.SetDefault("state.file", ".stargazer-state.yaml")
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.State.File != ".stargazer-state.yaml" || cfg.State.Explicit {
		t.Errorf("default must stay as is and not be explicit: %+v", cfg.State)
	}
}

func TestStateFileFromTheEnvironmentStaysRelativeToTheWorkingDirectory(t *testing.T) {
	t.Setenv("STARGAZER_STATE_FILE", "mine.yaml")
	v, _ := newViper(t, "state:\n  file: \"records/state.yaml\"\n")
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.State.File != "mine.yaml" || !cfg.State.Explicit {
		t.Errorf("env override: %+v", cfg.State)
	}
}

func TestYouTubeFilesKeepAHomePathAndResolveRelativeOnesAgainstTheConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".stargazer.yaml")
	body := "youtube:\n  client_file: ~/.config/stargazer/client_secret.json\n  token_file: ~/.config/stargazer/youtube-token.json\n  playlist_file: yt.json\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	v.SetConfigFile(cfgPath)
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.YouTube.ClientFile != "~/.config/stargazer/client_secret.json" || cfg.YouTube.TokenFile != "~/.config/stargazer/youtube-token.json" {
		t.Errorf("a ~ path must be left for the loader to expand, not joined onto the config directory: %+v", cfg.YouTube)
	}
	if cfg.YouTube.PlaylistFile != filepath.Join(dir, "yt.json") {
		t.Errorf("a relative path resolves against the config file: %s", cfg.YouTube.PlaylistFile)
	}
}

func TestPlaylistRecordDefaultsBesideTheConfigFileNotTheWorkingDirectory(t *testing.T) {
	v, dir := newViper(t, "adr:\n  groups:\n    - namespace: x\n      dir: adrs\n")
	v.SetDefault("youtube.playlist_file", ".stargazer-youtube.json")
	cfg, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.YouTube.PlaylistFile != filepath.Join(dir, ".stargazer-youtube.json") {
		t.Errorf("running from another directory must find the same record, got %s", cfg.YouTube.PlaylistFile)
	}
}
