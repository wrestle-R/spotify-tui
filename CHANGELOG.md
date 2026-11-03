# What's new

Features and fixes, grouped by release series, newest first. Full details, upgrade instructions and validation live on each version's page. GitHub hosts the Linux and Apple Silicon Mac downloads.

**Latest stable: [v0.5.3](#v053)** · Stable releases only

[Download archive ↗](https://github.com/wrestle-R/spotify-tui-rdp/releases/tag/v0.5.3)

## 0.5.x

**Explore, proper next/previous and desktop media controls**

<a id="v053"></a>

### v0.5.3

3 November 2026 · **Stable** · Linux x86_64 and Apple Silicon macOS

- Open or focus the terminal player through MPRIS Raise and the tray's Open action.
- Reuse the existing player window on Hyprland, Sway and X11 with wmctrl.
- Serialize simultaneous open requests so repeated clicks during startup share one terminal.
- Register a desktop application entry and icon; its launcher follows managed updates.
- Return activation errors instead of silently ignoring them.

[Linux ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.3/spotify-tui_0.5.3_linux_amd64.tar.gz) · [Apple Silicon Mac ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.3/spotify-tui_0.5.3_darwin_arm64.tar.gz) · [Details →](docs/releases/v0.5.3.md)

<a id="v052"></a>

### v0.5.2

3 November 2026 · **Stable** · Linux x86_64 and Apple Silicon macOS

- Show only four compact shortcut groups in the hold-i modal, together without scrolling.
- Remove bottom key hints and keep the green player controls.
- Cache loaded views with their selection and scroll position; R forces a refresh.
- Render collection pages progressively in ordered 50-song batches, with two requests at a time.
- Load Explore's first songs without waiting for the whole library.
- Refresh incomplete provider metadata so desktop duration and artwork fill in correctly.

[Linux ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.2/spotify-tui_0.5.2_linux_amd64.tar.gz) · [Apple Silicon Mac ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.2/spotify-tui_0.5.2_darwin_arm64.tar.gz) · [Details →](docs/releases/v0.5.2.md)

<a id="v051"></a>

### v0.5.1

2 November 2026 · **Stable** · Linux x86_64 and Apple Silicon macOS

- Show a styled shortcut overlay while i is held; release or switch away to dismiss it.
- Group shortcuts into responsive columns with highlighted keys and clickable section jumps.
- Publish album artwork and album names through MPRIS; retain metadata across partial provider updates.
- Export complete media properties before announcing the player on D-Bus.

[Linux ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.1/spotify-tui_0.5.1_linux_amd64.tar.gz) · [Apple Silicon Mac ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.1/spotify-tui_0.5.1_darwin_arm64.tar.gz) · [Full details →](docs/releases/v0.5.1.md)

<a id="v050"></a>

### v0.5.0

2 November 2026 · **Stable** · Linux x86_64 and Apple Silicon macOS

- Open Explore first with Spotify top tracks, local rotation and liked-song fallback.
- Continue liked songs and playlists from the selected row, with next/previous owned by the background helper.
- Add Linux MPRIS media keys and playerctl controls, including metadata, position and volume.
- Put all instructions on i, with scrolling and plain footer labels.
- Save private search history and support mouse selection, history, scrolling and seeking.
- Keep Spotify Autoplay account-controlled, including after Explore and search tracks.

[Apple Silicon Mac ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.0/spotify-tui_0.5.0_darwin_arm64.tar.gz) · [Linux ↓](https://github.com/wrestle-R/spotify-tui-rdp/releases/download/v0.5.0/spotify-tui_0.5.0_linux_amd64.tar.gz) · [Full details →](docs/releases/v0.5.0.md)

## 0.4.x

**Spotify TUI on Apple Silicon Mac**

<a id="v040"></a>

### v0.4.0

29 October 2026 · **Stable** · Linux x86_64 and Apple Silicon macOS

- Add native macOS browser login, a short private socket path and native archive/update validation.
- Use a portable installer/downloader with checksums, safe extraction and matching Go/Rust activation.
- Gate releases on native Linux, macOS 15 and macOS 26 build/test/package/terminal checks.
- Publish the measured Linux RAM comparison, test conditions and sampler on the website.
- Add lightweight page motion with reduced-motion support and a pause control.
- Keep Soloist separate, the tray Linux-only, and real Mac account/audio verification explicitly unclaimed.

[Apple Silicon Mac ↓](https://github.com/wrestle-R/Spotify-Tui/releases/download/v0.4.0/spotify-tui_0.4.0_darwin_arm64.tar.gz) · [Linux ↓](https://github.com/wrestle-R/Spotify-Tui/releases/download/v0.4.0/spotify-tui_0.4.0_linux_amd64.tar.gz) · [Full details →](docs/releases/v0.4.0.md)

## 0.3.x

**Music in the tray, updates in the terminal**

<a id="v031"></a>

### v0.3.1

28 October 2026 · **Stable** · Linux x86_64

- Add the Next.js website under `next/`: glass logo, landing page, step-by-step account setup and a changelog with release detail pages.
- Keep playback restoration marked busy after an update, so keyboard requests cannot race the initial play/seek operation.
- Restore an up-to-date playback position and preserve the no-tray preference when restarting after an update.

[Download v0.3.1 ↓](https://github.com/wrestle-R/Spotify-Tui/releases/download/v0.3.1/spotify-tui_0.3.1_linux_amd64.tar.gz) · [Full release details →](docs/releases/v0.3.1.md) · [GitHub release ↗](https://github.com/wrestle-R/Spotify-Tui/releases/tag/v0.3.1)

<a id="v030"></a>

### v0.3.0

28 October 2026 · **Stable** · Linux x86_64

- Add a Linux tray with the approved music-note logo, playback controls and an action to reopen the TUI.
- Check stable releases on startup. Ctrl-U verifies and installs the update, then restarts the application.
- Fix pause/resume decisions using the helper's current state and wait for provider confirmation. Ignore older state snapshots.
- Publish the repository and support public downloads without GitHub login.

[Download v0.3.0 ↓](https://github.com/wrestle-R/Spotify-Tui/releases/download/v0.3.0/spotify-tui_0.3.0_linux_amd64.tar.gz) · [Full release details →](docs/releases/v0.3.0.md) · [GitHub release ↗](https://github.com/wrestle-R/Spotify-Tui/releases/tag/v0.3.0)

## 0.2.x

**A more comfortable terminal player**

Keyboard movement that follows the focused pane, a Spotify-green theme, generic demo music and a downloadable Linux package.

<a id="v021"></a>

### v0.2.1

28 October 2026 · **Stable** · Linux x86_64

**Complete release documentation in the Linux download**

- Include the changelog alongside the README and version detail pages in the extracted package.
- Keep matching TUI/helper versions and verify the complete stable download and installation path.

[Download v0.2.1 ↓](https://github.com/wrestle-R/Spotify-Tui/releases/download/v0.2.1/spotify-tui_0.2.1_linux_amd64.tar.gz) · [Full release details →](docs/releases/v0.2.1.md) · [GitHub release ↗](https://github.com/wrestle-R/Spotify-Tui/releases/tag/v0.2.1)

<a id="v020"></a>

### v0.2.0

28 October 2026 · **Stable** · Linux x86_64

**Intuitive navigation, Spotify colours and the first packaged Linux release**

- Move with arrows or j/k, return with Left/Esc, and open with Right/Enter. Paging stays in the focused pane; narrow terminals keep that pane visible.
- Enter starts the local player when needed. Clear request feedback, editable search and reconnect controls make everyday use easier.
- Try generic sample music with `--demo`, without signing in. Go and Rust now have their own source directories.

[Download v0.2.0 ↓](https://github.com/wrestle-R/Spotify-Tui/releases/download/v0.2.0/spotify-tui_0.2.0_linux_amd64.tar.gz) · [Full release details →](docs/releases/v0.2.0.md) · [GitHub release ↗](https://github.com/wrestle-R/Spotify-Tui/releases/tag/v0.2.0)

## Earlier planning material

The [planning-v0.1.0 bundle](https://github.com/wrestle-R/Spotify-Tui/releases/tag/planning-v0.1.0), published on 28 October 2026 in Asia/Calcutta, contains documents and a terminal concept. It is not an older player version. The first source implementation was subsequently published on main before the 0.2.x Linux release.

## Release verification

[Read the latest release details →](docs/releases/v0.5.3.md)
