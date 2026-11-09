package render

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/costap/stargazer/internal/domain"
)

// LCARS colour palette
var (
	amber    = lipgloss.Color("#FF9900")
	blue     = lipgloss.Color("#99CCFF")
	red      = lipgloss.Color("#CC4444")
	green    = lipgloss.Color("#33CC99")
	yellow   = lipgloss.Color("#FFCC33")
	white    = lipgloss.Color("#FFFFFF")
	dimGrey  = lipgloss.Color("#666666")
	darkPill = lipgloss.Color("#222222")
)

var (
	labelStyle = lipgloss.NewStyle().Foreground(amber).Bold(false)
	bodyStyle  = lipgloss.NewStyle().Foreground(white)
	greenStyle = lipgloss.NewStyle().Foreground(green).Bold(false)
	amberStyle = lipgloss.NewStyle().Foreground(amber)
	blueStyle  = lipgloss.NewStyle().Foreground(blue)
	whiteStyle = lipgloss.NewStyle().Foreground(white)
	dimStyle   = lipgloss.NewStyle().Foreground(dimGrey)
	redStyle   = lipgloss.NewStyle().Foreground(red).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(yellow)

	// LCARS badge styles
	capRegistry = lipgloss.NewStyle().Background(blue).Foreground(darkPill).Bold(true).Padding(0, 1)
	capTitle    = lipgloss.NewStyle().Background(amber).Foreground(darkPill).Bold(true).Padding(0, 1)
	pillStatus  = lipgloss.NewStyle().Background(amber).Foreground(darkPill).Bold(true).Padding(0, 1)
	pillAlert   = lipgloss.NewStyle().Background(red).Foreground(white).Bold(true).Padding(0, 1)
	pillMuted   = lipgloss.NewStyle().Background(dimGrey).Foreground(white).Padding(0, 1)

	pillActive = lipgloss.NewStyle().Background(amber).Foreground(darkPill).Bold(true).Padding(0, 1)
	pillOk     = lipgloss.NewStyle().Background(green).Foreground(darkPill).Bold(true).Padding(0, 1)
)

// LabelStyle returns the amber label style for use outside this package.
func LabelStyle() lipgloss.Style { return labelStyle }

// Field prints a key/value pair in LCARS style.
func Field(key, value string) {
	fmt.Printf("  %s  %s\n",
		labelStyle.Render(fmt.Sprintf("%-20s", strings.ToUpper(key))),
		bodyStyle.Render(value),
	)
}

// SectionHeader prints a named section divider in amber with an LCARS elbow marker.
func SectionHeader(title string) {
	fmt.Printf("\n  %s\n  %s\n",
		labelStyle.Render("▐▌ "+strings.ToUpper(title)),
		dimStyle.Render(strings.Repeat("─", 62)),
	)
}

func BannerHeader(stardate, title, badge string) {
	bannerStyle := lipgloss.NewStyle().
		Background(amber).
		Foreground(darkPill).
		Bold(true).
		Padding(0, 1)

	badgeStyle := lipgloss.NewStyle().
		Background(blue).
		Foreground(darkPill).
		Bold(true).
		Padding(0, 1)

	fmt.Printf("\n %s %s %s\n",
		badgeStyle.Render("USS STARGAZER"),
		bannerStyle.Render(fmt.Sprintf("%-45s", title)),
		badgeStyle.Render(badge),
	)
}

func SectionFrameStart(code, title string) {
	fmt.Printf("  %s %s %s\n",
		amberStyle.Render("╭──●"),
		pillActive.Render(code),
		labelStyle.Render(title),
	)
	fmt.Printf("  %s\n", dimStyle.Render("│"))
}

func SectionFrameEnd(subcode string) {
	fmt.Printf("  %s%s %s\n",
		dimStyle.Render("╰────────────────────────────────────────── "),
		amberStyle.Render("■"),
		dimStyle.Render(subcode),
	)
}

