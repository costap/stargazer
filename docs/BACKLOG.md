# Stargazer backlog

Written 2026-11-09 while wiring a daily-summary wrapper script to Stargazer. Every item was checked against the working tree on that date. This repo has no commits yet, so there is no revision to cite; re-verify before trusting an item that has aged.

## How to use this file

- The index table is the whole picture. Pick from it, then read only that item.
- Each item is written to be handed to an agent as is: Problem, Proposal, Done when. Respect `AGENTS.md`: `internal/domain` stays pure Go, and never invent ADR numbers or dependency edges.
- `Status` is one of `open`, `ready` (decisions made, can start), `doing`, `done`, `dropped`. Change it in the table and in the item.
- `Size`: S under an hour, M half a day, L a day or more. `Pri`: P1 means the board can mislead you today, P2 means it works but costs you effort, P3 is polish.
- "Frees in the wrapper" names what the wrapper script deletes once the item ships. That is the payoff of doing it, because the wrapper currently works around each of these.
- Evidence labels: **ran** means I executed it, **read** means I read the code and did not run it.

## Index

| ID | Title | Pri | Size | Status | Needs | Frees in the wrapper |
|---|---|---|---|---|---|---|
| SG-001 | Failures must be loud | P1 | S | done | | the stderr scraping |
| SG-002 | Resolve paths against the config file | P1 | S | done | | the generated absolute-path config |
| SG-003 | A bare ADR ref must not bind ambiguously | P1 | S | done | | nothing, but the board is wrong without it |
| SG-004 | Machine-readable output (`--json`) | P1 | M | done | | the markdown parser |
| SG-005 | A dispatched state for Accepted ADRs | P1 | M | done | | the 45-day and story-citation filter |
| SG-006 | An unset Next Action is not a default | P2 | S | open | | the default-action list |
| SG-007 | Parked state for blocked ADRs | P2 | M | done | | |
| SG-008 | One definition of WIP | P2 | S | done | | |
| SG-009 | Read the ADR candidate backlog | P2 | M | done | | the candidates collector |
| SG-010 | README and config drift | P3 | S | done | | |
| SG-011 | Tests for the parts that decide things | P2 | M | done | | |
| SG-012 | Launch day count truncates | P3 | S | open | | the one-day disagreement |
| SG-013 | The ADR parser guesses instead of refusing | P3 | S | open | | |
| SG-014 | The audio queue is never enabled from the CLI | P2 | S | done | | |
| SG-015 | Fronts: non-ADR work with its own limit | P2 | M | done | | |
| SG-016 | Keep a YouTube playlist in sync with the audio queue | P2 | L | doing (P1-P4 done; live run worked, phone check open) | SG-014 | the manual link copying |
| SG-017 | Reclaim check: active items with no Next Action | P2 | S | open | SG-006 | |
| SG-018 | `stargazer lint`: ADR structure and supersession integrity | P2 | M | open | SG-013 | |
| SG-019 | Read-only MCP server for the board | P3 | M | open | SG-004 | |
| SG-020 | Suggest a signal for an open decision (keyword overlap) | P3 | S | open | | |
| SG-021 | A shareable view of capacity for managers and product | P2 | M | open | SG-004 | being asked "do you have room?" |
| SG-022 | Clean the roadmap: missing decisions, and decisions that are not mine | P1 | M | open | | chasing others for decisions |

## Suggested order

1. SG-001 and SG-002 first. Together they remove the failure I hit by running from the wrong directory, and both are small.
2. SG-011 alongside SG-003. Write the matcher tests first; SG-003 is then a failing test turned green.
3. SG-004, then SG-005. The JSON schema should carry the dispatched state, so decide SG-005's storage question before freezing the schema.
4. SG-008, SG-007 and SG-009 are done. They are all the same question: what counts as work in flight. Decide once.
5. SG-006 then SG-017 (the end-of-day "is my state written down" check), SG-012 and SG-013 whenever there is a spare half hour. SG-010 and SG-014 are done.
6. SG-016 phase 5 (the live playlist run) as soon as the Google client file exists.
7. SG-018 after SG-013: both are about refusing to guess what an ADR file says.
8. SG-019 and SG-020 only if the board is being asked for by agents or the signal matching is being done by hand often enough to hurt. See "Ideas reviewed".

---

## SG-001 Failures must be loud

Pri P1 | Size S | Status done 2026-11-09

**Problem.** When Stargazer cannot read the ADR folders it prints one `Warning scanning [...]` line on stderr, exits 0, and renders a calm board: `CAPACITY NOMINAL  LOAD: 0/3 SLOTS ENGAGED`. A board that says capacity is nominal because it read nothing is worse than no board. Same shape for a roadmap that fails to load (`Notice: could not load roadmap file`).
Evidence: **ran**. Running `stargazer sync -c cfg.yaml` from a directory where the relative ADR paths do not resolve gave exit 0 and the nominal board, while the same config from the repo root gave `WIP SATURATED, 6 active fronts`. **read**: `internal/app/engine.go` step 1 and `internal/scanner/adr.go` `ScanAllADRGroups`, which `continue`s past a failed group and returns a nil error.

