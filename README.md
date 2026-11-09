# Stargazer (`USS Stargazer — NCC-2882`)

> **LCARS-styled Architectural Dispatch & Strategic Defragmentation Engine**

Stargazer helps an architect keep a limited number of decisions in flight. It bridges high-level product roadmaps, multi-track Architecture Decision Records (ADRs), and radar signals from [V'Ger](https://github.com/costap/vger) and [Astrometrics](https://github.com/costap/tech-signals) into one operational dispatch board.

```text
 ▄██████████████████████████████████████████████████████████████████████▄
 █ NCC-2882  STARGAZER DISPATCH INTERFACE            L-100 [2026-09-15] █
 ▀██████████████████████████████████████████████████████████████████████▀

```

---

## ⚡ The Problem: Cognitive Pipeline Thrashing

An architect on a greenfield platform heading for a launch moves across several fronts at once:

* **Upstream Roadmaps:** Commercial commitments and feature targets from product owners.
* **Active Decisions:** Data isolation, authentication at the edge, and messaging topology.
* **Downstream Execution:** Handing off sliced contracts and user stories to engineering teams.
* **Horizon Scanning:** Continuous intake of technical talks and Astrometrics radar signals.

Tracking all of this across chat threads, mental state, and ad-hoc notes causes context fragmentation. Stargazer establishes an asynchronous manufacturing pipeline that separates **what you must design (`owner: me`)** from **what you supervise or delegate**.

---

## 🚀 Key Features

* **Strict "Do vs. Delegate" Partitioning:** Parses non-derivable roadmap milestones (`roadmap.yaml`) to separate your critical design path from upstream product blockers (`ref: null`) and squad tasks.
* **Multi-Track ADR Reconciliation:** Scans independent directories (`2-product`, `3-platform`) where numbers overlap, mapping git commit freshness and review states (`Proposed` vs `Accepted`).
* **Non-Destructive Memory Preservation:** Automatically preserves your manual `- **Next Action:**` scratchpad notes inside `ARCH_DISPATCH.md` across sync runs.
* **Astrometrics & Radar Signal Ingestion:** Ingests `signals.json` from [V'Ger](https://github.com/costap/vger) / [Astrometrics](https://github.com/costap/tech-signals), slotting candidate technology PoCs directly into roadmap horizons.
* **Off-Desk Audio Queue:** Extracts curated YouTube talks from conference digests (`StrimziCon`, `ArgoCon`, `IstioCon`, `KubeCon`) so you can consume architectural research during walks, transit, or physical workouts.
* **Tactical LCARS Terminal:** Built with Charm Lipgloss, rendering authentic Star Trek LCARS status boards, live Stardate telemetry, and cognitive WIP overload warnings.

---

## 📦 Project Layout

```text
stargazer/
├── cmd/stargazer/main.go        # Composition entrypoint (delegates to cli.Execute)
├── internal/
│   ├── cli/                     # Cobra CLI commands (root, sync, status)
│   ├── config/                  # Viper settings (.stargazer.yaml, env vars)
│   ├── domain/                  # Pure domain entities (ADR, Phase, Signal, Board)
│   ├── scanner/                 # ADR git scanner, roadmap loader, signals client
│   ├── render/                  # LCARS terminal and ARCH_DISPATCH.md writers
│   └── app/                     # Core orchestrator service (Engine)
├── configs/                     # Sample configurations
├── Makefile
└── go.mod

```

---

## 🛠️ Quick Start

### 1. Installation

```bash
git clone https://github.com/costap/stargazer.git
cd stargazer
make build

# Optional: install to local PATH
make install

```

### 2. Configuration (`.stargazer.yaml`)

Place `.stargazer.yaml` in your workspace root or `$HOME/.config/stargazer/`:

```yaml
adr:
  wip_limit: 3
  groups:
    - namespace: "2-product"
      dir: "docs/adrs/product"
    - namespace: "3-platform"
      dir: "docs/adrs/platform"

roadmap:
  file: "roadmap.yaml"

signals:
  # A local file or an http(s) URL. Optional.
  source: "https://astrometrics.pfclabs.net/data/signals.json"

dispatch:
  output: "ARCH_DISPATCH.md"
  silent: false

```

### 3. Usage

```bash
# Reconcile ADRs, roadmap, and signals into ARCH_DISPATCH.md
stargazer sync

# Override target files on the fly
stargazer sync --roadmap custom-roadmap.yaml --wip-limit 4

# Render a degraded board instead of failing when an input cannot be read
stargazer sync --keep-going

# Also emit the board as JSON: to stdout, or to a file
stargazer sync --json
stargazer sync --json=board.json

# List every Accepted ADR in the hand-off section, dispatched or not
stargazer sync --all-handoffs

```

**Paths.** A relative path set in the config file is resolved against the config file's own directory, so the same file behaves the same from any working directory. A path given as a flag (`--roadmap`, `--out`, `--signals`) or an environment variable stays relative to where you run the command.

**JSON.** `--json` prints one JSON document to stdout and nothing else (the terminal board is suppressed); `--json=path` writes it to a file. The markdown board is still written. The document has a `schema` number that changes only on a breaking change, the WIP count, the launch countdown, the active and hand-off ADRs (with `date`, `status_line`, `phase`, `next_action` and whether you wrote it by hand), the roadmap gaps, and `warnings` and `adr_unknown` so a consumer can tell a degraded board from a calm one.

**Dispatch state.** The hand-off section lists Accepted ADRs awaiting stories. To record that an ADR's stories have been handed off, add it to the state file (`state.file`, default `.stargazer-state.yaml`), keyed by its track-qualified id:

```yaml
dispatched:
  2-product/0099: PROJ-5301   # a ticket key, or any note such as "done"
```

A dispatched ADR drops out of the hand-off section on the next sync and stays out; the section says how many were left out. A key with no value still means dispatched. An entry that matches no Accepted ADR (a typo, a bare number, an ADR that is still Proposed) prints a `Notice:` and is ignored. A missing state file at the default path means nothing is dispatched; one you named in the config that is missing, or one that cannot be parsed, fails the sync like any other unreadable input.

**Failures.** If an ADR directory cannot be read, or a roadmap you named explicitly cannot be loaded, `sync` prints what failed, exits non-zero and writes nothing. A missing roadmap at the default path is only a notice. With `--keep-going` it renders the board with a `DEGRADED` banner and exits 0, but if the ADR sources were the problem it still leaves `ARCH_DISPATCH.md` untouched: the hand-written `Next Action` lines in that file exist nowhere else, and a board with no pipeline would erase them.

---

## 🗺️ Roadmap Manifest Schema (`roadmap.yaml`)

Stargazer does not duplicate your issue tracker or service registries. It only tracks **milestones and the decisions each still needs**:

```yaml
roadmap_as_of: "2026-06-01"
launch: "2026-09-15"          # L-0 milestone for automated countdown

phases:
  - id: phase-1-launch
    name: Web checkout launch
    target: "2026-Q3"
    status: in-flight
    scope: "Web checkout only, 3 payment methods, 1 region."
    needs:
      - kind: decision        # ref: null captures open blockers before an ADR exists
        ref: null
        gist: Region count at launch, and which regions
        owner: product
        blocks: [PROJ-101, PROJ-102, REL-03]

      - kind: adr
        ref: 2-product/0099
        owner: me
        note: Accepting it needs a partial-supersession note on 2-product/0037

      - kind: design
        ref: designs/checkout-risk
        owner: me
        squad: checkout

```

**ADR references.** Qualify an `adr` ref with its track (`2-product/0099`). A bare number (`0099`, `99`, `ADR-0099`) binds only when exactly one track has that number; if several do, or none, it binds to nothing and sync prints a `Notice:` naming the candidates.


---

## 🖖 Philosophy & Rhythm

1. **Protect Deep Design:** Morning hours are reserved for offline architecture design on items in your active pipeline.
2. **Batch Asynchronous Communication:** Midday is dedicated to answering chat questions and running ADR reviews.
3. **Automate Ticket Mechanics:** Accepted ADRs are piped directly into story-generation skills for PM and squad backlog refinement.
4. **Reclaim Off-Desk Time:** When it is time to step away, close the IDE. The dispatch board holds the exact next action for every front, while the audio queue takes care of conference research on the move.