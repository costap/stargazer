package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/costap/stargazer/internal/config"
)

func writeADR(t *testing.T, dir, name, title, status string) {
	t.Helper()
	body := "# 1. " + title + "\n\nDate: 2026-01-01\n\n## Status\n\n" + status + "\n\n## Context\n\nx\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestScanAllADRGroupsReportsAGroupItCannotRead(t *testing.T) {
	good := t.TempDir()
	writeADR(t, good, "0001-a.md", "A", "Proposed")
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	adrs, err := ScanAllADRGroups([]config.ADRGroupConfig{
		{Namespace: "2-product", Dir: good},
		{Namespace: "3-platform", Dir: missing},
	})
	if err == nil {
		t.Fatal("an unreadable group must produce an error, not be skipped")
	}
	if !strings.Contains(err.Error(), "3-platform") || !strings.Contains(err.Error(), missing) {
		t.Errorf("error should name the group and the path: %v", err)
	}
	if len(adrs) != 1 || adrs[0].QualifiedID != "2-product/0001" {
		t.Errorf("ADRs from the readable group must still be returned, got %+v", adrs)
	}
}

func TestScanAllADRGroupsReportsEveryFailedGroup(t *testing.T) {
	root := t.TempDir()
	_, err := ScanAllADRGroups([]config.ADRGroupConfig{
		{Namespace: "a", Dir: filepath.Join(root, "x")},
		{Namespace: "b", Dir: filepath.Join(root, "y")},
	})
	if err == nil || !strings.Contains(err.Error(), "[a]") || !strings.Contains(err.Error(), "[b]") {
		t.Errorf("both failures should be reported: %v", err)
	}
}

func TestScanAllADRGroupsNoErrorWhenAllGroupsRead(t *testing.T) {
	dir := t.TempDir()
	writeADR(t, dir, "0001-a.md", "A", "Accepted")
	adrs, err := ScanAllADRGroups([]config.ADRGroupConfig{{Namespace: "2-product", Dir: dir}})
	if err != nil || len(adrs) != 1 {
		t.Fatalf("err=%v adrs=%d", err, len(adrs))
	}
}