**Proposal.**
- `ScanAllADRGroups` returns a non-nil error when any configured group fails, or wraps them with `errors.Join`.
- The engine decides policy: any ADR group failing is fatal by default, with `--keep-going` to render a degraded board.
- A degraded board starts with a banner (`DEGRADED: 3-platform not read`) in both the markdown and the LCARS output, and the capacity line is replaced by `UNKNOWN`.
- Zero ADRs found across all groups is itself a warning.

**Done when.**
- [x] An unreadable ADR directory makes `sync` exit non-zero.
- [x] With `--keep-going`, the markdown starts with a `DEGRADED` banner and the capacity line says unknown, never nominal.
- [x] A roadmap that fails to load is treated the same way.
- [x] A test covers a missing directory.

**Open question.** Should a missing roadmap be fatal? It is optional in the config, so probably only when `roadmap.file` was set explicitly.

**Outcome.** Done as proposed, with these decisions and findings:
- A missing roadmap is fatal only when explicit (set in the config file, a flag or the environment, tracked as `Roadmap.Explicit`). A missing roadmap at the default path stays a notice. A roadmap that exists but fails to parse is always a warning.
- `cmd/stargazer/main.go` discarded the error from `cli.Execute`, so the process exited 0 on every failure of every command, not only this one. That was half of the original symptom and it is fixed: failures now exit 1.
- **Degraded boards never overwrite `ARCH_DISPATCH.md` when the ADRs were not read.** This was not in the proposal and it matters. Your `Next Action` notes live only in that file; a board with no pipeline would erase them on the next sync. A degraded board that did read the ADRs (for example a bad roadmap) is written, with the banner.
- `--keep-going` exits 0. The warnings still go to stderr and the board carries the banner, so it stays loud without breaking a script that opted in.
- `domain.DispatchBoard` gained `Warnings` and `ADRUnknown`, which SG-004's JSON should expose.
- A partial read (one ADR group readable, one not) counts as unknown, because a WIP computed from half the pipeline is a wrong number.
- Tests: scanner, engine and config. I mutation-checked them by re-introducing the swallowed error, the unresolved paths and the board overwrite; each mutation fails a test.
- Not done: nothing tests `main`'s exit code or the renderers' degraded output directly; the engine tests read the written markdown, but the LCARS banner is only checked by hand.

---

## SG-002 Resolve paths against the config file

Pri P1 | Size S | Status done 2026-11-09

**Problem.** Relative `adr.groups[].dir`, `roadmap.file` and `dispatch.output` resolve against the process cwd. That is how SG-001 gets triggered, and it means the same `-c` file behaves differently depending on where you stand. The shipped example config uses relative paths, so it invites the mistake.
Evidence: **ran** (see SG-001). **read**: `internal/config/base.go` does no path handling.

**Proposal.** After `ReadInConfig`, resolve every relative path from the config file against the config file's own directory. Paths given as CLI flags stay relative to cwd, which is what a shell user expects. When no config file was found, cwd is correct.

**Done when.**
- [x] The same config file produces the same board from any cwd.
- [x] A flag such as `--roadmap x.yaml` still resolves against cwd.
- [x] A test covers both.

**Outcome.** Done in `config.Load`. A path is resolved against the config file's directory only if that file itself sets it and nothing with higher precedence (a flag, passed as `Load(v, overriddenKeys...)`, or a `STARGAZER_` environment variable) overrode it. Two cases the proposal did not mention:
- A key the config file omits is not resolved, so a config under `$HOME/.config/stargazer` does not drag the default `roadmap.yaml` in there.
- `signals.source` is left alone when it is a URL.

**Behaviour change.** A config file whose relative paths were written to be read from a different directory will now resolve elsewhere. The one config I know of, the wrapper's generated file, uses absolute paths and is unaffected.

---

## SG-003 A bare ADR ref must not bind ambiguously

Pri P1 | Size S | Status done 2026-11-09

**Problem.** `AGENTS.md` section 3B says an unqualified reference such as `0099` or `ADR-0099` "binds if unambiguous across tracks". The code does not check. `ADR.Matches` returns true for any ADR with that number, and `correlateRoadmapWithADRs` takes the first match, which is whichever track sorts first. The two real tracks reuse numbers, so a bare number silently binds to the wrong decision and tags it with a phase.
Evidence: **ran**. A roadmap with `ref: "0031"` tagged `2-product/0031` (the Accepted message-broker decision) as `[phase-1-launch]` and left `3-platform/0031` (Security scanning in CI, Proposed) untagged. **read**: `internal/domain/adr.go` `Matches`, and the loop with `break` in `internal/app/engine.go`.

**Proposal.** In `correlateRoadmapWithADRs`, collect all matches for a ref. Exactly one: bind. More than one: leave unresolved and print a notice naming both candidates. Zero: print a notice too. Keep `Matches` pure; the ambiguity decision belongs to the caller.

**Done when.**
- [x] A bare number that exists in both tracks binds to nothing and prints a notice listing both.
- [x] A bare number that exists in one track binds to it.
- [x] `2-product/0031` and `product/0031` still bind exactly.
- [x] Table-driven tests cover `0031`, `31`, `ADR-0031`, and a number present in only one track.

