package domain

import "testing"

func TestShortDescTitleWins(t *testing.T) {
	n := PhaseNeed{Title: "All environments except DR", Gist: "A long first sentence. Second."}
	if got := n.ShortDesc(80); got != "All environments except DR" {
		t.Fatalf("got %q", got)
	}
}

func TestShortDescFallsBackToFirstSentence(t *testing.T) {
	n := PhaseNeed{Gist: "Operator admin operations finished: provisioned end to end.\n  Second sentence. Third."}
	if got := n.ShortDesc(120); got != "Operator admin operations finished: provisioned end to end." {
		t.Fatalf("got %q", got)
	}
}

func TestShortDescTruncatesOnRunes(t *testing.T) {
	n := PhaseNeed{Title: "Zürich ünïcode title that is long"}
	got := n.ShortDesc(10)
	if r := []rune(got); len(r) != 10 || r[9] != '…' {
		t.Fatalf("got %q (%d runes)", got, len(r))
	}
}

func TestShortDescEmptyWhenNothingToSay(t *testing.T) {
	if got := (&PhaseNeed{}).ShortDesc(40); got != "" {
		t.Fatalf("invented %q", got)
	}
}

func TestDisplayRefPrefersOutcomeID(t *testing.T) {
	doc := "3-platform/docs/01-Status.md"
	n := PhaseNeed{ID: "environments", Ref: &doc}
	if got := n.DisplayRef(); got != "environments" {
		t.Fatalf("got %q", got)
	}
	n = PhaseNeed{Ref: &doc}
	if got := n.DisplayRef(); got != doc {
		t.Fatalf("got %q", got)
	}
	if got := (&PhaseNeed{}).DisplayRef(); got != "(no ADR yet)" {
		t.Fatalf("got %q", got)
	}
}

func TestOwnerTag(t *testing.T) {
	cases := []struct {
		n    PhaseNeed
		want string
	}{
		{PhaseNeed{Owner: "team", Review: "me"}, "TEAM, REVIEW: ME"},
		{PhaseNeed{Owner: "product", With: "compliance", Review: "me"}, "PRODUCT, WITH: COMPLIANCE, REVIEW: ME"},
		{PhaseNeed{}, "UNASSIGNED"},
	}
	for _, c := range cases {
		if got := c.n.OwnerTag(); got != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
}
