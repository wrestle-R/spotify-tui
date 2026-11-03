# Spotify TUI: detailed implementation plan

Prepared: 28 October 2026, Asia/Calcutta.

Project directory: `/home/rdp/Desktop/code/Spotify-Tui`.

## 1. What we are making

A small terminal music player for Arch Linux. Open it, find a song or playlist, press Enter, and listen through this computer. Spotify's desktop app should be fully closed. The interface stays simple: a library on the left, a song list on the right, and playback information at the bottom.

Lower memory use is the reason for the project. Neither the existing Spotify footprint nor a replacement's memory use has been measured. Performance numbers below are proposed engineering targets, not claims about a built app.

The app does not need AI, chat, voice commands, album-art rendering, recommendations, lyrics, podcasts, social features, visualizers, a web dashboard, or a tray UI. Avoid adding those while building the requested basics.

## 2. The feasibility decision comes first

**Do not mistake a detailed plan for proof that every requirement is currently achievable.**

Current findings, with primary links in [SOURCES.md](docs/SOURCES.md):

| --- | --- | --- | --- |
| Spotify desktop app controlled by a TUI | Could retain the official player's capabilities | Requires the official player | Fails the explicit replacement requirement |
| Locally owned audio files | Could support genuine local playback | Does not download the user's Spotify catalog | A different scope, not an automatic substitute |

The remaining architecture is a conditional build specification. It becomes executable project work only after the gate passes or the user explicitly changes the requirement.

## 3. What “works” means

All of these must be true for full acceptance:

1. The user's real Premium account can authenticate through supported browser authorization or provider pairing.
2. The user's liked songs and required playlists are accessible, including all pages of results.
3. Catalog search can find a track and start its full Spotify audio on this computer.
4. The official desktop app and web player remain closed throughout normal playback.
6. After all player processes are stopped, the app can cold-start without external connectivity and play downloaded tracks, including one that was not previously played.
7. Playback controls, navigation, errors, and terminal restoration work on Arch Linux.
8. Memory and CPU measurements meet the agreed budget over extended use.
9. Account data and credentials remain local except for necessary provider requests and any explicitly authorized secrets-file sync.

Mock data, a rendered TUI, successful compilation, or a streaming demo alone does not establish acceptance. Evidence requirements are in [ACCEPTANCE.md](docs/ACCEPTANCE.md).

## 4. Requirements and deliberate limits

| ID | Requirement | Priority | Verification |
| --- | --- | --- | --- |
| R01 | Go terminal interface | Required | Source layout and terminal demonstration |
| R02 | Rust playback helper | Required | Helper owns playback lifecycle; Go contains no audio engine |
| R03 | Spotify account login | Required | Real account authorization and successful restart |
| R04 | Liked songs | Required | Correct saved-track order, paging, and playback |
| R05 | Playlists | Required | User-owned, collaborative, and required followed playlists checked separately |
| R06 | Search | Required | Track search plus playback, with paging and failure handling |
| R07 | Standalone local audio | Required | Desktop/web clients closed; sound from this machine |
| R09 | Lower sustained memory use | Required | Compare measured whole-player footprint with Spotify baseline |
| R10 | Keyboard navigation | Required | Vim-style and arrow keys; no mouse dependency |
| R11 | Pause, previous, next, seek, volume | Required | Real audio state matches displayed state |
| R12 | Shuffle, repeat, basic queue | Supporting basics | Local/provider queue semantics verified |
| R13 | Media keys through MPRIS | Useful after core passes | Consistent state across UI and media-key actions |
| R14 | Keep playing after closing the TUI | Proposed default | Detach and reattach without duplicate players |

R14 is a proposed lifecycle choice, not a confirmed user preference. An explicit stop action must always exist. Do not make a background service start at login without a later user request.

Read-only library browsing is enough for the first accepted version. Editing playlists or liking/unliking tracks is outside this plan's initial scope; that avoids extra permissions and synchronization conflicts.

## 5. Phase 0: feasibility before product code

### 5.1 Establish the local baseline

