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
| SG-009 | Read the ADR candidate backlog | P2 | M | open | SG-008 | the candidates collector |
| SG-010 | README and config drift | P3 | S | open | | |
| SG-011 | Tests for the parts that decide things | P2 | M | done | | |
| SG-012 | Launch day count truncates | P3 | S | open | | the one-day disagreement |
| SG-013 | The ADR parser guesses instead of refusing | P3 | S | open | | |
| SG-014 | The audio queue is never enabled from the CLI | P2 | S | open | | |
| SG-015 | Fronts: non-ADR work with its own limit | P2 | M | done | | |

## Suggested order

1. SG-001 and SG-002 first. Together they remove the failure I hit by running from the wrong directory, and both are small.
2. SG-011 alongside SG-003. Write the matcher tests first; SG-003 is then a failing test turned green.
3. SG-004, then SG-005. The JSON schema should carry the dispatched state, so decide SG-005's storage question before freezing the schema.
4. SG-008 and SG-007 are done; SG-009 remains. They are all the same question: what counts as work in flight. Decide once.
5. SG-006, SG-010, SG-012 whenever there is a spare half hour.

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

Pri P2 | Size M | Status open | Needs SG-008

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

Pri P3 | Size S | Status open

**Problem.**
- The README usage examples showed `stargazer sync --dry-run` (removed from the README 2026-11-09; the flag is still unimplemented). The command has no such flag: `syncDryRun` is declared in `internal/cli/sync.go` and passed to the engine, but never bound with `Flags().BoolVar`. **ran**: `unknown flag: --dry-run`.
- README and the example config documented `signals.file` and `signals.url` (fixed 2026-11-09). The config struct has `signals.source`, `filter_rings` and `filter_tags` only, so those keys are ignored. **read**: `internal/config/base.go`; the flag binds `signals.source`.
- The example config file was named `.stagazer.example.yaml` (missing `r`). Fixed 2026-11-09, together with the example config and roadmap, which now use `signals.source` and agree on the `2-product` / `3-platform` namespaces.

**Proposal.** Register the flag. The config key, file name and namespaces are already fixed.

**Done when.**
- [ ] `stargazer sync --dry-run` works and writes nothing.
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

Pri P2 | Size S | Status open

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
