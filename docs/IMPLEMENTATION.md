# Implementation and test status

## Linux release: 0.2.1

- Reworked keyboard focus and movement: Up/Down or j/k, Left/h/Esc for library, Right/l/Enter to open, Tab to switch. Paging and Home/End move the focused pane and preserve its scroll position.
- Narrow terminals display the focused pane rather than hiding the library. Selection/focus markers work without colour; Spotify-green selection and charcoal background provide the colour theme.
- Search supports cursor editing, Unicode delete/backspace, Ctrl-U/Ctrl-W, paste and cancel. Pasted text outside a field cannot execute shortcuts.
- Enter on a song starts/connects the helper, waits for readiness and sends playback. Pending requests provide feedback, suppress duplicates and allow navigation. Quit cancels pending work. Reconnect now actually restarts a stuck/unpaired helper.
- Loading, empty, unavailable-item and provider/account errors have separate visible states. Search refresh retains its query; delayed collection/account results are rejected.
- `--demo` uses generic local sample songs and playlists without authentication, provider startup, network access or audio. The README screenshot is a capture of that mode. Sample durations are fixtures, not catalog metadata.
- Code now lives under `go/` and `rust/`, with root `scripts/`, `protocol/`, `docs/` and `design/`. Root Make targets handle both languages.
- Linux x86_64 release packaging includes both binaries and a user-prefix installer; a checksum-verifying downloader handles private GitHub Releases. The Rust helper needs glibc >=2.39. Go is built with CGO disabled.

Verification: keyboard regression tests and layout bounds, Go race/vet/format, Rust tests/fmt/clippy and actual Go/Rust protocol pass after restructuring. A real-account PTY walkthrough passed collection navigation, paging, search, narrow resize, automatic startup/playback, pause and terminal restoration. A later repetition encountered an external Spotify connectivity timeout while searching; the UI displayed the connection failure. Another cold-start repetition hit Soloist's intermittent saved-session login problem. The account restored after restart and the standalone live playback/control check passed again. This does not establish perfect network/provider recovery. The generic demo was also exercised with empty XDG paths and wrote no account/runtime files. Linux package checks passed valid extraction, corrupt checksum refusal, traversal/link refusal, existing-destination protection, temporary cleanup, installation/upgrade and both version commands with paths containing spaces. Run `make package-check` to reproduce the package checks.

## Implemented

- Go terminal interface using Bubble Tea and Lip Gloss, with library focus, track lists, keyboard search, playback status, resize handling, and terminal restoration.
- Browser Authorization Code with PKCE, exact loopback redirect, callback state validation, expiry/refresh handling, and coordinated refreshes. `login --no-browser` supports completing verification in an ordinary browser.
- Desktop-keyring storage. An owner-only plaintext fallback requires explicit configuration. The Web API client secret is not used.
- Real Spotify profile, playlist listing, owned/collaborative playlist items, liked songs with pagination, and initial catalog search results.
- Rust playback helper, private Unix-socket protocol, same-user credential checks, bounded command/frame limits, and separate provider-state confirmation.
- Official Soloist process startup with persistent data, saved-session restoration on a clean online restart, loopback WebSocket control, pause/resume, individual-track and playlist-context playback, seek, queue append, shuffle/repeat controls, and volume commands.

## Verified on the actual machine