Observed during planning: the requested folder was empty, it had no Git repository or ancestor repository, the execution host identifies as Arch Linux x86_64, Go and Rust commands are installed, and a PipeWire server is active. These are read-only observations. Toolchain versions, development headers, target terminal, audio backend compatibility, and account API access still need verification.

Future preflight records: Go version, Rust/Cargo version, chosen terminal, PipeWire/PulseAudio compatibility, writable XDG directories, keyring availability, and installed candidate provider version. Do not install packages just because this plan lists a dependency. Inspect the selected provider's actual requirements first.

Measure the existing Spotify application under a repeatable workload before replacing it. Do not assume the user's rough “a lot” report gives a usable baseline.

### 5.3 Check library access as its own gate

Spotify's current playlist-items reference limits access to playlists the user owns or collaborates on. Merely following a playlist is a distinct case. New API restrictions can therefore affect “my playlists” even when login succeeds. [Playlist-items reference](https://developer.spotify.com/documentation/web-api/reference/get-playlists-items).

Check at least one owned playlist, one private playlist, one collaborative playlist if available, one followed playlist if used, and one Spotify-curated playlist if used. Record which real requirements work with the account and app mode. Do not promise access to every playlist merely because `/me/playlists` returns its name.

### 5.4 Feasibility deliverables

## 6. Technology choices

| Layer | Proposed technology | Responsibility |
| --- | --- | --- |
| Terminal app | Go with Bubble Tea | Input, navigation, view state, terminal lifecycle |
| Terminal widgets | Bubbles, used selectively | Search input and simple lists |
| Styling | Lip Gloss | Spacing, borders, selection, terminal colors |
| Playback helper | Rust | Provider lifecycle, audio control, download capability boundary |
| Async helper runtime | Tokio, minimal required features | IPC, process supervision, provider events |
| Local protocol | Versioned JSON over a Unix-domain socket | Requests, responses, playback events |
| Library/auth HTTP | Go standard `net/http` | Spotify OAuth and metadata requests |
| Metadata storage | SQLite, owned by Go | Collections, paging, indexes, stale-cache state |
| Configuration | Small TOML file | Non-secret settings and paths |
| Secrets | Desktop keyring where available | OAuth refresh token and provider credentials |
| Linux audio | PipeWire default output via provider support | Sound on the user's existing audio setup |

