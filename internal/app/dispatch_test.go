package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/costap/stargazer/internal/config"
	"github.com/costap/stargazer/internal/domain"
	"github.com/costap/stargazer/internal/render"
)

// addADR writes another ADR into the fixture's only group.
func addADR(t *testing.T, cfg *config.Config, file, title, status string) {
	t.Helper()
	body := "# 1. " + title + "\n\nDate: 2026-02-03\n\n## Status\n\n" + status + "\n\n## Context\n\nx\n"
	if err := os.WriteFile(filepath.Join(cfg.ADR.Groups[0].Dir, file), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeState(t *testing.T, cfg *config.Config, body string) {
	t.Helper()
	cfg.State.File = filepath.Join(filepath.Dir(cfg.Dispatch.Output), "state.yaml")
	if err := os.WriteFile(cfg.State.File, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func syncOpts(cfg *config.Config, opts SyncOptions) (string, error) {
	var out bytes.Buffer
	e := NewEngine(cfg)
	e.Stdout = &out
	err := e.SyncWithOptions(context.Background(), opts)
	return out.String(), err
}

func handoffSection(board string) string {
	i := strings.Index(board, "Ready for Hand-off")
	j := strings.Index(board, "Radar Signals")
	if i < 0 || j < i {
		return board
	}
	return board[i:j]
}

func TestAnAcceptedADRIsListedUntilItIsMarkedDispatched(t *testing.T) {
	cfg, _ := fixture(t)
	addADR(t, cfg, "0002-a.md", "Alpha", "Accepted")
	addADR(t, cfg, "0003-b.md", "Beta", "Accepted")

	if _, err := syncOpts(cfg, SyncOptions{}); err != nil {
		t.Fatal(err)
	}
	if h := handoffSection(read(t, cfg.Dispatch.Output)); !strings.Contains(h, "2-product/0002") || !strings.Contains(h, "2-product/0003") {
		t.Fatalf("before any state both Accepted ADRs are listed:\n%s", h)
	}

	writeState(t, cfg, "dispatched:\n  2-product/0002: PROJ-5301\n")
	for i := 0; i < 2; i++ { // and it stays that way on later syncs
		if _, err := syncOpts(cfg, SyncOptions{}); err != nil {
			t.Fatal(err)
		}
		h := handoffSection(read(t, cfg.Dispatch.Output))
		if strings.Contains(h, "2-product/0002") {
			t.Errorf("sync %d: the dispatched ADR must not be listed:\n%s", i+1, h)
		}
		if !strings.Contains(h, "2-product/0003") {
			t.Errorf("sync %d: the undispatched ADR must still be listed:\n%s", i+1, h)
		}
		if !strings.Contains(h, "1 dispatched ADRs not listed") {
			t.Errorf("sync %d: the board should say one was left out:\n%s", i+1, h)
		}
	}
}

func TestAllHandoffsListsDispatchedOnesAndMarksThem(t *testing.T) {
	cfg, _ := fixture(t)
	addADR(t, cfg, "0002-a.md", "Alpha", "Accepted")
	writeState(t, cfg, "dispatched:\n  2-product/0002: PROJ-5301\n")

	if _, err := syncOpts(cfg, SyncOptions{AllHandoffs: true}); err != nil {
		t.Fatal(err)
	}
	h := handoffSection(read(t, cfg.Dispatch.Output))
	if !strings.Contains(h, "- [x] **[2-product/0002]") || !strings.Contains(h, "**Dispatched:** PROJ-5301") {
		t.Errorf("a dispatched ADR should be listed, ticked and referenced:\n%s", h)
	}
}

func TestDispatchStateMatchesCaseInsensitivelyAndAcceptsAnEmptyValue(t *testing.T) {
	cfg, _ := fixture(t)
	addADR(t, cfg, "0002-a.md", "Alpha", "Accepted")
	writeState(t, cfg, "dispatched:\n  2-Product/0002:\n")
	out, err := syncOpts(cfg, SyncOptions{JSON: "-", AllHandoffs: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc render.Document
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Handoffs) != 1 || !doc.Handoffs[0].Dispatched || doc.Handoffs[0].DispatchRef != "yes" {
		t.Errorf("a key with no value still means dispatched: %+v", doc.Handoffs)
	}
}

func TestApplyDispatchStateNamesEntriesThatMatchNothing(t *testing.T) {
	adrs := []domain.ADR{
		{QualifiedID: "2-product/0031", Status: domain.StatusAccepted},
		{QualifiedID: "3-platform/0031", Status: domain.StatusAccepted},
		{QualifiedID: "2-product/0050", Status: domain.StatusProposed},
	}
	st := &domain.DispatchState{Dispatched: map[string]string{
		"2-product/0031": "PROJ-1",
		"0031":           "bare number is ambiguous",
		"2-product/0050": "not accepted yet",
		"2-product/9999": "no such ADR",
	}}
	notices := applyDispatchState(adrs, st)

	if adrs[0].Dispatched != "PROJ-1" || adrs[1].IsDispatched() || adrs[2].IsDispatched() {
		t.Errorf("only the exact, Accepted match may be marked: %+v", adrs)
	}
	joined := strings.Join(notices, "\n")
	for _, k := range []string{`"0031"`, `"2-product/0050"`, `"2-product/9999"`} {
		if !strings.Contains(joined, k) {
			t.Errorf("no notice for %s in:\n%s", k, joined)
		}
	}
	if len(notices) != 3 {
		t.Errorf("want 3 notices, got %q", notices)
	}
}

func TestMissingDefaultStateIsFineButAnExplicitOneIsNot(t *testing.T) {
	cfg, dir := fixture(t)
	cfg.State.File = filepath.Join(dir, "absent.yaml")
	if _, err := syncOpts(cfg, SyncOptions{}); err != nil {
		t.Fatalf("a missing state file at the default path is normal: %v", err)
	}
	cfg.State.Explicit = true
	if _, err := syncOpts(cfg, SyncOptions{}); err == nil {
		t.Fatal("a state file you named that is missing must fail the sync")
	}
}

func TestMalformedStateFileIsAWarning(t *testing.T) {
	cfg, _ := fixture(t)
	writeState(t, cfg, "dispatched: [unclosed\n")
	if _, err := syncOpts(cfg, SyncOptions{}); err == nil {
		t.Fatal("a state file that cannot be parsed must not be ignored")
	}
}

// JSON on stdout must be the whole of stdout, and must carry what the markdown carries.
func TestJSONToStdoutIsPureAndDescribesTheBoard(t *testing.T) {
	cfg, _ := fixture(t)
	addADR(t, cfg, "0002-a.md", "Alpha", "Accepted")
	out, err := syncOpts(cfg, SyncOptions{JSON: "-"})
	if err != nil {
		t.Fatal(err)
	}
	var doc render.Document
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout is not a single JSON document: %v\n%s", err, out)
	}
	if strings.TrimSpace(out)[0] != '{' || !strings.HasSuffix(strings.TrimSpace(out), "}") {
		t.Errorf("stdout carries something besides the document:\n%s", out)
	}
	if doc.Schema != render.JSONSchemaVersion || doc.WIP.Count != 1 || doc.WIP.Limit != 3 {
		t.Errorf("wip/schema wrong: %+v", doc)
	}
	a := doc.Active[0]
	if a.ID != "2-product/0001" || a.Date != "2026-01-01" || a.StatusLine != "Proposed" || a.NextActionSet {
		t.Errorf("active ADR wrong (a generated default is not a set action): %+v", a)
	}
	if len(doc.Handoffs) != 1 || doc.Handoffs[0].ID != "2-product/0002" {
		t.Errorf("handoffs wrong: %+v", doc.Handoffs)
	}
	if _, err := os.Stat(cfg.Dispatch.Output); err != nil {
		t.Error("--json must not stop the markdown board being written")
	}
}

func TestNextActionSetIsTrueOnlyForHandWrittenNotes(t *testing.T) {
	cfg, _ := fixture(t)
	if _, err := syncOpts(cfg, SyncOptions{}); err != nil { // writes the default into the board
		t.Fatal(err)
	}
	out, _ := syncOpts(cfg, SyncOptions{JSON: "-"})
	var doc render.Document
	_ = json.Unmarshal([]byte(out), &doc)
	if doc.Active[0].NextActionSet {
		t.Error("a default that was written back by an earlier sync is still not a note of mine")
	}

	board := read(t, cfg.Dispatch.Output)
	board = strings.Replace(board, "- **Next Action:** "+defaultProposedAction, "- **Next Action:** phone the vendor", 1)
	if err := os.WriteFile(cfg.Dispatch.Output, []byte(board), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _ = syncOpts(cfg, SyncOptions{JSON: "-"})
	_ = json.Unmarshal([]byte(out), &doc)
	if !doc.Active[0].NextActionSet || doc.Active[0].NextAction != "phone the vendor" {
		t.Errorf("a hand-written note must be reported as set: %+v", doc.Active[0])
	}
}

func TestJSONFlagWithAPathWritesAFile(t *testing.T) {
	cfg, dir := fixture(t)
	path := filepath.Join(dir, "board.json")
	out, err := syncOpts(cfg, SyncOptions{JSON: path})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("nothing may go to stdout when a path was given: %q", out)
	}
	var doc render.Document
	if err := json.Unmarshal([]byte(read(t, path)), &doc); err != nil || len(doc.Active) != 1 {
		t.Errorf("file is not the board: err=%v doc=%+v", err, doc)
	}
}

// A consumer must be able to tell a degraded board from a calm one by the JSON alone.
func TestDegradedJSONSaysSo(t *testing.T) {
	cfg, dir := fixture(t)
	cfg.ADR.Groups = append(cfg.ADR.Groups, config.ADRGroupConfig{Namespace: "3-platform", Dir: filepath.Join(dir, "missing")})
	out, err := syncOpts(cfg, SyncOptions{JSON: "-", KeepGoing: true})
	if err != nil {
		t.Fatal(err)
	}
	var doc render.Document
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.ADRUnknown || len(doc.Warnings) == 0 || !strings.Contains(strings.Join(doc.Warnings, " "), "3-platform") {
		t.Errorf("degraded state must appear in the JSON: %+v", doc)
	}
}
