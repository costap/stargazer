# SG-016 Keep a YouTube playlist in sync with the audio queue

Status: phases 1 to 4 built 2026-11-09; phase 5 (live check) open.

## Goal

When you step away, open one playlist on your phone, press play, and the talks Stargazer has queued come up in order. No copying links by hand.

## Decisions (made)

| Question | Decision |
|---|---|
| Which playlist | Stargazer creates and owns a private playlist, "Stargazer queue", on first run. It only ever adds and removes items in that playlist and never deletes the playlist itself |
| What leaves it | A cap (default 25, newest first) plus a hand-kept `heard:` list in the state file. Anything whose signal is no longer `spotted` also drops out. The YouTube API cannot read watch history, so "heard" is yours to record |
| How it runs | A separate command, `stargazer playlist sync`, with its own `--dry-run` and a `stargazer playlist login` step. `stargazer sync` stays local and never touches the network for this |

## Behaviour

1. **Desired list.** Start from the audio queue `stargazer sync` already builds (SG-014): YouTube links from signals and from each digest's "Watch These First" section, de-duplicated. Drop any entry whose signal id or video id is under `heard:`. Drop signals whose status is not `spotted`. Order: signals newest first, then digest links in digest order. Keep the first N (`youtube.playlist.cap`).
2. **Diff.** Compare with the playlist's current items by video id. Add what is missing, remove what is not wanted, and reorder only when the order differs. The planner is a pure function, so it is table-tested with no network.
3. **Apply.** Print the plan, then apply it. `--dry-run` prints and stops. Re-running with no changes does nothing (idempotent).
4. **Failure is loud.** Non-zero exit and a named reason for an expired login, a revoked token, exhausted quota or a missing playlist. A failed playlist sync never affects the markdown board.

## Design

- `internal/domain/playlist.go` (pure Go): `VideoID(url)`, `PlanPlaylist(desired, current) Plan{Add, Remove, Move}`. No I/O.
- `internal/youtube/`: a small client over `net/http` and `golang.org/x/oauth2` (not the large Google SDK). Calls: `playlists.insert`, `playlistItems.list`, `playlistItems.insert`, `playlistItems.delete`, `playlistItems.update` (position).
- Auth: OAuth 2.0 installed-app flow with a loopback redirect and PKCE, scope `https://www.googleapis.com/auth/youtube`. The client id and secret come from the JSON Google lets you download, at `youtube.client_file`. The token is stored at `youtube.token_file` (default `~/.config/stargazer/youtube-token.json`, mode 0600), outside any repo.
- Machine-owned sidecar `.stargazer-youtube.json` next to the state file: the playlist id and last sync time. The hand-kept state file stays read-only to Stargazer, so the id goes in the sidecar, which is git-ignored.
- Config: `youtube.client_file`, `youtube.token_file`, `youtube.playlist.title`, `youtube.playlist.cap`. State file: `heard:` (a list of signal ids or video ids).
- Quota: the default allowance is 10,000 units a day. List is 1 unit a page; insert, delete, update and playlist create are 50 each. A cold fill of 25 talks costs about 1,300 units, and a typical day a few hundred.

## Phases

| # | Work | Needs you |
|---|---|---|
| P0 | SG-014: audio queue on by default, `--no-audio`, extraction tested. Found and fixed: digest links were never matched, because the parser required `[http` and the real digests write `**[Title](url)**` | done |
| P1 (done) | `domain.VideoID`, `PlanPlaylist` with table tests; `heard:` in the state file; the desired-list builder | nothing |
| P2 (done) | `internal/youtube` client, tested against an `httptest` server | nothing |
| P3 (done) | `playlist login`: loopback and PKCE flow, token file | Google Cloud setup (below) |
| P4 (done) | `playlist sync` and `--dry-run`, the sidecar, config, README | nothing |
| P5 | Live check: create the playlist, fill it, change the queue, resync, confirm it on your phone | P3 done, and a few minutes with you |

P1, P2 and P4 can be built and fully tested without any Google account. P3 and P5 cannot.

## What you set up once (Google Cloud, about ten minutes)

1. Create a Google Cloud project and enable the YouTube Data API v3.
2. Configure the OAuth consent screen as External, then set its publishing status to In production. In Testing status Google expires the refresh token after seven days, and the sync would start failing a week after login. An unverified personal app in production just shows a warning screen once.
3. Create an OAuth client of type Desktop app and download its JSON to the path you put in `youtube.client_file`.
4. Run `stargazer playlist login`, approve in the browser once.

## Risks and notes

- Background and screen-off playback in the YouTube mobile app needs YouTube Premium. Without it, play stops when the screen locks. Check this before building P3 and P5, because it decides whether the playlist solves "when I'm out".
- Quota is shared with anything else on the same Google Cloud project.
- No client secret or token may be committed: the repo is going public. The example config carries placeholders only, the token lives outside the repo, and the sidecar is git-ignored.
- Digest link extraction depends on the `## Watch These First` heading. If tech-signals changes that heading, the digest half of the queue silently empties. P1 should log a notice when a digest has no such section.
- Deleting is limited to items in the owned playlist, by playlist item id, and only those the plan marks for removal.