These choices are engineering proposals. The first build must select mutually compatible stable dependency versions, pin Go modules and Cargo dependencies, and document the toolchain versions. Do not mix example imports from different major versions of the Charm libraries. [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Bubbles](https://github.com/charmbracelet/bubbles), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Tokio](https://docs.rs/tokio/latest/tokio/).

Use separate processes rather than Go/Rust FFI. That keeps the build understandable, avoids a cgo audio bridge, and lets the terminal reconnect after a UI crash. This is a maintainability choice, not a claim that two processes always use less RAM.

SQLite driver selection happens during dependency review. Prefer a driver that preserves a straightforward Go build, then measure its actual overhead. Both runtimes must not open the same database for writes.

## 7. Runtime architecture

The Go process never receives decoded audio or an entire audio download. The Rust helper never renders terminal views. Metadata traffic and playback traffic have separate failure states.

Start with one provider after Phase 0. Do not build a plugin marketplace or several production adapters. If Soloist is selected, Rust supervises the official binary and translates its local events; this does not make the official binary Rust code. If librespot is selected after an explicitly approved streaming-only scope change, Rust embeds or controls it using a pinned supported release.

## 8. Provider-specific constraints

### Soloist candidate

The official overview describes headless local playback and control integration. It requires a personal API key, prohibits redistributing its binaries, and says builds expire after 90 days. This makes it worth checking before assuming an unofficial engine is necessary, but also introduces an update dependency. [Overview](https://developer.spotify.com/documentation/soloist).

Its stored Connect session and API key are separate from the Go app's Web API authorization. Plan for initial pairing using an official Spotify client, then verify that subsequent cold starts work after that client is closed. Do not promise one universal login for all layers. [Authentication](https://developer.spotify.com/documentation/soloist/concepts/authentication).

Its local WebSocket interface has no built-in authentication or browser-origin protection. If used, bind it to loopback, avoid an exposed LAN listener, verify actual readiness, and document the residual local-process/browser threat rather than calling loopback an authentication mechanism. A failed listener may leave the provider running without a usable control API, so PID existence is not a readiness check. [WebSocket reference](https://developer.spotify.com/documentation/soloist/reference/websocket-api), [CLI reference](https://developer.spotify.com/documentation/soloist/reference/command-line).

### Librespot candidate

Do not extract credentials or client identities from the official desktop app to work around provider restrictions. Account authentication changes are a provider compatibility issue to diagnose, not something this plan can wish away.

## 9. TUI design and interaction

Use [the terminal mockup](design/tui-concept.png) as a direction, not a literal specification of working downloads. Album art and the earlier GUI concepts are excluded.

```text
music                                           Online
Search: _
+--------------------+--------------------------------------------+
| Library            | Liked songs                                |
| > Liked songs      | > Intro          The xx              2:07  |
|   Playlists        |   Nude           Radiohead           4:15  |
|   Downloads        |   Holocene       Bon Iver            5:37  |
|                    |                                            |
| Late nights        |                                            |
| On repeat          |                                            |
| Slow mornings      |                                            |
+--------------------+--------------------------------------------+
Playing: Intro - The xx           [========----------] 0:42 / 2:07
/ search  j/k move  enter play  space pause  ? help  q quit
```

Default at 100x30: sidebar roughly 24 columns, remaining width for title, artist, duration. At 80x24 reduce optional labels. At 60x18 switch to one active pane and a short playback line. Below the supported minimum display a resize message while keeping quit and playback controls usable.

Use terminal colors where possible, one restrained accent, and ASCII fallback for box drawing. No Nerd Font dependency. Width handling must work for Unicode song names, combining characters, emoji, and wide CJK characters. Sanitize control characters from all remote text before terminal rendering.

| Key | Action | Context rule |
| --- | --- | --- |
| `j/k`, arrows | Move selection | In lists only; letters type normally in search |
| `Tab`, `Shift-Tab` | Change pane focus | Preserve each pane's selection |
| `Enter` | Open collection or play selected track | Never launches a browser to play audio |
| `/` | Start search | Focus text input |
| `Esc` | Cancel search or close popup | Does not stop current playback |
| `Space` | Pause/resume | In text input inserts a space |
| `n/p` | Next/previous | In text input types letters |
| `h/l` | Seek backward/forward five seconds | In list/playback context only |
| `+/-` | Adjust app volume | Clamp to provider-supported range |
| `s` | Toggle shuffle | Outside text input |
| `r` | Cycle repeat off/collection/track | Outside text input |
| `d` | Download selected track or collection | Only with verified provider capability |
| `D` | Open Downloads | No silent deletion |
| `x` | Remove selected download | Downloads view; show confirmation |
| `R` | Refresh current collection | Does not erase a valid cache on failure |
| `?` | Open concise help | Show active keybindings |
| `q` | Close TUI | Proposed detach behavior; show audio continues |
| `Q` | Stop playback and close | Explicit shutdown action |
| `Ctrl-C` | Interrupt TUI safely | Restore terminal; apply documented detach policy |

These are proposed defaults, configurable later if needed. Minimize settings in the first version.

Loading is a short status line. Distinguish empty, loading, stale, forbidden, and failed collections. “No songs” is not an appropriate message for an API error. Show the active account/provider in a small status/help view, not a dashboard.

## 10. Login and credential handling

Use Authorization Code with PKCE for Web API access. A desktop client must not rely on hiding a client secret inside its binary. Use a random verifier, S256 challenge, and a separate random state. Bind the temporary callback listener to an explicit loopback IP, validate the callback path/state, apply a timeout, and close it after authorization. The registered redirect and the implementation must agree. [PKCE](https://developer.spotify.com/documentation/web-api/tutorials/code-pkce-flow), [redirect rules](https://developer.spotify.com/documentation/web-api/concepts/redirect_uri).

Proposed redirect to validate during setup: `http://127.0.0.1:43821/callback`. Check port availability before opening a browser; do not silently pick a differently registered URI.

Proposed Web API scopes: `user-library-read`, `playlist-read-private`, and `playlist-read-collaborative` when needed. Add `user-read-private` only if used by the selected account checks. Add `user-read-playback-state` and `user-modify-playback-state` only if verified operations require Web API playback control. Request no write-library, email, or recommendation-related scopes without a feature that needs them. Confirm exact current scope requirements against each endpoint during Phase 0.

First run: explain developer-app setup, capture the non-secret client ID, authorize in the browser, establish the account namespace, then pair the playback provider if it has separate credentials. Canceling any step returns to a usable retry screen. No Spotify password should be typed into this TUI.

Refresh once per expiry using a shared refresh lock; concurrent requests wait for it. On a 401, attempt one refresh and replay only safe read requests. Failed refresh requires reauthorization. Never retry a playback mutation blindly when its outcome is unknown.

Store secrets in the user's available keyring. If unavailable, offer an explicitly disclosed permission-restricted local credential file; owner-only permissions are not encryption. Do not log tokens, callback codes, authorization headers, API keys, or raw provider command lines containing credentials. Provider launch arguments require a specific review if the only documented key mechanism exposes it to same-user process inspection.

Logging out stops this project's provider session, clears its credentials and account-specific cached data as appropriate, and leaves unrelated Spotify clients alone. Switching accounts must not reuse another account's metadata or authorized audio.

## 11. Metadata, search, and API behavior

Go owns one bounded HTTP client, account authorization, and a single metadata repository. Start with direct standard-library HTTP wrappers for the small endpoint set instead of a large SDK that may lag current response shapes.

| Operation | Candidate Web API route | Required handling |
| --- | --- | --- |
| Identify current user | `GET /v1/me` | Stable account namespace, nullable fields |
| Liked songs | `GET /v1/me/tracks` | Pagination, added-date order, unavailable tracks |
| Playlist list | `GET /v1/me/playlists` | Pagination, privacy, ownership/collaboration distinction |
| Playlist items | `GET /v1/playlists/{id}/items` | Current route and response shape, restrictions |
| Track search | `GET /v1/search` with `type=track` | Query escaping, result paging, access limits |

Do not use the deprecated playlist `/tracks` route in new code. Current February migration material specifies a smaller search limit for affected apps; start conservatively with at most 10 results and verify the live mode. Parse the current response rather than assuming an old SDK's fields. [API change reference](https://developer.spotify.com/documentation/web-api/references/changes/february-2026).

Later changes matter too: July 2026 increased app limits and moved development quota accounting to the developer account. Do not copy the earlier one-client-ID rule into setup instructions as current fact. Handle both rate-limiting and account-quota errors. [July changelog](https://developer.spotify.com/documentation/web-api/references/changes/july-2026).

Use a bounded in-flight request budget, a sensible timeout, and cancellation when changing views. On 429 respect `Retry-After` where present, retain a cooldown, and show it. If an account quota is exhausted, stop background retrying and explain the condition. Retry safe reads on transient failures with capped backoff; do not spin forever. [Rate limits](https://developer.spotify.com/documentation/web-api/concepts/rate-limits).

## 12. State and storage

Use XDG locations, keeping runtime data outside the source checkout:

| Data | Proposed location | Owner |
| --- | --- | --- |
| Non-secret configuration | `$XDG_CONFIG_HOME/spotify-tui/config.toml` | Go |
| Metadata database | `$XDG_DATA_HOME/spotify-tui/library.sqlite3` | Go |
| Logs/state | `$XDG_STATE_HOME/spotify-tui/` | Each process, separate files |
| Socket and lock | `$XDG_RUNTIME_DIR/spotify-tui/` | Rust |
| Playback session | Provider-specific private directory | Provider through Rust |
| Temporary playback cache | `$XDG_CACHE_HOME/spotify-tui/` | Provider through Rust |

Use standard XDG defaults when variables are unset. Validate runtime-directory ownership and permissions; do not place an unauthenticated socket at a predictable world-writable path. If there is no suitable runtime directory, use a private per-user directory and clearly document its lifecycle.

Proposed metadata tables: accounts, tracks, collections, collection_items, sync_state, and search_cache. A track is keyed by provider URI/account context. Collection items preserve occurrence/order; do not deduplicate a playlist by track ID and lose repeated entries. Track records include nullable duration, artists, playable status, item kind, and last-fetched time. Do not serialize arbitrary complete API payloads indefinitely.

Keep UI selection state in a small local state file. Do not store credentials in SQLite. Provider download manifests and audio license details remain provider-owned; Go may keep a derived status summary but never invent entitlement state.

Fetch pages incrementally and render visible rows. Bound in-memory track pages, database page cache, logs, response sizes, and background work. A tentative first budget is 8 recently viewed track pages of up to 100 records; validate it against actual endpoint limits and memory profiling. A full metadata collection may live on disk without residing in both Go and Rust memory.

## 13. Local Go/Rust protocol

Use newline-delimited JSON over a Unix-domain socket, with `0600` socket permissions inside a `0700` directory and same-user peer validation where supported. Keep the wire contract versioned independently from app release versions. No HTTP server or LAN port is needed for our own IPC.

Each command has protocol version, request ID, method, and params. Each response has the matching ID and either result or structured error. Events carry a monotonically increasing sequence, helper session ID, and type. A reconnect obtains a fresh state snapshot before applying events; discard events from an old helper session.

Proposed commands: `hello`, `capabilities`, `state`, `play_collection`, `play_track`, `pause`, `resume`, `next`, `previous`, `seek`, `set_volume`, `set_shuffle`, `set_repeat`, `downloads_list`, `download_start`, `download_cancel`, `download_remove`, and `shutdown`.

These are our future helper methods, not claims about existing Spotify endpoints. Unsupported download commands return `UnsupportedCapability` and cannot satisfy release acceptance.

Bound individual messages to 256 KiB initially and reject oversize frames. Do not send entire libraries over this socket: Go resolves metadata and supplies a bounded queue window or a provider collection URI. Large providers' queue limits require explicit handling, not truncated collections. Use timeouts, preserve unknown-outcome mutation errors, and define whether a request can be retried. Include cross-language golden fixtures for messages and errors.

## 14. Playback, queue, and lifecycle

One helper owns one local playback session. Launch only when needed, acquire a lock, and advertise readiness after initialization succeeds. A second TUI connects to the existing helper; it must not spawn a second player or steal playback.

The queue stores stable track identities and an occurrence index. Selecting a song in a collection starts at that occurrence and continues through the collection, rather than playing only that row. If a provider owns queue/shuffle/repeat, it is authoritative. If local queue coordination is necessary, document that design and test it; do not run two independent queue owners.

State transitions include stopped, loading, playing, paused, reconnecting, unavailable, and error. Do not show Playing before the provider confirms playback. Interpolate the progress display locally from timestamped provider state; avoid network polling once per frame. Stop interpolation on pause, buffering, seek, or disconnect.

Unsupported tracks show a reason where available. Avoid an endless skip loop when a playlist has no playable items. Preserve the selected occurrence through metadata refreshes. Handle one-track and empty queues, manual skip while repeat-track is enabled, and previous behavior near the start of a track.

Use the current system output by default. Test speakers, headphones, output changes, Bluetooth loss, PipeWire restart, and suspend/resume. Do not change the user's global volume, default sink, or audio configuration during setup.

Proposed background policy: `q` detaches the TUI, `Q` stops audio and exits, and a future explicit `stop` command terminates this project's helper. Tell the user what happened. No automatic systemd service or autostart entry in the first version. If the UI crashes, restore its terminal on the next safe opportunity and leave helper behavior deterministic. If the helper crashes, show it and reconnect only with bounded retries; do not create an infinite restart loop.

## 16. Resource budgets and measurement

Proposed first targets, to confirm against the chosen provider and this machine:

| Measure | Target | Measurement boundary |
| --- | --- | --- |
| Warm startup | Usable cached library within 1 second | Credentials already present, no account login |
| Local input response | 95th percentile under 50 ms | Key event to updated view, no network dependency |
| Idle memory | Combined PSS at or below 100 MiB | Go, Rust, and any external provider processes |
| Playing memory | Combined PSS at or below 150 MiB | Same process set, representative library |
| Compared with Spotify | Aim for at least 50% lower PSS | Same workload and accounting convention |
| Idle CPU | Mean below 1% of one logical core | Normalized process CPU convention stated |
| Playback CPU | Mean below 5% of one logical core | No visualizers or album image decoding |
| Long-session memory | No unexplained sustained growth | Sample 1-hour and 8-hour runs after warmup |

These are acceptance proposals, not measured results or user-confirmed numerical requirements. If a legitimate provider cannot meet them, report the measurements and discuss the budget rather than omitting its process from totals.

Measure PSS from `/proc/<pid>/smaps_rollup` where accessible. RSS can be recorded too but do not add shared pages repeatedly and label that precise physical consumption. Use a dedicated process group or user scope to account for descendants and providers that may outlive the TUI. Baseline and replacement must use comparable terminal overhead accounting.

Use the same playlist, bitrate, output device, and run duration. Record library size, versions, cache warmth, OS and audio stack, and sampling interval. Test visible TUI, detached playback, paused, searching, long library browsing, and downloads if available. Measure cache and log disk growth separately from RAM.

Avoid continuous rendering while idle. Use event-driven updates and at most a modest progress tick while playing. Avoid unbounded channels, one task per song, duplicate cover art, full-library copies across processes, and repeated OAuth refresh goroutines. Profile before introducing compression, custom allocators, or clever caches.

## 17. Implementation sequence and stop conditions

| Phase | Work | Exit condition |
| --- | --- | --- |
| 1 | Pin toolchains/dependencies, choose licenses, define protocol and fixtures | Small Go/Rust IPC round trip with clean shutdown |
| 2 | Login, account namespace, bounded metadata client | Real liked songs, playlist access matrix, and refresh pass |
| 3 | Rust provider adapter and local audio | Full tracks with official clients closed; restart pass |
| 4 | Go TUI with real data and controls | Keyboard, resize, Unicode, and async race checks pass |
| 6 | Recovery, long-run profiling, media keys if included | Stability and resource checks pass on Arch |
| 7 | Packaging and final handoff | Clean install, upgrade, uninstall, and user walkthrough pass |

Within each phase, deliver a small working slice and record evidence before expanding it. Fix blocking behavior rather than adding screens. Do not give a calendar commitment for phases 1–7 until Phase 0 is positive; research results are not a schedule guarantee.

## 18. Verification strategy

Use focused automated tests for failure-prone behavior: OAuth callback validation, refresh coordination, pagination consistency, playlist occurrence preservation, delayed-search results, IPC framing/versioning, queue transitions, download state interpretation, and terminal restoration. Do not write tests merely to assert that getters return constants.

Go tests include reducer/input behavior, HTTP fixtures, socket disconnects, and text layout at common sizes. Rust tests include provider-adapter behavior, state/event sequences, bounded queues, framing, and shutdown. Cross-language fixtures must be consumed by both suites, including malformed and oversize messages.

Planned quality gates include Go formatting/tests/vet/race checks and Rust formatting/tests/clippy/locked release build. Exact commands and run conditions are specified in [ACCEPTANCE.md](docs/ACCEPTANCE.md). No such build checks have run during planning because no application code exists.

## 19. Error and recovery behavior

| Failure | Required response |
| --- | --- |
| OAuth canceled | Return to login state; keep terminal usable |
| Callback port in use | Explain conflict before opening browser; no random redirect mismatch |
| Token revoked | Request reauthorization; avoid infinite refresh |
| Playlist forbidden | Explain ownership/collaboration restriction; do not show empty success |
| HTTP 429 | Respect cooldown; distinguish account quota when supplied |
| Track unavailable | Mark reason and avoid endless automatic skipping |
| Provider missing | Give exact missing component and setup step |
| Provider expired | Explain update requirement without deleting credentials |
| Provider control socket absent | Treat as not ready even if its process exists |
| Helper crash | Surface failure, reconnect within a cap, preserve library state |
| Disk full | Stop new download safely; preserve existing valid downloads |
| Keyring locked | Give unlock/retry path; do not print secrets |
| Metadata refresh interrupted | Keep previous complete generation with stale indicator |
| Incompatible Go/Rust versions | Explain required matching installation; do not silently downgrade protocol |

Logs are opt-in for verbose diagnostics, bounded and rotated. Normal UI errors are short and actionable. Debug reports redact account identifiers where practical and never include credential storage or the entire private library.

## 20. Source layout

Updated during implementation after the user requested separate `go/`, `rust/` and `scripts/` directories:

```text
Spotify-Tui/
  README.md
  PLAN.md
  .gitignore
  go/
    go.mod
    go.sum
    cmd/spotify-tui/main.go
    internal/auth/
    internal/config/
    internal/ipc/
    internal/spotify/
    internal/ui/
  rust/
    Cargo.toml
    Cargo.lock
    src/main.rs
  protocol/fixtures/
  scripts/
  docs/
  design/
  local-docs/                  # ignored private notes
```

Keep one Go module and one Rust package initially. No monorepo orchestration framework is necessary. Add a future implementation-specific provider module only after choosing the actual provider. The generic provider boundary must not become several speculative backends.

## 21. Packaging and local development

Build Go and Rust release binaries independently, then package matching versions together. Proposed executable names are `spotify-tui` and `spotify-tui-playback`; verify they do not collide with an existing installation before adding anything to PATH.

The package should locate its paired helper from an explicit installation path or validated configuration. Include a version/doctor command to report binary versions, provider presence, audio availability, XDG paths, and protocol compatibility without exposing credentials.

Use the existing Arch package manager for actual native requirements after inspecting them. Do not instruct the user to pipe remote install scripts into a shell. Document runtime audio dependencies and any glibc/architecture constraints. Test the resulting artifacts on the user's machine, not just in a compiler environment.

If Soloist is chosen, link to official downloads and let the user obtain it; do not bundle its archive in our release. Surface build expiration early enough to update while online. Any automatic update mechanism is separate scope and must verify provenance.

The user subsequently authorized creating a GitHub repository and a download script. This planning handoff therefore includes a private repository, an initial documentation commit, and a planning release with a verified-download script. No system service, global audio reconfiguration, Spotify account-login browser session, package installation, or application implementation is included. A later build request still cannot make missing provider capabilities exist.

## 22. Local notes and environment-file workflow

The project folder was not a Git repository when first checked. After the user's repository request, a Git repository was initialized here and a private GitHub repository was created at `wrestle-R/Spotify-Tui`. The root `.gitignore` contains `/local-docs/` before local memory notes are created. Those notes are excluded from commits, release archives, and uploads.

Private continuation notes live in `local-docs/preferences.md`, `context.md`, and `tasks.md`. Never stage or upload them. If future work discovers tracked notes, explain that ignoring them does not remove prior history before taking any action.

No `.env` file is needed for this plan, and none has been created. Prefer non-secret TOML settings plus a keyring for normal operation. If future authorized work creates or changes a real environment file, first ensure it is ignored, confirm the actual GitHub remote and Varte configuration, inspect installed `varte` help, then run `varte push -f <path>` with explicit destination options as needed. Do not guess the destination, print secret values, or claim sync without CLI confirmation. Record any failed sync in private task notes by path and destination only. Placeholder `.env.example` files do not require secret sync.

## 23. Decisions, assumptions, and remaining questions

Proposed defaults: Rust owns playback/helper work, Go owns library/auth/UI; `q` detaches; `Q` stops; no autostart; read-only library; submit-to-search; finite disk cache; MPRIS after core verification. These may be adjusted without changing the confirmed product requirements.

Unanswered details to resolve when they matter:

- Which followed or Spotify-curated playlists must be playable? This affects API feasibility.
- Is initial one-time pairing through an official mobile client acceptable if subsequent use is independent?
- Should closing the TUI continue audio, or stop it? This plan proposes explicit detach/stop keys.
- Which terminal should be used for final acceptance and what numeric memory budget is acceptable?

## 24. What exists after this planning session

Delivered: this plan, README, acceptance checklist, dated source ledger, download guide, future release workflow specification, the simple terminal reference image, an actual planning-bundle download script, and ignored local continuation notes. The repository and planning release provide a downloadable handoff. Planning-distribution verification is separate from application verification.
