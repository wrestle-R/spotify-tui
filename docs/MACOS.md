# macOS setup and verification

Spotify TUI v0.4.0 targets **Apple Silicon (arm64), macOS 15 or later**. Intel Macs
are not supported by this release. The matching official Soloist download is listed
on [Spotify's platform downloads](https://developer.spotify.com/documentation/soloist/reference/downloads-and-updates).

## Install

Python 3.9+ is required for the installer/downloader. Go, Rust, GNU coreutils and
Linux `flock` are not required to use the downloaded release. Python is not needed
to run the installed terminal player. Use the portable download command in
[v0.4.0](releases/v0.4.0.md#downloads), then verify:

```bash
~/.local/bin/spotify-tui version
~/.local/bin/spotify-tui-playback --version
~/.local/bin/spotify-tui --demo
```

Both binaries must report 0.4.0. Add `~/.local/bin` to your shell PATH if you want
to run commands without the full path. The installer does not rewrite shell files.
It keeps previous build directories under `~/.local/lib/spotify-tui/` for rollback.

## Local audio and browser sign-in

Download and extract **Apple Silicon Mac** Soloist from Spotify's official download
page into a stable user-owned location. Do not select a Linux archive. The provider
is separate from this project's release and must not be redistributed with it.

Create your own Spotify Developer app and register
`http://127.0.0.1:43821/callback`. Then run:

```bash
~/.local/bin/spotify-tui setup --client-id YOUR_PUBLIC_CLIENT_ID --soloist-path /absolute/path/to/soloist
~/.local/bin/spotify-tui login
~/.local/bin/spotify-tui provider-key
~/.local/bin/spotify-tui
```

Login opens your default browser through macOS `open`. The provider-key input is
hidden, and Keychain stores the credentials. A Keychain refusal should be resolved
through its normal consent/settings; plaintext fallback remains explicit via
`setup --allow-file-token` and is not enabled automatically.

Initial pairing is separate: start the helper with Enter on a song or `connect`,
then select **Spotify TUI Local** in another Spotify client's device picker on the
same local network. Premium/provider setup is required. Do not put keys or tokens
in GitHub Actions secrets merely to run ordinary CI. Once paired, test with the
official desktop client closed and compare the TUI status with actual audio.

## Files and platform behaviour

Existing XDG environment overrides are honoured. Defaults remain `~/.config/spotify-tui`,
`~/.local/share/spotify-tui`, `~/.cache/spotify-tui` and `~/.local/state/spotify-tui`.
The short runtime socket directory uses the macOS temporary directory plus the
user's numeric UID, with owner-only permissions. No shared system daemon is added.

The terminal navigation is the same as Linux. `q` closes the terminal UI while
leaving audio running; `Q` stops it. `spotify-tui stop` stops the detached helper.
The Linux DBus tray is disabled on Mac. This release has no menu-bar companion.
Ctrl-U checks/installs the native stable update and restarts the matching pair.

## Signing and Gatekeeper

The CI build ad-hoc signs the native binaries, which is different from a Developer
ID signature and Apple notarization. This release is not notarized. Gatekeeper
and browser-download quarantine behaviour can differ from `curl` and runner tests.
No installer disables Gatekeeper, removes quarantine attributes or bypasses a
security warning. A notarized distribution needs a separate Developer ID/Apple
notary setup; see [Apple's workflow](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution).

## Verification boundaries

Required hosted runners: Linux x86_64 on Ubuntu 24.04 and native Apple Silicon on
macOS 15 and macOS 26. The release pipeline fails if any required job fails.

Automated checks cover Go race checks and native Rust tests; Go/Rust Unix IPC with
a fake provider endpoint; native binary architecture/version validation; archive
checksums, unsafe members and duplicate records; install/reinstall/old-build
retention/unrelated-file refusal; and a real PTY demo with navigation, search,
help, resize, quit, terminal restoration and no account files written. The official
provider's `--version` is checked without credentials and is not bundled in artifacts.

For a real Mac acceptance check, install the published archive, sign in normally,
pair and play a sample track. Pause/resume three times, seek, change tracks, quit
with `q`, reopen, stop with `Q`, and verify actual audio and displayed state agree.
Then restart cleanly with the official Spotify desktop app closed. Keep account
metadata and private log files out of public screenshots/issues.