**Outcome.** `correlateRoadmapWithADRs` collects every match and returns notices; the engine prints each as a `Notice:` line on stderr. One match binds. Several (a bare number in both tracks) or none bind nothing, and the notice names the phase, the ref and, for the ambiguous case, both qualified ids. `Matches` is unchanged and still pure; its tests pin that a bare number matches in every track that has it. The real `roadmap.yaml` produces no notices (every ref is track-qualified).
Decision to check: these are notices, not warnings, so they do not degrade the board or fail the run. But the wrapper treats any `Notice` line on stderr as failure, so an ambiguous ref in `roadmap.yaml` would make it report `stargazer unavailable`. That is loud but harsher than the fault deserves; narrowing the wrapper's check to `Warning` and `Error` is a one-line change if you prefer.

---

## SG-004 Machine-readable output (`--json`)

Pri P1 | Size M | Status done 2026-11-09

**Problem.** The only outputs are an LCARS terminal board and a markdown file, so every consumer has to scrape markdown. The wrapper does this with regexes keyed on emoji headings, `### [` and `- [x] **[`. A cosmetic change to a heading breaks it silently.

**Proposal.** `stargazer sync --json[=path]` writes a versioned document, built from the same `DispatchBoard` the renderers use.

```json
{
  "schema": 1,
  "synced_at": "2026-11-09T11:22:14Z",
  "wip": { "limit": 3, "count": 6, "over_by": 3 },
  "launch": { "date": "2026-09-15", "days": 99 },
  "active": [
    { "id": "3-platform/0031", "title": "...", "file": "0031-....md",
      "date": "YYYY-MM-DD", "status_line": "Proposed", "blocker": "",
      "phase": "phase-1-mvp", "next_action": "", "next_action_set": false }
  ],
  "handoffs": [ { "id": "...", "date": "...", "phase": "", "dispatched": false } ],
  "gaps": [ { "phase": "phase-1-mvp", "kind": "decision", "gist": "...", "blocks": [] } ],
  "warnings": []
}
```

Include the ADR `date` and the raw status line: the wrapper currently re-reads each ADR file to get both.

**Done when.**
- [x] `--json` produces the document above and nothing else on stdout.
- [x] `schema` is bumped on any breaking change.
- [x] Warnings from SG-001 appear in `warnings` as well as on stderr.
- [x] A golden-file test pins the shape.

**Outcome.** `internal/render/json.go` builds the document from the board the renderers use. Decisions: `--json` with no value writes to stdout and suppresses the terminal board; `--json=path` writes a file; the markdown board is rewritten as usual in both cases. A degraded board (`--keep-going`) still emits JSON, carrying `adr_unknown` and `warnings`, so a consumer can tell it from a calm one. List fields are `[]`, never `null`.
Changes to the proposed shape: `blocker` was dropped (it belongs to SG-007, which is undecided), `status` was added next to `status_line`, `adr_unknown` and `dispatched_hidden` were added, and `next_action_set` is false for the generated default actions even after an earlier sync wrote them into the board, which also settles most of SG-006 for JSON consumers. `launch.days` still has the SG-012 truncation. A golden file (`internal/render/testdata/board.golden.json`) pins the shape.
Found on the way: `--wip-limit N` set the board's limit to 0. `syncWIPLimit` in `internal/cli/sync.go` was declared but never bound to the flag, and the command copied it over the configured limit whenever the flag was given. Fixed by dropping the override (Viper already binds the flag), with a test that drives the real command line.

---

## SG-005 A dispatched state for Accepted ADRs

Pri P1 | Size M | Status done 2026-11-09

**Problem.** The hand-off section lists every Accepted ADR forever. There is no way to say "stories were generated". On 2026-11-09 it held 130 rows against 131 Accepted ADRs on disk, so the section carries no signal. It is meant to answer "what should I turn into stories this afternoon".
Evidence: **ran** (130 rows counted in the written board). **read**: `buildDispatchBoard` appends every `StatusAccepted` ADR to `Handoffs`.

**Proposal.** Give an Accepted ADR a dispatched state, and show only undispatched ones by default.

**Open question (decide before building).** Where does the state live?
- A. In the board markdown, as a Next Action keyword (`dispatched` or `dispatched: PROJ-5301`). Already human-editable and already preserved. Fragile, and a free-text field becomes a flag.
- B. A sidecar `.stargazer-state.yaml` keyed by qualified id. Explicit and diffable, one more file.
- C. Derived: an Accepted ADR counts as dispatched when a story file mentions it. Needs a stories directory and has the cross-track ambiguity from SG-003.
My lean is B for the flag and C only as a hint.

**Done when.**
- [x] The default board lists only Accepted ADRs not marked dispatched.
- [x] `--all-handoffs` shows every one.
- [x] Marking one dispatched removes it on the next sync and survives later syncs.
- [x] The JSON carries `dispatched`.

**Outcome.** Option B, chosen 2026-11-09. `state.file` (default `.stargazer-state.yaml`) holds `dispatched: {<qualified id>: <reference>}`. Stargazer only reads it. Keys match the track-qualified id exactly, case-insensitively; a bare number is never accepted, for the SG-003 reason. An entry matching no Accepted ADR prints a `Notice:`. A missing file at the default path is normal; a named one that is missing, or one that does not parse, is a warning like any other unreadable input. Dispatched ADRs are left out of the hand-off section and the board says how many (`dispatched_hidden`); `--all-handoffs` lists them ticked with their reference.
Not done: there is no `stargazer dispatch <id>` command, so marking an ADR means editing the YAML by hand. And on first use every Accepted ADR is undispatched (131 in the real repo), so a baseline is needed; see the open questions in the hand-over note below.
A side effect to know: an ADR that drops out of the hand-off section takes its hand-written Next Action with it, because the board is regenerated from what it lists.

