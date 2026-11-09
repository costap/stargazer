package render

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/costap/stargazer/internal/domain"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func str(s string) *string { return &s }

func sampleBoard() *domain.DispatchBoard {
	return &domain.DispatchBoard{
		SyncedAt: time.Date(2026, 11, 9, 11, 22, 14, 0, time.UTC),
		WIPLimit: 1,
		Roadmap: &domain.RoadmapManifest{
			Phases: []domain.Phase{{ID: "phase-1", Needs: []domain.PhaseNeed{
				{Kind: domain.NeedKindDecision, Gist: "Region count", Owner: "product", Blocks: []string{"PROJ-1"}},
				{Kind: domain.NeedKindADR, Ref: str("3-platform/0031"), Owner: "me"},
			}}},
		},
		ActiveADRs: []domain.ADR{
			{QualifiedID: "3-platform/0031", Title: "CI scanning", Filename: "0031-ci.md", Date: "2026-10-01",
				Status: domain.StatusProposed, StatusLine: "Proposed - pending PoC", EpochID: "phase-1",
				NextAction: "ask the vendor", NextActionSet: true},
			{QualifiedID: "3-platform/0040", Title: "Cluster upgrades", Filename: "0040-up.md",
				Status: domain.StatusProposed, StatusLine: "Proposed", NextAction: "Review RFC comments"},
		},
		Handoffs: []domain.ADR{
			{QualifiedID: "2-product/0099", Title: "Order idempotency", Filename: "0099-o.md", Date: "2026-10-20",
				Status: domain.StatusAccepted, StatusLine: "Accepted", Dispatched: "PROJ-5301"},
			{QualifiedID: "2-product/0100", Title: "Cart ownership", Filename: "0100-c.md", Date: "2026-10-22",
				Status: domain.StatusAccepted, StatusLine: "Accepted"},
		},
		DispatchedHidden: 4,
		Warnings:         []string{"roadmap x: unreadable"},
	}
}

func TestJSONMatchesTheGoldenFile(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, sampleBoard()); err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "board.golden.json")
	if *update {
		if err := os.WriteFile(golden, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("the JSON shape changed. If this is deliberate, bump JSONSchemaVersion when it is a breaking change and run `go test ./internal/render -update`.\n--- got\n%s\n--- want\n%s", buf.String(), want)
	}
}

func TestJSONNeverEmitsNullForLists(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, &domain.DispatchBoard{SyncedAt: time.Now(), WIPLimit: 3}); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"active", "handoffs", "gaps", "warnings"} {
		if _, ok := m[k].([]any); !ok {
			t.Errorf("%q must be a list, got %v", k, m[k])
		}
	}
	if m["launch"] != nil {
		t.Errorf("launch must be null without a roadmap date, got %v", m["launch"])
	}
	if int(m["schema"].(float64)) != JSONSchemaVersion {
		t.Errorf("schema = %v", m["schema"])
	}
}

// The day count depends on today, so it cannot sit in the golden file.
func TestJSONCarriesTheLaunchDate(t *testing.T) {
	b := sampleBoard()
	b.Roadmap.LaunchDate = "2999-01-01"
	doc := BuildDocument(b)
	if doc.Launch == nil || doc.Launch.Date != "2999-01-01" || doc.Launch.Days <= 0 {
		t.Errorf("launch = %+v", doc.Launch)
	}
}