- Go and Rust release builds.
- Go tests with race detector and vet; Go formatting.
- Rust tests, formatting, and clippy with warnings denied.
- Go calling the actual compiled Rust helper over a real Unix socket, including unsupported-download rejection and shutdown.
- Rust WebSocket bridge tested against a local synthetic provider; command acceptance is not misreported as confirmed playback.
- Actual Spotify browser authorization completed by the user in their normal browser. Tokens saved in the desktop keyring.
- Actual account identity, playlist listing, an owned/collaborative playlist's items, liked songs, and catalog search.
- Actual TUI displaying the real library, receiving search input, loading results, and restoring terminal settings on exit.
- Fast combined search input regression found in the real terminal smoke test and fixed.
- Official Soloist Linux binary downloaded from Spotify and executed locally: `1.3.10.21`, Linux x86_64, build dated 27 October 2026. The binary remains in ignored local work storage and is not redistributed by this repository.
- User completed one-time device pairing, then confirmed official desktop/web/mobile clients were closed.
- Local full-track playback state, advancing playback clock, pause, resume, seek, and volume-command acceptance.
- Actual running PipeWire output stream, traced through its client to the `soloist` executable. Soloist labels the stream “Spotify”, so checking only for a “Soloist” stream label misses it.
- Local installer dry run, installation and upgrade into an isolated project-work prefix, both installed version commands, and refusal to overwrite unrelated files/symlinks.
- Rebuilding while the helper is running: atomic executable replacement avoids Linux's text-file-busy failure.
- Seven Rust tests, including stale endpoint cleanup that preserves session files.
- Clean helper/provider stop and start followed by restored paired account, full-track state, progress, pause/resume and seek with Spotify desktop closed.
- Real shuffle and context/track repeat changes confirmed from provider state, with the user's original options restored afterward. Repeat-state parsing was corrected to match Soloist's documented string field.

Build toolchain on the tested Arch x86_64 machine: Go `1.27.1-X:nodwarf5`, Rust `1.99.0`, Cargo `1.99.0`. These identify the tested environment, not a tested minimum-version matrix.

## Restart fixes and limits

A live restart exposed stale provider endpoint files after abrupt termination. Startup now clears only `ws.addr` and `ws.port`, and shutdown gives Soloist time to exit gracefully. A subsequent startup initially reported unpaired, and the device was absent from both phone and desktop pickers while the phone provided USB tethering. Private provider diagnostics then confirmed successful restoration from the existing stored session. The normal helper restored pairing without another manual pairing, and a subsequent clean stop/start and playback test passed. The transient unpaired episode's full cause is not established; these results do not prove every failure/reconnection mode. No user network settings were changed.

## Remaining work and limits

- Broader crash/network-recovery reliability beyond the successful clean online restart remains unverified.
- Full liked-song sequential queue and starting a playlist at an arbitrary selected occurrence: not implemented. `Enter` plays an individual track; `P` plays the open playlist context from its start; `a` appends a track to the provider queue.
- Catalog search currently returns the first 10 results; search paging remains to be implemented.
- Followed/curated playlist item enumeration can be restricted by Spotify. A restricted playlist may still be playable as a provider context using `P`, but that is not a claim that its rows can be listed.
- Metadata caching uses JSON files in this first build; SQLite and a global disk-cache eviction policy from the original plan are deferred. Config is non-secret JSON rather than TOML to avoid an extra parser in the initial implementation.
- MPRIS/media keys, eight-hour stability testing, crash recovery under every termination mode, and complete upgrade/uninstall acceptance remain outstanding.
- Resource targets in PLAN.md are still targets until measured under the stated workload. Any short sample is labeled separately and does not establish long-term RAM savings.

## Short memory sample (historical, before the paired comparison)

With the terminal UI open and the local provider paired but paused, a single `/proc/*/smaps_rollup` sample measured **93.8 MiB combined PSS**: Go TUI 34.7 MiB, Rust helper 1.9 MiB, Soloist main 47.2 MiB and its crash handler 10.1 MiB. Rounding can make component totals differ by 0.1 MiB. This excludes the terminal/browser, is not a playing or eight-hour workload, and has no equivalent official Spotify baseline. No measured savings claim is made.

## Run it

```bash
cd /home/rdp/Desktop/code/Spotify-Tui
make build
./dist/spotify-tui
```

See [SETUP.md](SETUP.md) for fresh-machine setup. This machine's public client ID/provider path are in its private XDG config; credentials are in the keyring, not the repository. No real `.env` file was created.

Local install, if wanted:

```bash
bash scripts/install-local.sh --dry-run
bash scripts/install-local.sh
```

The script installs both built binaries in a version directory under a user prefix and creates paired command links. It refuses to replace unrelated existing files or symlinks. It does not install Soloist, log into Spotify, edit shell settings, or change global audio settings.

## Stable tray/update acceptance, 28 October 2026