---

## SG-006 An unset Next Action is not a default

Pri P2 | Size S | Status open

**Problem.** Every Proposed ADR gets `Review RFC comments; drive consensus with lead engineers` and every Accepted one gets `Run story-gen skill; review Gherkin contracts with pod PM`. They look like real notes, so the board cannot show which ADRs you have actually planned. Worse, the text is written into the markdown and read back by `ParseExistingDispatch` as a preserved note, so the default becomes sticky: changing it in code never changes an existing board.
Evidence: **read**: `applyPreservedActions` in `internal/app/engine.go` and `ParseExistingDispatch` in `internal/render/markdown.go`. **ran**: the board showed the identical line on all six Proposed ADRs.

**Proposal.** Default to empty. Render an unset action as `-` in markdown and as `unplanned` in LCARS. On load, treat the two old default strings as unset, so existing boards migrate. Expose `next_action_set` in the JSON.

**Done when.**
- [ ] A fresh board shows no invented action text.
- [ ] A board containing the old default strings reads them as unset.
- [ ] A hand-written action still survives a sync.
- [ ] The LCARS board makes unplanned ADRs visible.

---

## SG-007 Parked state for blocked ADRs

Pri P2 | Size M | Status done 2026-11-09

**Outcome.** Parking is explicit, not inferred: a `parked:` map in the state file (`id: reason`), Proposed ADRs only. Inferring it from the Status suffix was dropped, because a suffix says nothing about whether the ADR still costs attention (the open question below), and that is the owner's call. Parked ADRs have their own section, keep their hand-written Next Action, and are excluded from WIP; the JSON carries `parked` and `wip.parked`. No `adr.park_blocked` setting was needed.

**Problem.** Every Proposed ADR counts toward WIP, including ones that cannot move. Product ADR-0016 has been waiting on a proof of concept for months and takes a slot permanently, so the limit of three is breached by something you cannot act on. The ADR already says so in its Status line: `Proposed - pending PoC execution and load-test validation`.

**Proposal.** Treat text after `Proposed` in the Status line as a blocker. A blocked ADR is `parked`: listed in its own section, not counted toward the limit. Make that behaviour a setting (`adr.park_blocked`, default off) until you have decided SG-008.

**Done when.**
- [ ] A Status line with a suffix after `Proposed` marks the ADR parked when the setting is on.
- [ ] Parked ADRs appear in a separate section with the blocker text.
- [ ] WIP excludes them, and the JSON reports both counts.

**Open question.** Is a parked ADR really out of your head? If it still costs attention, leave this off.

---

## SG-008 One definition of WIP

Pri P2 | Size S | Status done 2026-11-09

**Outcome.** Decided: WIP is the Proposed ADRs that are not parked, and nothing else (not `owner: me` needs, no second limit). The sentence is in `AGENTS.md`, the README and `DispatchBoard`, and the capacity footer says how many are parked.

**Problem.** The docs disagree. `AGENTS.md` section 1.4 says the active pipeline is `Status: Proposed` with a limit of 3. Section 3A says "Only [`owner: me` needs] count toward active cognitive WIP". The code counts Proposed ADRs only (`buildDispatchBoard`), and the capacity message says "6 active fronts".

**Proposal.** Decide the definition, then make `AGENTS.md`, the README and the capacity message say it. Options: Proposed ADRs only (current behaviour); Proposed ADRs plus in-flight `owner: me` decisions with no ADR yet; or two limits.

**Done when.**
- [ ] One sentence defines WIP and appears in `AGENTS.md`, the README and the code comment.
- [ ] The capacity message uses the same words.

---

## SG-009 Read the ADR candidate backlog

Pri P2 | Size M | Status done 2026-11-09

**Outcome.** `candidates.dir` (optional, resolved against the config file). `ScanCandidates` returns the open files oldest first, ignores `README.md` and non-numbered files, and names a numbered file with no Status field in a warning. The board and JSON carry the queue and a gate (`CandidatesGated`: no free slot, or unknown WIP). The gated section lists titles with no call to action. The `/captains-log` collector still reads the candidates folder itself; switching it to the JSON `candidates` is part of the wider move onto `--json`.

**Problem.** `AGENTS.md` invariant 4 says any workflow that suggests new ADR candidates must warn when WIP is saturated, but Stargazer never sees candidates. The queue that the WIP limit exists to gate lives elsewhere, so the gating is done by hand or by the wrapper. A decision you do not draft yet and the ADR you are working on are the same pipeline.

**Proposal.** Optional `candidates.dir` of `NNN-*.md` files with a `Status: open` line. Render an ordered section, shown as gated when WIP is full (`3 of 3 slots used, 8 candidates wait`). Never render a "start this" call to action when the pipeline is saturated.

**Done when.**
- [ ] With `candidates.dir` unset, nothing changes.
- [ ] With it set, open candidates are listed and counted, promoted and dropped ones are not.
- [ ] A full pipeline shows the gate and no suggestion.
- [ ] The JSON carries the candidates and the gate state.

**Note.** In this setup the candidates folder is git-ignored and local to the architecture repository, so the path must be configurable and absence must be fine.

