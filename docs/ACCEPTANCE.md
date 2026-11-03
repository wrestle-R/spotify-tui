# Acceptance checks

Prepared 28 October 2026. This is a test specification. **No application test below has been run.** A future run must record PASS, FAIL, BLOCKED, or NOT RUN, with dated evidence. An unchecked item never implies success.

## Release gates

| Gate | Required result | Current status |
| --- | --- | --- |
| G1: real account | Login, liked songs, required playlists, search | NOT RUN |
| G2: local playback | Full audio with desktop/web clients closed | NOT RUN |
| G4: stability/resources | Long-run, recovery, and budget evidence | NOT RUN |
| G5: distribution | Matching Go/Rust artifacts and verified installer | NOT RUN for app; planning distribution checked separately |

An app release requires every required gate to pass. A planning release must clearly identify itself as documentation. Streaming-only software cannot be described as meeting the original requirements.

## A. Provider feasibility evidence

- [ ] Record current provider/version and dated documentation for supported music downloads.
- [ ] Confirm personal-app eligibility for this Premium account and Arch Linux.
- [ ] Identify download, removal, progress, and license renewal operations.
- [ ] Prove process-independent authorized session persistence.
- [ ] Record any required initial official-client pairing separately from routine operation.
- [ ] Check owned, private, collaborative, followed, and Spotify-curated playlist access where relevant.
- [ ] Stop and record no-go if any mandatory capability has no route. Do not invent a positive result.

## B. Account and metadata

| ID | Scenario | Expected result |
| --- | --- | --- |
| B01 | First browser authorization | Correct account; validated callback state; temporary listener closes |
| B02 | Cancel or deny authorization | TUI remains usable; retry available; no false logged-in state |
| B03 | Occupied callback port | Clear error before browser opens; no mismatched redirect |
| B04 | Token expires during concurrent reads | One refresh operation; waiting reads resume safely |
| B05 | Token revoked | One refresh attempt then reauthorization; no retry storm |
| B06 | Liked collection exceeds one API page | Correct complete order; no dropped or repeated page |
| B07 | Playlist includes same song twice | Both occurrences survive; selection plays intended occurrence |
| B08 | Playlist items include null/episode/local entries | No panic; unsupported entries accurately marked |
| B09 | Refresh fails halfway | Previous coherent collection retained and marked stale |
| B10 | Forbidden followed playlist | Access restriction shown; not an empty successful playlist |
| B11 | Search A returns after search B | Only B appears in current results |
| B12 | Search uses punctuation and non-Latin text | Escaping and rendering correct; no terminal escape execution |
| B13 | Rate limit and account quota responses | Appropriate cooldown or stop; visible reason; no busy loop |
| B14 | Account switch/logout | Credentials and cache isolated; no previous account data leakage |

## C. Local playback and terminal behavior

| ID | Scenario | Expected result |
| --- | --- | --- |
| C01 | Official desktop/web players closed | Full Spotify track plays from this machine |
| C02 | Launch after complete helper shutdown | Stored session restored through supported provider behavior |
| C03 | Play selected playlist occurrence | Starts correct item and continues through collection |
| C04 | Pause/resume/seek/volume | Actual audio and displayed state agree |
| C05 | Shuffle/repeat/next/previous | One queue authority; no duplicate advances or infinite loops |
| C06 | Empty or entirely unavailable collection | Useful error; no repeated auto-skip loop |
| C07 | Second TUI launch | Attaches to one helper; no duplicate local player |
| C08 | Detach and reattach | Matches documented policy; audio state preserved |
| C09 | Explicit stop | This project's helper/provider exits; unrelated clients unaffected |
| C10 | Resize 100x30, 80x24, 60x18 | Readable adapted layout; supported controls remain reachable |
| C11 | Resize below minimum | Resize notice; safe quit; no panic |
| C12 | Unicode, emoji, wide characters | Columns remain coherent; no raw control sequence injection |
| C13 | Type spaces, q, j/k, n/p in search | Text input gets characters; global actions do not fire |
| C14 | Normal exit, Ctrl-C, handled error, panic recovery | Terminal echo/cursor/raw mode restored where process can clean up |
| C15 | Provider missing/expired/control API unavailable | Correct actionable state; no false readiness |
| C16 | Helper crashes during playback | UI explains failure; bounded recovery; no zombie process growth |
| C17 | Speakers/headphones/default output changes | Expected routing; no unsolicited global volume change |
| C18 | Bluetooth disappears, PipeWire restarts, suspend/resume | Recoverable state; no permanent UI freeze |
| C19 | Malformed/incompatible IPC | Connection rejected safely; no crash or silent incompatible behavior |
| C20 | Slow client consumes events | Bounded buffers; full state resync when needed |
| C21 | Media keys if included | UI, MPRIS, and actual playback state remain consistent |

