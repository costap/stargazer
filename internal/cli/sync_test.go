package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
