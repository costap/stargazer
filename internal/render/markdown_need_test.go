package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/costap/stargazer/internal/domain"
)

func TestDelegatedNeedShowsDescriptionAndReviewer(t *testing.T) {
	doc := "3-platform/docs/01-Status.md"
	b := sampleBoard()
	b.Roadmap.Phases[0].Needs = []domain.PhaseNeed{
		{ID: "environments", Kind: domain.NeedKind("outcome"), Ref: &doc, Owner: "team", Review: "me", Title: "All environments except DR"},
	}
	path := filepath.Join(t.TempDir(), "BOARD.md")
	if err := WriteMarkdownBoard(path, b); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	out := string(data)
	for _, want := range []string{"`environments`", "All environments except DR", "TEAM, REVIEW: ME"} {
		if !strings.Contains(out, want) {
			t.Errorf("board missing %q", want)
		}
	}
	if strings.Contains(out, "(no ADR yet)") {
		t.Error("an outcome with an id must not read as an unassigned decision")
	}
}