---

## SG-010 README and config drift

Pri P3 | Size S | Status done 2026-11-09

**Outcome.** `--dry-run` is bound: it reconciles and reports, and writes neither the markdown board nor a `--json=path` file (a `Notice:` says so). `--json` on stdout still prints. The terminal summary is labelled as a dry run. A CLI test checks that nothing is written with the flag and that the same command line writes the board without it.

**Problem.**
- The README usage examples showed `stargazer sync --dry-run` (removed from the README 2026-11-09; the flag is still unimplemented). The command has no such flag: `syncDryRun` is declared in `internal/cli/sync.go` and passed to the engine, but never bound with `Flags().BoolVar`. **ran**: `unknown flag: --dry-run`.
- README and the example config documented `signals.file` and `signals.url` (fixed 2026-11-09). The config struct has `signals.source`, `filter_rings` and `filter_tags` only, so those keys are ignored. **read**: `internal/config/base.go`; the flag binds `signals.source`.
- The example config file was named `.stagazer.example.yaml` (missing `r`). Fixed 2026-11-09, together with the example config and roadmap, which now use `signals.source` and agree on the `2-product` / `3-platform` namespaces.

**Proposal.** Register the flag. The config key, file name and namespaces are already fixed.

**Done when.**
- [x] `stargazer sync --dry-run` works and writes nothing.
- [x] Every key in the README config block is read by the config struct.
- [x] The example config and example roadmap resolve against each other.

---

## SG-011 Tests for the parts that decide things

Pri P2 | Size M | Status done 2026-11-09 (the `DaysUntilLaunch` bullet moves to SG-012)

**Problem.** `go test ./...` reports `[no test files]` for every package, yet `AGENTS.md` tells agents to run it and the matcher carries real invariants.

**Proposal.** Table-driven tests where a wrong answer misleads you:
- `ADR.Matches` and the ambiguity rule (SG-003).
- Status parsing: next line after `## Status`, with suffixes like `Proposed - pending PoC`.
- `ParseExistingDispatch` round trip: a hand-written Next Action survives a sync (the stated non-destructive invariant).
- `DaysUntilLaunch` (SG-012).
- A scanner test over a temp directory with two tracks sharing a number.

**Done when.**
- [x] Each bullet has at least one failing-then-passing test, except `DaysUntilLaunch`: it calls `time.Until` with no injectable clock, so a test now would either be flaky around midnight or assert the bug. SG-012 owns the clock seam and its test.
- [x] `go test ./...` runs real tests.
- [x] The Makefile has a `test` target that does.

**Outcome.** New tests: `domain/adr_test.go` (17-row `Matches` table), `app/correlate_test.go` (10-row binding table, notices, ignored needs), `scanner/status_test.go` (status suffixes, status confined to its section, identity from filename and title, two tracks sharing a number), and `TestHandWrittenNextActionSurvivesASync` in `app/engine_test.go`. Mutation-checked: dropping `applyPreservedActions`, reading status from any section, and first-match-wins binding each fail the matching test. The Makefile `test` target already ran `go test -v ./...`; it now runs real tests. No parser bug turned up; the status tests passed on the first run.

---

## SG-012 Launch day count truncates

Pri P3 | Size S | Status open

**Problem.** `DaysUntilLaunch` is `int(time.Until(launchMidnightUTC).Hours() / 24)`. At 11:22 local on 2026-11-09 it gives 98 where the calendar difference to 2026-09-15 is 99, and the value changes depending on the time of day and the time zone. Another tool counting calendar days (the wrapper does) disagrees by one.
Evidence: **ran** (board showed `L-98`, the wrapper showed 99). **read**: `internal/domain/roadmap.go`.

**Proposal.** Compare calendar dates in the local zone: `launchDate.Sub(today).Hours()/24` with both truncated to midnight. Decide what a past date shows (`L+3`).

**Done when.**
- [ ] The count is the same at 00:01 and 23:59.
- [ ] A launch date in the past does not render a negative `L-` value.
- [ ] A test pins both.

---

## SG-013 The ADR parser guesses instead of refusing

Pri P3 | Size S | Status open

**Problem.** Found while writing the SG-011 tests. `ParseADRFile` defaults a file with no recognised `## Status` to `Proposed`, and a filename with no digits to number `0`, so a stray `README.md` or `template.md` in an ADR folder would appear as `Proposed` ADR 0000 and count against the WIP limit. Separately, `ScanADRDir` `continue`s past a file that fails to parse, so an ADR the scanner cannot read disappears from the count with no message, the same shape as SG-001 at file level.
Evidence: **read**: `internal/scanner/adr.go` `ParseADRFile` and `ScanADRDir`. **ran**: both real ADR folders hold 96 and 44 files, all numbered and all with a `## Status`, so nothing is wrong today.

**Proposal.** Skip files whose name has no leading number, with no notice (they are not ADRs). For a numbered file with no recognised status, report it as a warning instead of calling it Proposed. Report a file that fails to parse through the same joined-error path as SG-001.

**Done when.**
- [ ] A `README.md` in an ADR folder is not counted.
- [ ] A numbered file with no `## Status` is named in a warning and not counted as Proposed.
- [ ] A file that cannot be read is named in a warning.

---

## SG-014 The audio queue is never enabled from the CLI