// VersionCard prints system version and build metadata in LCARS style.
func VersionCard(version, commit, date, goVersion, platform string) {
	SectionHeader("SYSTEM VERSION & BUILD METADATA")
	fmt.Println()
	Field("Version", version)
	Field("Git Commit", commit)
	Field("Build Date", date)
	Field("Go Version", goVersion)
	Field("Platform", platform)
	fmt.Println()
}

// PrintTerminalSummary renders the entire dispatch state in LCARS terminal format.
func PrintTerminalSummary(board *domain.DispatchBoard, outputPath string, dryRun bool) {

	// 1. Top LCARS Solid Header Bar
	launchCode := "L-TBD"
	launchDate := "UNSET"
	if board.Roadmap != nil && board.Roadmap.LaunchDate != "" {
		launchCode = fmt.Sprintf("L-%d", board.Roadmap.DaysUntilLaunch())
		launchDate = board.Roadmap.LaunchDate
	}

	modeBadge := fmt.Sprintf("%s [%s]", launchCode, launchDate)
	if dryRun {
		modeBadge = "DRY-RUN // NO-COMMIT"
	}

	bannerTitle := "STARGAZER SYSTEM DISPATCH"
	fmt.Printf("\n %s %s %s\n",
		capRegistry.Render(fmt.Sprintf("NCC-2882 : %s", domain.StardateString(time.Now()))),
		capTitle.Render(fmt.Sprintf("%-42s", bannerTitle)),
		capRegistry.Render(modeBadge),
	)
	if board.Degraded() {
		fmt.Printf(" %s  %s\n", pillAlert.Render("DEGRADED"), redStyle.Render("some inputs could not be read; this board may be incomplete"))
		for _, w := range board.Warnings {
			fmt.Printf("   %s\n", redStyle.Render("- "+truncate(w, 100)))
		}
	}
	fmt.Printf(" %s\n", amberStyle.Render(strings.Repeat("━", 67)))

	// Telemetry strip
	asOf := "CURRENT"
	if board.Roadmap != nil && board.Roadmap.RoadmapAsOf != "" {
		asOf = board.Roadmap.RoadmapAsOf
	}
	fmt.Printf("  %s %-16s %s %-16s %s %s\n",
		amberStyle.Render("STATUS:"), whiteStyle.Render("ONLINE"),
		amberStyle.Render("ROADMAP:"), whiteStyle.Render(asOf),
		amberStyle.Render("SYNC:"), dimStyle.Render(board.SyncedAt.Format("15:04:05 MST")),
	)

	// 2. Upstream Roadmap & Delivery Epochs (Do vs. Delegate)
	if board.Roadmap != nil && len(board.Roadmap.Phases) > 0 {
		for i, phase := range board.Roadmap.Phases {
			phaseCode := fmt.Sprintf("EP-%02d", i+1)
			statusBadge := "IN-FLIGHT"
			if !strings.EqualFold(phase.Status, "in-flight") && !strings.EqualFold(phase.Status, "active") {
				statusBadge = "HORIZON"
			}

			SectionFrameStart(phaseCode, fmt.Sprintf("%s // %s [%s]", phase.Name, phase.Target, statusBadge))

			if phase.Scope != "" {
				fmt.Printf("  %s %s %s\n",
					amberStyle.Render("│"),
					dimStyle.Render("SCOPE:"),
					whiteStyle.Render(strings.TrimSpace(phase.Scope)),
				)
				fmt.Printf("  %s\n", amberStyle.Render("│"))
			}

			// Segregate Needs: "owner: me" vs "owner: other"
			var myNeeds []domain.PhaseNeed
			var delegated []domain.PhaseNeed

			for _, need := range phase.Needs {
				if need.IsMine() {
					myNeeds = append(myNeeds, need)
				} else {
					delegated = append(delegated, need)
				}
			}

			// My Critical Path (Do)
			if len(myNeeds) > 0 {
				fmt.Printf("  %s  %s\n", amberStyle.Render("│"), redStyle.Render("▼ CRITICAL PATH (OWNER: ME)"))
				for _, need := range myNeeds {
					kindStr := fmt.Sprintf("[%-4s]", strings.ToUpper(string(need.Kind)))
					fmt.Printf("  %s    • %s %-24s %s\n",
						amberStyle.Render("│"),
						blueStyle.Render(kindStr),
						whiteStyle.Render(need.DisplayRef()),
						dimStyle.Render(truncate(need.Gist, 26)),
					)
					if need.Note != "" {
						fmt.Printf("  %s      %s %s\n",
							amberStyle.Render("│"),
							dimStyle.Render("└─►"),
							warnStyle.Render(truncate(need.Note, 52)),
						)
					}
				}
				fmt.Printf("  %s\n", amberStyle.Render("│"))
			}

			// Delegated / Blocked Downstream (Supervise)
			if len(delegated) > 0 {
				fmt.Printf("  %s  %s\n", amberStyle.Render("│"), amberStyle.Bold(true).Render("▼ DELEGATED / UPSTREAM BLOCKED"))
				for _, need := range delegated {
					kindStr := fmt.Sprintf("[%-4s]", strings.ToUpper(string(need.Kind)))
					ownerTag := strings.ToUpper(need.Owner)
					if ownerTag == "" {
						ownerTag = "UNASSIGNED"
					}
					fmt.Printf("  %s    • %s %-24s %s %s\n",
						amberStyle.Render("│"),
						dimStyle.Render(kindStr),
						whiteStyle.Render(need.DisplayRef()),
						dimStyle.Render("OWNER:"),
						amberStyle.Render(ownerTag),
					)
					if len(need.Blocks) > 0 {
						fmt.Printf("  %s      %s %s\n",
							amberStyle.Render("│"),
							dimStyle.Render("└─► BLOCKS:"),
							redStyle.Render(strings.Join(need.Blocks, ", ")),
						)
					}
				}
			}

			SectionFrameEnd(fmt.Sprintf("SEC-%04d", 1000+i*23))
		}
	}

	// 3. Active Pipeline: Proposed ADRs (WIP limit enforcement)
	SectionFrameStart("ACT-01", "ACTIVE PIPELINE // PROPOSED ADRS")
	if len(board.ActiveADRs) == 0 {
		if board.ADRUnknown {
			fmt.Printf("  %s  %s\n", amberStyle.Render("│"), redStyle.Render("UNKNOWN: the ADR sources could not be read."))
		} else {
			fmt.Printf("  %s  %s\n", amberStyle.Render("│"), dimStyle.Render("No active proposed ADRs in flight."))
		}
	} else {
		for _, adr := range board.ActiveADRs {
			trackTag := ""
			if adr.Track != "" {
				trackTag = fmt.Sprintf("[%s] ", strings.ToUpper(adr.Track))
			}
			fmt.Printf("  %s  • %-22s %-30s %s\n",
				amberStyle.Render("│"),
				blueStyle.Render(trackTag+adr.RawID),
				whiteStyle.Render(truncate(adr.Title, 30)),
				dimStyle.Render(adr.LastCommit),
			)
			fmt.Printf("  %s    %s %s\n",
				amberStyle.Render("│"),
				amberStyle.Render("NEXT:"),
				dimStyle.Render(truncate(adr.NextAction, 52)),
			)
		}
	}
	SectionFrameEnd("WIP-8842")

	// 4. Ready for Pod Hand-off: Accepted ADRs
	SectionFrameStart("HND-02", "READY FOR SQUAD HAND-OFF // ACCEPTED ADRS")
	if len(board.Handoffs) == 0 {
		if board.ADRUnknown {
			fmt.Printf("  %s  %s\n", amberStyle.Render("│"), redStyle.Render("UNKNOWN: the ADR sources could not be read."))
		} else {
			fmt.Printf("  %s  %s\n", amberStyle.Render("│"), dimStyle.Render("No pending ADR hand-offs."))
		}
	} else {
		for _, adr := range board.Handoffs {
			fmt.Printf("  %s  ✓ %-22s %s\n",
				amberStyle.Render("│"),
				blueStyle.Render(adr.DisplayName()),
				whiteStyle.Render(truncate(adr.Title, 38)),
			)
			if adr.IsDispatched() {
				fmt.Printf("  %s    %s %s\n",
					amberStyle.Render("│"),
					dimStyle.Render("DISPATCHED:"),
					dimStyle.Render(truncate(adr.Dispatched, 48)),
				)
				continue
			}
			fmt.Printf("  %s    %s %s\n",
				amberStyle.Render("│"),
				dimStyle.Render("TASK:"),
				dimStyle.Render(truncate(adr.NextAction, 52)),
			)
		}
	}
	if board.DispatchedHidden > 0 {
		fmt.Printf("  %s  %s\n", amberStyle.Render("│"),
			dimStyle.Render(fmt.Sprintf("%d dispatched ADRs not listed (--all-handoffs)", board.DispatchedHidden)))
	}
	SectionFrameEnd("REL-3109")

	// 5. Asynchronous Off-Desk Audio Queue
	if len(board.AudioQueue) > 0 {
		SectionFrameStart("AUD-03", "OFF-DESK COMMS & SENSOR FEED // AUDIO QUEUE")
		maxDisplay := 3
		if len(board.AudioQueue) < maxDisplay {
			maxDisplay = len(board.AudioQueue)
		}

		for i := 0; i < maxDisplay; i++ {
			talk := board.AudioQueue[i]
			fmt.Printf("  %s  %02d. %-12s %s\n",
				amberStyle.Render("│"),
				i+1,
				blueStyle.Render("["+talk.ID+"]"),
				whiteStyle.Render(truncate(talk.Title, 44)),
			)
			fmt.Printf("  %s      %s %s\n",
				amberStyle.Render("│"),
				dimStyle.Render("LINK:"),
				dimStyle.Render(talk.URL),
			)
		}

		if len(board.AudioQueue) > maxDisplay {
			fmt.Printf("  %s      %s\n",
				amberStyle.Render("│"),
				dimStyle.Render(fmt.Sprintf("... plus %d more queued in ARCH_DISPATCH.md", len(board.AudioQueue)-maxDisplay)),
			)
		}
		SectionFrameEnd("FRQ-9014")
	}

	// 6. Tactical Telemetry & WIP Status Footer
	fmt.Printf("\n %s\n", amberStyle.Render(strings.Repeat("━", 67)))
	wipStatus := fmt.Sprintf("%d/%d SLOTS ENGAGED", len(board.ActiveADRs), board.WIPLimit)

	if board.ADRUnknown {
		fmt.Printf(" %s  %s\n",
			pillAlert.Render("CAPACITY UNKNOWN"),
			redStyle.Render("the ADR sources could not be read, so WIP cannot be stated"),
		)
	} else if board.HasWIPViolation() {
		fmt.Printf(" %s  %s\n",
			pillAlert.Render("WIP SATURATED"),
			redStyle.Render(fmt.Sprintf("CRITICAL: %d active fronts exceeds capacity limit of %d", len(board.ActiveADRs), board.WIPLimit)),
		)
	} else {
		fmt.Printf(" %s  %s %s  %s %s\n",
			pillStatus.Render("CAPACITY NOMINAL"),
			dimStyle.Render("LOAD:"),
			whiteStyle.Render(wipStatus),
			dimStyle.Render("OUTPUT:"),
			blueStyle.Render(outputPath),
		)
	}
	fmt.Println()

	if !dryRun {
		Field("Dispatch File", outputPath)
	}
	fmt.Println()
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
