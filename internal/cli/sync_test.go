package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Drives the real command line: flags, Viper binding and config loading together.
func TestSyncFlagsReachTheBoard(t *testing.T) {
	dir := t.TempDir()
	adrs := filepath.Join(dir, "adrs")
	if err := os.Mkdir(adrs, 0o700); err != nil {
		t.Fatal(err)
	}
	adr := "# 1. Thing\n\nDate: 2026-01-01\n\n## Status\n\nProposed\n"
	if err := os.WriteFile(filepath.Join(adrs, "0001-thing.md"), []byte(adr), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, ".stargazer.yaml")
	cfg := "adr:\n  wip_limit: 3\n  groups:\n    - namespace: 1-test\n      dir: adrs\ndispatch:\n  output: board.md\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "board.json")

	rootCmd.SetArgs([]string{"sync", "-c", cfgPath, "--wip-limit", "2", "--silent", "--json=" + out})
	if err := Execute(context.Background()); err != nil {
		t.Fatal(err)
	}

	var doc struct {
		WIP struct{ Limit, Count int } `json:"wip"`
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.WIP.Limit != 2 || doc.WIP.Count != 1 {
		t.Errorf("--wip-limit 2 must reach the board, got %+v", doc.WIP)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	adrs := filepath.Join(dir, "adrs")
	if err := os.Mkdir(adrs, 0o700); err != nil {
		t.Fatal(err)
	}
	adr := "# 1. Thing\n\nDate: 2026-01-01\n\n## Status\n\nProposed\n"
	if err := os.WriteFile(filepath.Join(adrs, "0001-thing.md"), []byte(adr), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, ".stargazer.yaml")
	cfg := "adr:\n  wip_limit: 3\n  groups:\n    - namespace: 1-test\n      dir: adrs\ndispatch:\n  output: board.md\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // flag values outlive a single Execute
		_ = syncCmd.Flags().Set("dry-run", "false")
		_ = syncCmd.Flags().Set("json", "")
	})
	jsonOut := filepath.Join(dir, "board.json")

	rootCmd.SetArgs([]string{"sync", "-c", cfgPath, "--silent", "--dry-run", "--json=" + jsonOut})
	if err := Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"board.md", "board.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			t.Errorf("--dry-run must not write %s", f)
		}
	}

	// And without the flag the same command line does write the board.
	rootCmd.SetArgs([]string{"sync", "-c", cfgPath, "--silent", "--dry-run=false", "--json="})
	if err := Execute(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "board.md")); err != nil {
		t.Errorf("without --dry-run the board is written: %v", err)
	}
}

func TestPlaylistLoginWithoutAClientFileSaysWhatToSet(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".stargazer.yaml")
	if err := os.WriteFile(cfgPath, []byte("adr:\n  groups:\n    - namespace: x\n      dir: adrs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{"playlist", "login", "-c", cfgPath})
	err := Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "youtube.client_file") {
		t.Errorf("want a hint naming youtube.client_file, got %v", err)
	}
}
