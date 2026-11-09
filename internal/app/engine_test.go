package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/costap/stargazer/internal/config"
)

// fixture builds a config over a temp tree with one readable ADR group.
func fixture(t *testing.T) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	adrs := filepath.Join(dir, "adrs")
	if err := os.Mkdir(adrs, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "# 1. Thing\n\nDate: 2026-01-01\n\n## Status\n\nProposed\n\n## Context\n\nx\n"
	if err := os.WriteFile(filepath.Join(adrs, "0001-thing.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.ADR.WIPLimit = 3
	cfg.ADR.Groups = []config.ADRGroupConfig{{Namespace: "2-product", Dir: adrs}}
	cfg.Dispatch.Output = filepath.Join(dir, "ARCH_DISPATCH.md")
	cfg.Dispatch.Silent = true
	return cfg, dir
}

func sync(cfg *config.Config, keepGoing bool) error {
	return NewEngine(cfg).SyncWithOptions(context.Background(), SyncOptions{KeepGoing: keepGoing})
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSyncWritesACleanBoardWhenEverythingIsReadable(t *testing.T) {
	cfg, _ := fixture(t)
	if err := sync(cfg, false); err != nil {
		t.Fatal(err)
	}
	got := read(t, cfg.Dispatch.Output)
	if strings.Contains(got, "DEGRADED") || strings.Contains(got, "UNKNOWN") {
		t.Errorf("a clean sync must not claim to be degraded:\n%s", got)
	}
	if !strings.Contains(got, "2-product/0001") {
		t.Errorf("the ADR is missing from the board:\n%s", got)
	}
}

func TestSyncFailsAndWritesNothingWhenAnADRGroupIsUnreadable(t *testing.T) {
	cfg, dir := fixture(t)
	cfg.ADR.Groups = append(cfg.ADR.Groups, config.ADRGroupConfig{Namespace: "3-platform", Dir: filepath.Join(dir, "missing")})

	err := sync(cfg, false)
	if !errors.Is(err, ErrDegraded) {
		t.Fatalf("want ErrDegraded, got %v", err)
	}
	if _, statErr := os.Stat(cfg.Dispatch.Output); !os.IsNotExist(statErr) {
		t.Error("no board may be written when an ADR group could not be read")
	}
}

func TestKeepGoingNeverOverwritesTheBoardWhenADRsWereNotRead(t *testing.T) {
	cfg, dir := fixture(t)
	cfg.ADR.Groups = []config.ADRGroupConfig{{Namespace: "3-platform", Dir: filepath.Join(dir, "missing")}}
	const notes = "# board\n### [3-platform/0001] X\n- **Next Action:** my own note\n"
	if err := os.WriteFile(cfg.Dispatch.Output, []byte(notes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sync(cfg, true); err != nil {
		t.Fatalf("--keep-going must not fail: %v", err)
	}
	if read(t, cfg.Dispatch.Output) != notes {
		t.Error("a degraded run erased the Next Actions in the existing board")
	}
}

func TestZeroADRsAcrossAllGroupsIsAWarningNotACalmBoard(t *testing.T) {
	cfg, dir := fixture(t)
	empty := filepath.Join(dir, "empty")
	if err := os.Mkdir(empty, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg.ADR.Groups = []config.ADRGroupConfig{{Namespace: "2-product", Dir: empty}}
	if err := sync(cfg, false); !errors.Is(err, ErrDegraded) {
		t.Fatalf("an empty pipeline across every group must warn, got %v", err)
	}
}

func TestExplicitRoadmapThatIsMissingDegradesTheBoardButKeepsADRs(t *testing.T) {
	cfg, dir := fixture(t)
	cfg.Roadmap.File = filepath.Join(dir, "roadmap.yaml")
	cfg.Roadmap.Explicit = true

	if err := sync(cfg, false); !errors.Is(err, ErrDegraded) {
		t.Fatalf("want ErrDegraded, got %v", err)
	}
	if err := sync(cfg, true); err != nil {
		t.Fatal(err)
	}
	got := read(t, cfg.Dispatch.Output)
	if !strings.Contains(got, "DEGRADED BOARD") || !strings.Contains(got, "roadmap") {
		t.Errorf("the banner should name the roadmap:\n%s", got)
	}
	if strings.Contains(got, "UNKNOWN") || !strings.Contains(got, "2-product/0001") {
		t.Errorf("ADRs were read, so the pipeline must be shown, not unknown:\n%s", got)
	}
}

func TestMissingDefaultRoadmapIsOnlyANotice(t *testing.T) {
	cfg, dir := fixture(t)
	cfg.Roadmap.File = filepath.Join(dir, "roadmap.yaml")
	cfg.Roadmap.Explicit = false
	if err := sync(cfg, false); err != nil {
		t.Fatalf("an absent default roadmap is normal: %v", err)
	}
}

func TestMalformedDefaultRoadmapIsStillAWarning(t *testing.T) {
	cfg, dir := fixture(t)
	cfg.Roadmap.File = filepath.Join(dir, "roadmap.yaml")
	if err := os.WriteFile(cfg.Roadmap.File, []byte("phases: [oops\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sync(cfg, false); !errors.Is(err, ErrDegraded) {
		t.Fatalf("a roadmap that exists but cannot be parsed must warn, got %v", err)
	}
}

// The stated non-destructive invariant: a Next Action written by hand in the board
// survives the next sync. Without this a sync would wipe the working memory.
func TestHandWrittenNextActionSurvivesASync(t *testing.T) {
	cfg, _ := fixture(t)
	if err := sync(cfg, false); err != nil {
		t.Fatal(err)
	}
	board := read(t, cfg.Dispatch.Output)

	const mine = "- **Next Action:** call the PO about the cutoff"
	var edited []string
	replaced := false
	for _, line := range strings.Split(board, "\n") {
		if !replaced && strings.HasPrefix(strings.TrimSpace(line), "- **Next Action:**") {
			line, replaced = mine, true
		}
		edited = append(edited, line)
	}
	if !replaced {
		t.Fatalf("no Next Action line in the generated board:\n%s", board)
	}
	if err := os.WriteFile(cfg.Dispatch.Output, []byte(strings.Join(edited, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := sync(cfg, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, cfg.Dispatch.Output); !strings.Contains(got, "call the PO about the cutoff") {
		t.Errorf("the hand-written Next Action was lost:\n%s", got)
	}
}