SIGKILL cannot run terminal cleanup. Document recovery for externally killed processes rather than claiming panic handling solves every termination mode.

## E. Resource measurements

Baseline Spotify and the candidate with the same playlist, bitrate, output, and library size. Record whether terminal overhead is included and apply the same rule to comparisons. Account for all helper/provider descendants, not just the Go PID.

Suggested sample plan:

| Workload | Duration | Record |
| --- | --- | --- |
| Cold launch and warm launch | 5 runs each | Usable-view latency; peak memory |
| Visible, paused | 10 minutes | Mean/peak PSS and CPU |
| Visible, playing | 60 minutes | Mean/peak PSS, CPU, input latency |
| Detached playback | 60 minutes | Whole-player footprint and orphan checks |
| Search and large-library navigation | 100 transitions | Retained-memory behavior and race failures |
| Extended playback | 8 hours | Time-series memory trend after warmup; glitches; retries |
| Downloads, if supported | Representative small collection | Memory, CPU, disk growth, storage bound |

Sample at a consistent interval such as 5 seconds. Prefer combined PSS from readable `smaps_rollup` records; also retain RSS for comparison. Treat measurement limitations explicitly. Separate true retention from temporary buffers and allocator plateaus.

Compare results against PLAN.md's proposed budgets. Any increased budget needs a reason and user-visible measurement. Do not report estimated savings as measured results.

## F. Build and integration gates

Run these only after application source and manifests exist:

```bash
gofmt -l go/cmd go/internal
(cd go && go vet ./...)
(cd go && go test ./...)
(cd go && go test -race ./...)
(cd go && CGO_ENABLED=0 go build -trimpath -o ../dist/spotify-tui ./cmd/spotify-tui)
cargo fmt --manifest-path rust/Cargo.toml --check
cargo clippy --manifest-path rust/Cargo.toml --locked --all-targets -- -D warnings
cargo test --manifest-path rust/Cargo.toml --locked
cargo build --manifest-path rust/Cargo.toml --locked --release
```

Use deterministic synthetic records for HTTP and UI tests. Use fixture messages consumed by both Go and Rust for the protocol contract. Private listening data and tokens must not enter the test repository.

## G. App distribution, after implementation

- [ ] Archive contains matching-version Go and Rust binaries, license notices, and clear runtime prerequisites.
- [ ] Architecture and Linux ABI compatibility are documented.
- [ ] SHA-256 verifies archive integrity; signed checksums or artifact attestations are a separate authenticity improvement.
- [ ] Installer downloads to a temporary directory and verifies before extraction/execution.
- [ ] Installer refuses unsupported platforms and invalid/missing checksums.
- [ ] Installer defaults to a user-owned prefix, supports an explicit prefix, and does not use sudo.
- [ ] Existing version is preserved until both new binaries are in place; failed upgrades can roll back.
- [ ] Paths with spaces, interrupted downloads, corrupt archives, and private-repository authentication failures are tested.
- [ ] Runtime credentials are never included in a release or requested by the installer.
- [ ] Official provider binaries are distributed only where their actual terms allow it.
- [ ] Clean install, upgrade, uninstall, and launch are tested on Arch.

## H. Evidence record template

For each test record: test ID; date; app/Go/Rust/provider versions; environment; preconditions; steps; expected result; actual result; PASS/FAIL/BLOCKED; evidence location; next action.

At the planning stage, report document/link/archive checks separately from application verification. A successfully downloaded plan does not mean a successfully tested music player.
