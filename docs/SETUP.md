# Platform note

Apple Silicon Mac users: follow [macOS setup](MACOS.md). Linux setup below uses the same account/provider flow. Soloist has a separate official Mac arm64 build. The Linux tray and namespace offline probe are not macOS features.

# Run the development build

This is the initial Go/Rust implementation. Offline Spotify downloads remain unsupported and the complete product is not accepted. Real account, local playback, and a clean online restart passed on the development machine; the isolated offline cold start failed. Fresh installations require your own account and provider setup. Automated tests use synthetic data.

## Build

From `/home/rdp/Desktop/code/Spotify-Tui`:

```bash
make build
./dist/spotify-tui --help
./dist/spotify-tui doctor
./dist/spotify-tui
```

Requires Go and Rust/Cargo. The build creates both executables in `dist/`. Credentials and runtime state live in private XDG directories, outside this source checkout. No `.env` file is needed.

## Browser sign-in for library and search

Use your normal browser for Spotify verification. Embedded browsers can fail verification and are not necessary for this application.

1. Open [Spotify Developer dashboard](https://developer.spotify.com/dashboard) in your usual browser and sign in there.
2. Create or use your own development app with Web API access. Complete any account/terms/verification steps yourself.
3. Register this exact redirect: `http://127.0.0.1:43821/callback`.
4. Allowlist your Spotify account if required by the dashboard.
5. Copy the public **client ID**, not the client secret.
6. Run setup and login:

```bash
./dist/spotify-tui setup --client-id YOUR_PUBLIC_CLIENT_ID
./dist/spotify-tui login
```

The app launches your default system browser through `xdg-open`. If you prefer opening the URL yourself:

```bash
./dist/spotify-tui login --no-browser
```

Keep that command running, open the printed URL in your normal browser on the same machine, and approve the requested read-only library/playlist access. Authorization returns to the local callback. The terminal confirms success only after the code exchange and credential save succeed. No password is entered into the TUI.

The callback expires after five minutes. If the port is occupied, the app reports it before opening a browser. You can repeat login after resolving the conflict. Do not paste callback codes, access tokens, refresh tokens, or a client secret into chat.

Credentials use the desktop keyring. If no usable keyring exists and you consciously accept plaintext owner-only files, enable `--allow-file-token` during setup. That fallback is not encryption. The default does not silently write plaintext tokens.

## Local playback through official Soloist

Web API sign-in above supplies library/search access. Soloist has separate pairing and an individual provider key.

1. Obtain the Linux x86_64 build from [Spotify's official Soloist downloads](https://developer.spotify.com/documentation/soloist/reference/downloads-and-updates). This repository does not redistribute that binary.
2. Put `soloist` on PATH or supply its absolute path during setup.
3. Generate your personal Soloist API key in Spotify's developer dashboard. Keep it private.
4. Save it through the hidden input:

```bash
./dist/spotify-tui provider-key
```

5. Launch the TUI and press `c`. The Rust helper starts Soloist with bounded playback cache and a loopback control interface.
6. Pair by selecting **Spotify TUI Local** from an official Spotify client's device list on the same local network. Close that official client afterward and verify local playback independently. Keep the same data directory for later restarts. Automatic restoration passed a clean online restart test after the startup/shutdown fixes.

If the device is missing when a phone supplies USB tethering, try Spotify desktop on the same computer for the pairing step. Discovery across tethering is not verified. Do not delete provider data or credentials as a first troubleshooting step.

The provider key is passed from Go to Rust over stdin. Soloist's documented startup interface uses a command-line key argument internally; same-user process inspection can expose that argument. Logs deliberately avoid printing it. If this exposure is unacceptable, do not configure this provider; the current integration has no verified alternate secret-input interface.

Soloist's loopback API has no built-in client authentication/origin protection. Our own Go/Rust interface uses a private Unix socket with same-user checks, but that does not add authentication to Soloist's separate loopback listener. This local integration limitation remains relevant when choosing the provider.

If verification fails in the normal browser too, complete Spotify's recovery or verification flow there. The app does not bypass verification or solve CAPTCHAs. If the developer dashboard itself is unavailable, account configuration and live playback testing remain blocked.

## Keyboard behavior

- Up/Down or `j/k` moves in the focused pane. Left/`h`/Esc returns to the library; Right/`l`/Enter opens a collection. Enter on a song plays it.
- Tab/Shift-Tab switches panes. Page Up/Down and Home/End act on the focused pane. On narrow terminals, that pane occupies the screen instead of hiding the library.
- `/` or Ctrl-F searches. Left/Right, Home/End, Delete and Backspace edit the query; Ctrl-G clears it, Ctrl-W deletes a word. Enter searches and Esc cancels without replacing your collection.
- Space pauses/resumes. `n/p` or Ctrl-Right/Left skips tracks; `[/]` or Alt-Left/Right seeks five seconds; `+/-` adjusts volume.
- `a` appends the selected song; `P` plays the open playlist from its start; `s` toggles shuffle and `r` cycles repeat.
- `R`/Ctrl-R refreshes the focused pane. `c` reconnects; `L` signs in through the browser; `?`/F1 opens help.
- Enter on a song connects the player automatically. Pending commands show feedback and prevent duplicate submissions while navigation remains available.
- `q` closes the TUI with audio continuing; `Q` stops the helper and exits; Ctrl-C exits. `spotify-tui stop` stops this project's helper from another terminal.
- `d` displays the unsupported offline capability. It cannot download songs.

Explore is the first collection and opens on startup. It uses Spotify top tracks with the `user-top-read` permission; press `L` once to grant that permission after upgrading. Existing sign-ins fall back to local listening history and liked songs. Select an Explore/search song to play it, then Spotify Autoplay supplies suggestions when enabled in your account. Soloist exposes no autoplay toggle, so enable it in Spotify settings. Existing Spotify queue entries retain their usual priority.

Liked songs and playlists continue from the selected row using a sequence owned by the Rust helper, including when the terminal is closed. `Ctrl-Right` / `n` skips forward and `Ctrl-Left` / `p` goes back (or restarts the current song after three seconds). Collection transitions can briefly buffer while Soloist changes context. A new selection replaces the local sequence; another Spotify controller can take over playback. A sequence accepts up to 5,000 playable tracks. `P` starts the native playlist context from its beginning. Search currently returns ten results.

Press `i`, `?` or F1 for the full scrollable instructions. Arrow/page keys and the mouse wheel scroll the instructions, and Esc returns. Click a collection to open it, a song to play it, a saved search to rerun it or the progress bar to seek. The player buttons support clicks too. Hold Shift for your terminal's normal text selection.

Search shows the last 30 distinct queries, stored locally with up to 100 played songs in the private state directory (`history.json`, mode 0600). Up/Down recalls queries; Ctrl-X clears search history; Ctrl-G clears only the input. Ctrl-arrows control playback while typing. Demo history never touches disk, and logout removes local history.

## Linux media keys and MPRIS

The player registers `org.mpris.MediaPlayer2.spotify_tui` on the session bus, independently of the tray. It works with `--no-tray` and continues after `q`. The desktop can use play/pause, next/previous, position, metadata, volume, shuffle and repeat. MPRIS is a Linux desktop integration; macOS does not gain media-key support in this release.

```bash
playerctl -p spotify_tui play-pause
playerctl -p spotify_tui next
playerctl -p spotify_tui previous
playerctl -p spotify_tui position 30
playerctl -p spotify_tui volume 0.5
```

`Q`, `stop`, logout and updates stop the media bridge. It needs the current user's D-Bus session and a paired local player. External controls report provider errors; a dispatched option change may still depend on the active Spotify context's available actions.

## Tests

```bash
make test
make check
SPOTIFY_TUI_TEST_HELPER="$PWD/dist/spotify-tui-playback" make test
```

The last command runs Go against the real compiled Rust helper with a deliberately unavailable provider. It verifies capabilities, communication, offline rejection, and shutdown without an account. Rust also tests its WebSocket bridge against a synthetic local provider. These are integration checks, not proof of Spotify audio or offline entitlement.

## Tray and updates

The normal TUI starts a pure-Go StatusNotifierItem companion automatically. No GTK or extra tray binary is required. It needs a desktop panel with a StatusNotifier watcher: for example Quickshell's tray on Hyprland. GNOME may require an AppIndicator extension. We do not change panel configuration. `spotify-tui doctor` reports host availability and whether the companion is running. `--no-tray` skips automatic startup.

Left-click opens a terminal TUI. The context menu offers Open, Pause/Resume, Next, Previous, Stop audio and quit tray, and Quit tray while keeping audio. Terminal choices are Kitty, Foot, Alacritty, Konsole, GNOME Terminal and xterm. `spotify-tui tray` runs the singleton companion; `tray-stop` stops it. `q` keeps it and audio running. `Q` stops both.

The TUI checks GitHub's latest stable release in the background at startup. It does not install automatically. Ctrl-U checks again or installs the offered update, verifies the archive SHA-256, ELF architecture and matching Go/Rust version, then restarts. Playing tracks are requested again with their prior position after helper restart, subject to Spotify connectivity. Playback can be interrupted while restarting. Ctrl-G clears a search field; Ctrl-U is reserved globally for updating.

Use `spotify-tui check-updates` or `spotify-tui update` outside the UI. Public releases need only network access; authenticated `gh` is a fallback for private repository access or API restrictions. Only Linux x86_64 and ordinary stable versions are accepted. Updates require a managed installation from `install.sh`, and refuse unrelated files or command symlinks. Old builds remain available for rollback. A single `lib/spotify-tui/current` pointer activates both binaries; installations and updates share a lock. Config, keyring and provider data are not modified. Archives are checksum-verified, not signed.
