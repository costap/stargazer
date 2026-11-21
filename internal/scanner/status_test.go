package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/costap/stargazer/internal/domain"
)

func parse(t *testing.T, filename, body string) domain.ADR {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, filename)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := ParseADRFile("2-product", p, filename)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestStatusParsing(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   domain.ADRStatus
	}{
		{"plain proposed", "Proposed", domain.StatusProposed},
		{"proposed with a suffix", "Proposed - pending PoC", domain.StatusProposed},
		{"proposed lower case", "proposed", domain.StatusProposed},
		{"plain accepted", "Accepted", domain.StatusAccepted},
		{"accepted with a date", "Accepted (2026-10-01)", domain.StatusAccepted},
		{"rejected", "Rejected", domain.StatusRejected},
		{"superseded by", "Superseded by ADR-0100", domain.StatusSuperseded},
		{"accepted, then a partial supersession note", "Accepted\n\nPartially superseded by ADR-0099", domain.StatusAccepted},
		{"unrecognised keeps the default", "Draft", domain.StatusProposed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := parse(t, "0007-x.md", "# 7. X\n\nDate: 2026-01-01\n\n## Status\n\n"+tc.status+"\n\n## Context\n\nx\n")
			if a.Status != tc.want {
				t.Errorf("status %q parsed as %s, want %s", tc.status, a.Status, tc.want)
			}
		})
	}
}

// Words in other sections must never change the status.
func TestStatusOnlyComesFromTheStatusSection(t *testing.T) {
	a := parse(t, "0007-x.md", "# 7. X\n\n## Status\n\nProposed\n\n## Context\n\nAccepted practice elsewhere.\n\n## Decision\n\nRejected options are listed below.\n")
	if a.Status != domain.StatusProposed {
		t.Errorf("status leaked from another section: %s", a.Status)
	}
}

func TestIdentityFromFilenameAndTitle(t *testing.T) {
	a := parse(t, "0099-order-idempotency.md", "# 99. Order placement idempotency\n\n## Status\n\nProposed\n")
	if a.Number != 99 || a.RawID != "0099" || a.QualifiedID != "2-product/0099" {
		t.Errorf("identity wrong: %+v", a)
	}
	if a.Title != "Order placement idempotency" {
		t.Errorf("title wrong: %q", a.Title)
	}
}

// The two real tracks reuse numbers; both must survive a scan as distinct ADRs.
func TestTwoTracksSharingANumberStayDistinct(t *testing.T) {
	prod, plt := t.TempDir(), t.TempDir()
	writeADR(t, prod, "0031-broker.md", "Broker", "Accepted")
	writeADR(t, plt, "0031-scanning.md", "Scanning", "Proposed")

	adrs, err := ScanADRDir("2-product", prod)
	if err != nil {
		t.Fatal(err)
	}
	more, err := ScanADRDir("3-platform", plt)
	if err != nil {
		t.Fatal(err)
	}
	adrs = append(adrs, more...)

	ids := map[string]domain.ADRStatus{}
	for _, a := range adrs {
		ids[a.QualifiedID] = a.Status
	}
	if len(ids) != 2 || ids["2-product/0031"] != domain.StatusAccepted || ids["3-platform/0031"] != domain.StatusProposed {
		t.Errorf("tracks collided or lost status: %v", ids)
	}
}

func TestDateAndStatusLineAreCaptured(t *testing.T) {
	a := parse(t, "0007-x.md", "# 7. X\n\nDate: 2026-10-26\n\n## Status\n\nProposed - pending PoC\n\nRelated to: ADR-0001\n\n## Context\n\nDate: 1999-01-01\n")
	if a.Date != "2026-10-26" {
		t.Errorf("date = %q (the first Date line wins)", a.Date)
	}
	if a.StatusLine != "Proposed - pending PoC" {
		t.Errorf("status line = %q (only the first line under Status)", a.StatusLine)
	}
	b := parse(t, "0008-y.md", "# 8. Y\n\n## Status\n\nAccepted\n")
	if b.Date != "" {
		t.Errorf("an ADR without a date has none, got %q", b.Date)
	}
}

func TestLoadDispatchStateReadsFronts(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.yaml")
	y := "fronts:\n  provisioning:\n    title: Provisioning\n    next: settle risk delegation\n    ref: PR 49\n"
	if err := os.WriteFile(p, []byte(y), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadDispatchState(p)
	if err != nil {
		t.Fatal(err)
	}
	f := st.OrderedFronts()
	if len(f) != 1 || f[0].ID != "provisioning" || f[0].Next != "settle risk delegation" || f[0].Ref != "PR 49" {
		t.Errorf("fronts: %+v", f)
	}
}