Pri P2 | Size S | Status done 2026-11-09

**Outcome.** The queue is on by default; `--no-audio` leaves it out. `extractAudioQueue` is tested over one signal, a non-video link, a repeated URL, a short `youtu.be` link and digest links. The test found a real bug: digest links were never extracted, because the parser required `[http` and the real digests write `**[Title](url)**`. A single link regex now handles both. Run against the real signals, the queue holds the YouTube talks from signals and digests. The playlist half of the idea is SG-016.

**Problem.** Found while fixing the `--wip-limit` bug in SG-004. `syncCmd` passes `IncludeAudioQueue: syncAudioQueue`, a package variable that is never bound to a flag and is always false. Only `Engine.Sync` (which no command calls) sets it to true. So the README's headline "Off-Desk Audio Queue" feature does not run from `stargazer sync`; the extraction code exists and is untested.
Evidence: **read**: `internal/cli/sync.go`, `internal/app/engine.go`. Not run end to end.

**Proposal.** Decide the default (on, like `Sync`), bind a `--no-audio` flag or a config key, and cover `extractAudioQueue` with tests over a small signals fixture. The same applies to `syncDryRun`, which SG-010 already covers.

**Done when.**
- [ ] `stargazer sync` with a signals source produces the audio queue section.
- [ ] A test pins the extraction for one signal and one digest, including de-duplication.

---

## SG-015 Fronts: non-ADR work with its own limit

Pri P2 | Size M | Status done 2026-11-09

**Problem.** SG-008 defined WIP as Proposed ADRs only, so work that needs no ADR (operator and brand provisioning, ops go-live preparation) took deep blocks and cost nothing on the board.

**Outcome.** A `fronts:` map in the state file (`id: {title, next, ref}`), a separate limit (`fronts.limit`, default 2), its own board section, `fronts` and `wip.fronts*` in the JSON. Two limits, never summed: this is SG-008's third option, taken after the first two left real work invisible. Fronts are hand-maintained; nothing is inferred from PRs or the readiness register.

---

## SG-016 Keep a YouTube playlist in sync with the audio queue

Pri P2 | Size L | Status doing | Needs SG-014

**Progress 2026-11-09.** Phases 1, 2 and 4 are built and tested without a Google account: the pure planner (`domain.PlanPlaylist`, `VideoID`), `heard:` in the state file, the desired-list builder, the API client (tested against an in-memory fake), the token refresh and save, and `stargazer playlist sync [--dry-run]` with its playlist record. Mutation checks on the planner, the heard filter, the status filter, dry-run and the private-playlist setting each failed a test. Against the real signals a dry run lists 25 adds and writes nothing. Phase 3 followed the same day: `playlist login` (loopback on 127.0.0.1, PKCE S256, random state, offline access, a refresh token required), tested against a fake token endpoint that checks the code verifier against the challenge; mutation checks on the state check, PKCE, the refresh-token requirement and the loopback bind each failed a test. V'Ger has no OAuth flow to copy (it uses a read-only API key). Phase 5, first live run (2026-11-09): the playlist was created and filled. It stopped on a video YouTube would not add (`videoNotFound`), which showed the sync must not die on one dead link. Fixed: an add that YouTube refuses as unavailable is recorded under `unavailable` in the playlist record, the plan is made again from the playlist as it stands, and the next talk takes the slot (bounded at 10 replans a run; quota and login errors still fail loudly; a recorded video is never retried). Four of the 29 queued links were dead, all from the `argoconAmsShorts2026` digest; YouTube's own oEmbed also answers 404 for them, so the digest links are wrong upstream, not a Stargazer fault. A second bug the live run exposed: the record's default path followed the working directory, so running from another directory would have created a second playlist; it now defaults beside the config file. Open: confirm on the phone, and the `heard:` round trip.

**Problem.** The audio queue is a list of links in a markdown file. Using it away from the desk means copying links into the YouTube app by hand.

**Proposal.** `stargazer playlist sync` keeps a private playlist that Stargazer owns in step with the queue: capped, newest first, minus talks you mark `heard:`. Full plan, decisions, phases and the one-time Google Cloud setup: `docs/plans/SG-016-youtube-playlist.md`.

**Done when.**
- [x] `stargazer playlist login` stores a token outside the repo. (built and tested against a fake; live check is phase 5)
- [x] `stargazer playlist sync --dry-run` prints the adds, removes and moves and changes nothing.
- [x] `stargazer playlist sync` creates the playlist on first run and is a no-op when nothing changed. (against a fake; live check is phase 5)
- [x] A talk listed under `heard:` leaves the playlist on the next sync.
- [x] The planner is table-tested, and the client is tested against a fake server.
- [x] No secret, token or playlist id is committed.

**Open question.** Background playback on a phone needs YouTube Premium. Confirm it before building the login step.

---

## Ideas reviewed 2026-11-09

A list of possible extensions was checked against what Stargazer is for and what it already does. Stargazer is deterministic: `AGENTS.md` forbids inventing ADR numbers, dates, squads or edges, `internal/domain` is pure Go, and a failed read is loud. An idea that needs a model to judge something does not fit inside it; it belongs in a skill or in V'Ger, with Stargazer supplying the data.

