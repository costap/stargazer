package render

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/costap/stargazer/internal/domain"
)

var (
	// Regex to extract ADR IDs from headers: "### [2-PRODUCT/0099] ..." or "- [ ] **[0012] ...**"
	headerIDRegex = regexp.MustCompile(`\[([0-9A-Za-z_\-\/]+)\]`)
	// Regex to capture manual next action lines
	actionRegex = regexp.MustCompile(`(?i)-\s*\*\*Next Action:\*\*\s*(.*)$`)
)

// ParseExistingDispatch scans an existing ARCH_DISPATCH.md and preserves manual Next Actions.
// Returns a map keyed by both raw ID ("0099") and qualified ID ("2-PRODUCT/0099").
func ParseExistingDispatch(filePath string) map[string]string {
	actions := make(map[string]string)
	file, err := os.Open(filePath)
	if err != nil {
		return actions
	}
	defer file.Close()

	var currentID string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Look for ADR item headers
		if strings.HasPrefix(line, "### [") || strings.HasPrefix(line, "- [ ] **[") || strings.HasPrefix(line, "- [x] **[") {
			if m := headerIDRegex.FindStringSubmatch(line); len(m) > 1 {
				currentID = strings.ToUpper(strings.TrimSpace(m[1]))
			}
		}

		// If we are under an ADR and hit Next Action, extract it
		if currentID != "" {
			if m := actionRegex.FindStringSubmatch(line); len(m) > 1 {
				action := strings.TrimSpace(m[1])
				actions[currentID] = action

				// Also index by bare number if track-qualified (e.g. "2-PRODUCT/0099" -> "0099")
				if parts := strings.Split(currentID, "/"); len(parts) == 2 {
					actions[parts[1]] = action
				}
				currentID = ""
			}
		}
	}

	return actions
}