The repository is public. v0.3.0 and v0.3.1 are ordinary stable GitHub releases, not drafts or previews. Published assets were not overwritten. Main subsequently adds the requested separate headphone hero to the website; the app's release binaries are unchanged by that visual revision.

The live checks below used GitHub-downloaded, installed binaries under `~/.local`, never `dist`. v0.3.0 was downloaded from its GitHub draft for acceptance, then published. v0.3.1 was installed through the actual stable-release updater.

- The actual Quickshell StatusNotifier watcher on Hyprland registered `Spotify TUI`. SNI properties and all six dbusmenu actions were queryable over the session bus. Icon fixtures validate ARGB pixmaps at tray sizes and the required menu wire signature. No panel or network configuration changed.
- The installed v0.3.1 tray Activate action opened Kitty running the installed TUI. Verified the TUI executable path and its terminal parent; Kitty reparents itself, so it is not retained under the tray process tree.
- Installed v0.3.0 loaded the real library, played a selected song, completed three Space pause/resume cycles with matching Playing/Paused labels, and reflected tray-menu pause/resume in the terminal. `q` preserved the tray/helper and restored terminal echo/canonical settings.
- The installed real-account playback check confirmed full-track state, advancing position, pause, resume and seek. The volume request was accepted at the existing level.
- Installed v0.3.0 detected stable v0.3.1 on startup. A real Ctrl-U downloaded and verified the release, activated the matching Go/Rust pair and exec'd the new installed executable in the same process. Both version commands reported 0.3.1. A new helper instance restored the playing track; Space pause/resume and terminal cleanup passed afterward. The app then reported it was up to date. No test version override or fake release endpoint was used for this live upgrade.
- A separate public download of v0.3.1 passed SHA-256 for archive and downloader. Its two executable bytes matched the installed pair. One HTTP transfer timed out; the downloader's authenticated GitHub CLI fallback recovered and verification passed. A subsequent direct public transfer completed.
- Installed v0.3.1 demo passed in a real terminal with empty XDG paths, generic sample songs, no audio and no account/runtime writes. Final doctor reported tray host/running, helper connected and paired. Audio was left paused.
- Go race tests, formatting/vet, eight Rust tests, fmt/clippy and package-check passed. Regression checks cover confirmed and queued toggles, stale polls, Ctrl-U in search, restoration's busy state, stable version ordering, corruption, unsafe archives, unrelated installations, activation and rollback retention.
- Next.js production export/type checking passed for home, docs, changelog and all four stable release pages. Desktop/mobile browser walkthroughs checked image loading, width bounds, navigation, release details and copy feedback. The browser clipboard connector did not consistently expose read-back; the button reported successful copying and the rendered commands were checked separately. Production dependency audit reported zero known vulnerabilities at the time of testing. The site is local/static source, not a deployed public website.


## Native macOS and release checks (29 October 2026)

[Run 40746914309](https://github.com/wrestle-R/Spotify-Tui/actions/runs/40746914309) passed Linux x86_64, native Apple Silicon macOS 15 and macOS 26, and the production website build. Both Mac jobs passed Go race checks, Rust tests, real Go/Rust socket checks with a fake provider endpoint, native signed executable/version checks, verified packaging/install/reinstall tests, an installed terminal demo and the official provider version probe. Real Mac account authorization, Keychain consent and audible playback remain unverified; see [MACOS.md](MACOS.md).

The Linux v0.4.0 artifact from that run was downloaded, checksum-verified and installed into a private managed prefix for the real-account RAM comparison. [MEMORY.md](MEMORY.md) records the complete process scope and limitations; its aggregate JSON contains no account data.

After sampling, the downloaded native Linux candidate passed the real-account full-track progress, pause, resume, seek and volume checks with the temporary official GUI fully closed. Playback was left paused. The production website passed TypeScript/static export, methodology navigation and mobile overflow checks at 390px and 320px. The animation control changed its pressed label, paused both hero and footer CSS animations, and exposed all reveal content. Reduced-motion CSS disables transforms/reveals and animations; OS preference emulation was not available through the browser connector.