| Idea | Verdict | Why |
|---|---|---|
| Two-phase day (deep block, batch coordination) | Already done | The daily timetable is in the captain's operating manual and the captain's log (DEEP, COORD, DISPATCH). One correction: the idea assumed a single active `owner: me` item, but the rule is three Proposed ADRs plus two fronts (SG-008, SG-015) |
| End-of-day reclaim ritual | Build the check, SG-017 | The manual already has a reclaim slot. What is missing is a way to see whether the state is actually written down |
| Decision quarantine (park ad-hoc asks as `ref: null` decisions in `roadmap.yaml`) | Decline as written | `roadmap.yaml` is committed and records only the launch date and phase-to-decision links, so a stream of ad-hoc asks does not belong there. The ADR candidate registry already does this job, and SG-009 gates it |
| `stargazer draft-adr` (LLM drafts an ADR from a signal) | Decline here | It needs a model, so it breaks the determinism rule, and a drafted ADR "matching your topology" is exactly where invented facts come from. The `adr` skill drafts ADRs, and V'Ger owns signal enrichment. Stargazer's part is to expose the signal data (SG-019) |
| `stargazer mcp` with `get_active_pipeline` and `get_adr` | Maybe, SG-019 | Read-only and deterministic, so it fits. The board is already available as `sync --json`, so wait until agents actually need it |
| `check_compliance(adr_id)` over the MCP server | Decline | Judging whether a change complies with an ADR is a reasoning task. It stays a skill (`architecture-critique`) |
| `stargazer critique` (supersession, regulatory, operational review) | Split | The supersession and structure checks are mechanical and become SG-018. The regulatory and operational review is a judgement call; the `architecture-critique` and `architecture-counter-review` skills exist for it |
| Embedding-based signal matching to open decisions | Decline embeddings, SG-020 instead | There are about 16 signals and a handful of open decisions. An embedding service and a model dependency is far more than that needs. Plain keyword overlap, labelled as a suggestion, covers it |
| Obsidian and Dataview | No code | The ADR files carry no frontmatter (`Date:` is a plain line), so Dataview cannot query them as they are. The JSON from `sync --json` is the stable interface if a dashboard is wanted |
| Offline audio with the playlist command | Already done | SG-014 and SG-016. Background playback on the phone needs YouTube Premium |

---

## SG-017 Reclaim check: active items with no Next Action

Pri P2 | Size S | Status open | Needs SG-006

**Problem.** The point of the end-of-day reclaim is that nothing lives only in your head: each active ADR and each front has a written Next Action. Today nothing tells you which ones do not. After SG-006 an unset action is distinguishable from a written one, so it can be counted.

**Proposal.** The board and the JSON say how many active ADRs and fronts have no hand-written Next Action, naming them (`wip.unplanned`, a list under each). A `--check` flag on `sync` exits non-zero when any active item is unplanned, so it can run as the last step of the day. Parked and Accepted ADRs are not counted. Fronts use their own `next:` field.

**Done when.**
- [ ] The JSON lists the active ADRs and fronts with no hand-written Next Action.
- [ ] `sync --check` exits non-zero when there is one, and prints which.
- [ ] The captain's log shows the count on the DEEP line.
- [ ] A parked ADR with no action is not reported.

---

## SG-018 `stargazer lint`: ADR structure and supersession integrity

Pri P2 | Size M | Status open | Needs SG-013

**Problem.** Accepted ADRs are immutable, so a mistake in their structure or in the links between them is found late. The house rules are mechanical: the required sections, and the rule that when a new ADR partially overrides an Accepted one, the old ADR's Status carries a "Partially superseded by ADR-NNNN" note. Nothing checks them.

**Proposal.** A read-only `stargazer lint` over the configured ADR groups. It reports, never edits and never guesses: a missing required section (Title, Date, Status, Context, Decision, Consequences); an ADR that says it supersedes or partially supersedes another whose Status does not say so back; a "superseded by" that names an ADR that does not exist in that track; and an ADR number that appears twice in one track. References are track-qualified, because the two tracks reuse numbers. The judgement calls (does this ADR contradict that one, is the compliance reasoning sound) stay with the `architecture-critique` skill.

**Done when.**
- [ ] Each rule has a test over a small fixture, and a clean fixture reports nothing.
- [ ] A bare "ADR-0031" that exists in both tracks is reported as ambiguous, not resolved.
- [ ] The exit code is non-zero on any finding, and every finding names the file and line.
- [ ] Run on the real ADR folders, every finding is read before it is trusted.

---

## SG-019 Read-only MCP server for the board

Pri P3 | Size M | Status open | Needs SG-004

**Problem.** Agents in other sessions need the current WIP, the parked and front lists, an ADR's parsed status and the open candidates. Today they run `sync --json` or read files. An MCP server would answer the same questions without pasting ADRs into a prompt.

**Proposal.** `stargazer mcp`: a stdio server exposing read-only tools over the same document `sync --json` emits: the board (WIP, active, parked, fronts, candidates and gate), one ADR by track-qualified id (status line, date, sections), and the open candidates. No tool calls a model, none writes, and none judges compliance. The server runs a sync in dry-run mode per request, so it never rewrites `ARCH_DISPATCH.md`.

**Done when.**
- [ ] The tools answer from the JSON document, and a test pins their output.
- [ ] No tool writes a file; a test asserts the board file is unchanged after a call.
- [ ] A degraded board is reported as degraded, with its warnings, never as empty.