// WriteMarkdownBoard renders the entire domain.DispatchBoard into the target file.
func WriteMarkdownBoard(filePath string, board *domain.DispatchBoard) error {
	var buf bytes.Buffer

	// Title & Metadata
	buf.WriteString("# ARCHITECTURAL DISPATCH BOARD\n\n")
	if board.Degraded() {
		buf.WriteString("> **DEGRADED BOARD:** some inputs could not be read, so this board may be incomplete.  \n")
		for _, w := range board.Warnings {
			buf.WriteString(fmt.Sprintf("> - %s  \n", w))
		}
		buf.WriteString("\n")
	}
	buf.WriteString(fmt.Sprintf("> **Last Sync:** %s  \n", board.SyncedAt.Format("2006-01-02 15:04:05 MST")))

	if board.Roadmap != nil {
		asOf := board.Roadmap.RoadmapAsOf
		if asOf == "" {
			asOf = "unspecified"
		}
		buf.WriteString(fmt.Sprintf("> **Roadmap As Of:** %s  \n", asOf))

		if board.Roadmap.LaunchDate != "" {
			days := board.Roadmap.DaysUntilLaunch()
			buf.WriteString(fmt.Sprintf("> **Target Launch:** `%s` (**L-%d days**)\n\n", board.Roadmap.LaunchDate, days))
		} else {
			buf.WriteString("\n")
		}
	}

	buf.WriteString("---\n\n")

	// 1. Upstream Roadmap & Delivery Epochs (Do vs. Delegate Partition)
	if board.Roadmap != nil && len(board.Roadmap.Phases) > 0 {
		buf.WriteString("## 🗺️ Product Roadmap & Delivery Epochs\n\n")

		for _, phase := range board.Roadmap.Phases {
			badge := "⏳ PLANNED"
			if strings.EqualFold(phase.Status, "in-flight") || strings.EqualFold(phase.Status, "active") {
				badge = "🔥 IN-FLIGHT"
			}

			buf.WriteString(fmt.Sprintf("### %s: %s (`%s`) — Target: %s\n", badge, phase.Name, phase.ID, phase.Target))
			if phase.Scope != "" {
				buf.WriteString(fmt.Sprintf("**Scope:** %s\n\n", strings.TrimSpace(phase.Scope)))
			}

			// Split needs: "owner: me" vs "owner: other"
			var myNeeds []domain.PhaseNeed
			var delegatedNeeds []domain.PhaseNeed

			for _, need := range phase.Needs {
				if need.IsMine() {
					myNeeds = append(myNeeds, need)
				} else {
					delegatedNeeds = append(delegatedNeeds, need)
				}
			}

			if len(myNeeds) > 0 {
				buf.WriteString("#### 🔴 My Critical Path (`owner: me`)\n")
				for _, need := range myNeeds {
					refStr := need.DisplayRef()
					squadStr := ""
					if need.Squad != "" {
						squadStr = fmt.Sprintf(" *(Squad: %s)*", need.Squad)
					}
					buf.WriteString(fmt.Sprintf("- **[%s]** `%s`%s\n", strings.ToUpper(string(need.Kind)), refStr, squadStr))
					if need.Gist != "" {
						buf.WriteString(fmt.Sprintf("  - *Gist:* %s\n", need.Gist))
					}
					if need.Note != "" {
						buf.WriteString(fmt.Sprintf("  - *Note:* %s\n", need.Note))
					}
					if len(need.Blocks) > 0 {
						buf.WriteString(fmt.Sprintf("  - *Blocks:* `%s`\n", strings.Join(need.Blocks, "`, `")))
					}
				}
				buf.WriteString("\n")
			}

			if len(delegatedNeeds) > 0 {
				buf.WriteString("#### 🟡 Delegated / Blocked Downstream\n")
				for _, need := range delegatedNeeds {
					refStr := need.DisplayRef()
					ownerStr := strings.ToUpper(need.Owner)
					if ownerStr == "" {
						ownerStr = "UNASSIGNED"
					}
					buf.WriteString(fmt.Sprintf("- **[%s]** `%s` — **Owner:** `%s`\n", strings.ToUpper(string(need.Kind)), refStr, ownerStr))
					if need.Gist != "" {
						buf.WriteString(fmt.Sprintf("  - *Gist:* %s\n", need.Gist))
					}
					if need.Note != "" {
						buf.WriteString(fmt.Sprintf("  - *Note:* %s\n", need.Note))
					}
					if len(need.Blocks) > 0 {
						buf.WriteString(fmt.Sprintf("  - *Blocks:* `%s`\n", strings.Join(need.Blocks, "`, `")))
					}
				}
				buf.WriteString("\n")
			}
		}
		buf.WriteString("---\n\n")
	}

	// 2. Active Pipeline: Proposed ADRs (WIP limit enforcement)
	wipWarn := ""
	if board.HasWIPViolation() {
		wipWarn = fmt.Sprintf(" ⚠️ **WIP VIOLATION (%d active > limit %d)**", len(board.ActiveADRs), board.WIPLimit)
	}
	buf.WriteString(fmt.Sprintf("## 🚀 Active Pipeline (Proposed / Under Review)%s\n", wipWarn))
	buf.WriteString("*RFCs requiring decisions, benchmarks, or lead sign-off.*\n\n")

	if len(board.ActiveADRs) == 0 {
		if board.ADRUnknown {
			buf.WriteString("_UNKNOWN: the ADR sources could not be read, so WIP cannot be stated._\n\n")
		} else {
			buf.WriteString("_No active proposed ADRs in flight._\n\n")
		}
	} else {
		for _, adr := range board.ActiveADRs {
			epochBadge := ""
			if adr.EpochID != "" {
				epochBadge = fmt.Sprintf(" `[%s]`", adr.EpochID)
			}
			buf.WriteString(fmt.Sprintf("### [%s] %s%s\n", adr.DisplayName(), adr.Title, epochBadge))
			buf.WriteString(fmt.Sprintf("- **Track:** `%s` | **File:** `%s` *(Updated: %s)*\n", adr.Track, adr.Filename, adr.LastCommit))
			buf.WriteString(fmt.Sprintf("- **Next Action:** %s\n\n", adr.NextAction))
		}
	}

	buf.WriteString("---\n\n")

	// 3. Ready for Hand-off: Accepted ADRs (Ready for Story-Gen Skill)
	buf.WriteString("## 📦 Ready for Hand-off (Accepted -> Pod Slicing)\n")
	buf.WriteString("*Decisions closed; ready to run story generator skill and hand off to PMs.*\n\n")

	if len(board.Handoffs) == 0 {
		if board.ADRUnknown {
			buf.WriteString("_UNKNOWN: the ADR sources could not be read._\n\n")
		} else {
			buf.WriteString("_No pending hand-offs._\n\n")
		}
	} else {
		for _, adr := range board.Handoffs {
			epochBadge := ""
			if adr.EpochID != "" {
				epochBadge = fmt.Sprintf(" `[%s]`", adr.EpochID)
			}
			buf.WriteString(fmt.Sprintf("- [ ] **[%s] %s**%s\n", adr.DisplayName(), adr.Title, epochBadge))
			buf.WriteString(fmt.Sprintf("  - **File:** `%s`\n", adr.Filename))
			buf.WriteString(fmt.Sprintf("  - **Next Action:** %s\n", adr.NextAction))
		}
		buf.WriteString("\n")
	}

	buf.WriteString("---\n\n")

	// 4. Radar Signals & Candidate PoCs
	buf.WriteString("## 🔬 Radar Signals & Candidate PoCs (Astrometrics)\n")
	buf.WriteString("*Active candidate technologies and architectural signals from V'Ger / Astrometrics.*\n\n")

	if len(board.Signals) == 0 {
		buf.WriteString("_No active candidate signals loaded._\n\n")
	} else {
		for _, s := range board.Signals {
			buf.WriteString(fmt.Sprintf("### [%s] %s (`%s` — %s)\n", s.ID, s.Title, s.Category, strings.ToUpper(s.Status)))
			if s.URL != "" {
				buf.WriteString(fmt.Sprintf("- **Reference:** <%s>\n", s.URL))
			}
			if s.WhyCaptured != "" {
				buf.WriteString(fmt.Sprintf("- **Why Captured:** %s\n", s.WhyCaptured))
			}
			if s.Enrichment.StackFit != "" {
				buf.WriteString(fmt.Sprintf("- **Stack Fit:** %s\n", s.Enrichment.StackFit))
			}
			if len(s.Enrichment.NextSteps) > 0 {
				buf.WriteString("- **Candidate PoC Tasks:**\n")
				for _, step := range s.Enrichment.NextSteps {
					buf.WriteString(fmt.Sprintf("  - [ ] %s\n", step))
				}
			}
			buf.WriteString("\n")
		}
	}

	buf.WriteString("---\n\n")

	// 5. Asynchronous Audio Queue (YouTube Conference Talks)
	if len(board.AudioQueue) > 0 {
		buf.WriteString("## 🎧 Asynchronous Audio Queue (Walks / Transit / Off-Desk)\n")
		buf.WriteString("*Prioritized conference talks for mobile/offline listening.*\n\n")

		for i, talk := range board.AudioQueue {
			buf.WriteString(fmt.Sprintf("%d. **[%s] %s**\n", i+1, talk.ID, talk.Title))
			buf.WriteString(fmt.Sprintf("   - **Link:** <%s>\n", talk.URL))
			if talk.Context != "" {
				buf.WriteString(fmt.Sprintf("   - *Context:* %s\n", talk.Context))
			}
		}
		buf.WriteString("\n")
	}

	return os.WriteFile(filePath, buf.Bytes(), 0644)
}
