package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCand(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanCandidatesReturnsOnlyOpenOnesOldestFirst(t *testing.T) {
	dir := t.TempDir()
	writeCand(t, dir, "002-b.md", "# 002 — Second\n\n- **Status:** open\n- **Raised:** 2026-10-05\n- **Source:** chat\n\n## Problem\n- **Status:** promoted\n")
	writeCand(t, dir, "001-a.md", "# 001 — First\n\n- **Status:** open\n- **Raised:** 2026-09-01\n")
	writeCand(t, dir, "003-c.md", "# 003 — Done\n\n- **Status:** promoted\n- **Raised:** 2026-08-01\n")
	writeCand(t, dir, "004-d.md", "# 004 — Gone\n\n- **Status:** dropped\n")
	writeCand(t, dir, "README.md", "# index\n")
	writeCand(t, dir, "regen.py", "x")

	got, err := ScanCandidates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "001" || got[1].ID != "002" {
		t.Fatalf("want 001 then 002 only, got %+v", got)
	}
	if got[0].Title != "First" || got[1].Source != "chat" || got[1].Raised != "2026-10-05" {
		t.Errorf("fields: %+v", got)
	}
	// "## " ends the header: a Status line further down must not change the answer.
}

func TestScanCandidatesNamesAFileItCannotJudge(t *testing.T) {
	dir := t.TempDir()
	writeCand(t, dir, "001-a.md", "# 001 — First\n\n- **Status:** open\n")
	writeCand(t, dir, "002-b.md", "# 002 — No status\n\nbody only\n")

	got, err := ScanCandidates(dir)
	if err == nil || !strings.Contains(err.Error(), "002-b.md") {
		t.Fatalf("want an error naming 002-b.md, got %v", err)
	}
	if len(got) != 1 {
		t.Errorf("the readable open candidate must still be returned: %+v", got)
	}
}

func TestScanCandidatesMissingDirFails(t *testing.T) {
	if _, err := ScanCandidates(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("a configured directory that does not exist must be an error")
	}
}