**Open question.** Is it needed? The captain's log and `sync --json` already cover the daily use. Build it when an agent session actually asks for the board and cannot get it.

---

## SG-020 Suggest a signal for an open decision (keyword overlap)

Pri P3 | Size S | Status open

**Problem.** Matching an open roadmap decision (a `ref: null` need) to the signal that addresses it is done by eye. With about five open decisions and sixteen signals, an embedding service is out of proportion, but a pointer would save the search.

**Proposal.** For each open decision in the in-flight phase, score the signals by shared stems between the decision's gist and the signal's title, category and why-captured text (the same word-stem approach the captain's log uses for candidates, three shared stems). Show up to two as `Suggested signal: 0015` under the decision in the board, and in the JSON as `suggested_signals`. Always labelled as a suggestion; nothing is written to `roadmap.yaml`.

**Done when.**
- [ ] A decision whose gist shares three stems with a signal shows that signal.
- [ ] A decision with no match shows nothing, not a weak guess.
- [ ] The suggestion is visibly a heuristic in every output.

## SG-021 A shareable view of capacity for managers and product

Pri P2 | Size M | Status open | Needs SG-004 (done)

**Problem.** Managers and product ask whether the architect has room, and what is in flight. The answer exists in the captain's log, but that is a personal view: it holds Slack items, local paths and next actions written for one reader. Every ad hoc question costs a context switch, which is what the log exists to remove.

**Proposal.** Generate a second, audience-safe view from the same `sync --json`, so there is one source and two renderings. It shows only what a requester needs: ADRs in flight against the limit of 3, the fronts (limit 2), what is parked and why, what was closed this week, and an honest "next free slot" line. It never shows Slack items, candidate titles from `analysis/`, readiness minutes or the daily timetable. It also says how to get work onto the queue: who to ask, and that new work waits for a free slot unless something is swapped out.

**Options.**
| Option | Fit |
|---|---|
| A page published from the board (a claude.ai artifact, or a markdown file in the committed docs) refreshed by `/loop` | Recommended. Read without asking; no new system. Needs a decision on where it is published |
| A weekly Slack post | Pushes rather than waits to be read, but Slack drafts are never sent from here, so it stays a manual step |
| A Jira board or dashboard | Duplicates the state in a second place that will drift |

**Open.** Who the audience is and which channel they already read. Whether the page may name ADR titles (they are committed, so yes) and whether the fronts may be named (they are hand-written, so check them).

**Done when.**
- [ ] `stargazer sync --json` renders a public view with a `--public` or `--audience` switch, with a test that no local-only field can reach it.
- [ ] The view states the limits and the current counts in plain words, and says when a request would have to swap something out.
- [ ] It is published somewhere the requesters already look, and refreshed on a schedule.
- [ ] The captain's log links to it, so the two cannot disagree.

## SG-022 Clean the roadmap: missing decisions, and decisions that are not mine

Pri P1 | Size M | Status open

**Problem.** `inventory/roadmap.yaml` was generated and is a good start, but two things are wrong with it. Decisions the architect believes matter for launch are missing. And some lower-level decisions are assigned to the architect that someone else has to make (a squad, a PO, platform). They sit in the WIP and DEEP lines as his, and he ends up chasing the real owner, which is the time cost he wants to remove. With launch at 2027-02-16 the file also drives the launch-phase ranking, so errors here skew the whole log.

**Proposal.** A one-off cleanup of the file, then a small structure change so it stays clean.
1. **Audit.** Walk every `needs` row and mark it keep, re-own or drop. A row stays only if the decision is architectural and the architect is the one who closes it.
2. **Re-own.** A decision someone else makes moves to them with a named owner and a "needed by" date, so the ask is made once, in writing, and not chased.
3. **Add the missing ones.** List what is important and absent, each with a gist, because the candidate registry cannot be cited from committed content.
4. **Structure.** Give each need a `decider` (who can close it) and a `needed_by`. Stargazer shows a need whose decider is not the architect under a separate "waiting on others" line, which does not count against WIP, the same idea as parked.
5. **Escalation, not chasing.** A need past its date becomes a line in the capacity view (SG-021), so the date is visible to the requester without a message.

The file is committed and a decision with the PO (its own header says so), so the content change goes through a normal change in the architecture repo. Only the `decider` and `needed_by` support belongs in Stargazer.

**Done when.**
- [ ] Every row in `roadmap.yaml` is kept, re-owned or removed, with the reason recorded in the change description.
- [ ] The missing decisions are added, each with a gist.
- [ ] Every remaining need has a `decider`; those not owned by the architect are listed as waiting on others and are not WIP.
- [ ] `stargazer sync` reports an unknown `decider` loudly (the SG-001 rule) and does not guess.
- [ ] The captain's log and the board agree on what the architect owes.

**Progress 2026-11-10.** The board now shows a short description for every roadmap need, and the owner with its reviewer (`OWNER: TEAM, REVIEW: ME`), including delegated rows, which used to show only a doc path. A need takes an optional `title:`, else the first sentence of its `gist`; an outcome shows its `id` instead of the doc path it is tracked in; a need with no ADR reads `(no ADR yet)`. Still open: group each outcome with its `for:` rows, the "waiting on others" line off WIP, and refusing an unknown `owner` loudly.
