# Linux and Apple Silicon Mac downloads

For v0.5.1, use the native platform downloads and portable Python downloader in [the release guide](releases/v0.5.1.md). See [Mac requirements](MACOS.md). Both archives contain an installer and matching Go/Rust binaries. Python 3.9+ is required for installation only; macOS builds are ad-hoc signed, not notarized.

## Older Linux-only download path


The repository is public. Download the stable Linux x86_64 archive from [GitHub Releases](https://github.com/wrestle-R/Spotify-Tui/releases/latest). It needs glibc 2.39+, libgcc_s and separate Spotify/Soloist setup. Offline downloads are unsupported.

```bash
curl -fL https://github.com/wrestle-R/Spotify-Tui/releases/download/v0.3.1/download-linux.sh -o download-linux.sh
less download-linux.sh
bash download-linux.sh --destination ./spotify-tui-linux
bash ./spotify-tui-linux/install.sh
~/.local/bin/spotify-tui --demo
```

The downloader verifies the archive against SHA256SUMS before extraction, rejects unsafe archive paths and links, and never runs a binary. Use a new destination directory. You can pin a stable version with `--version 0.3.1`. GitHub CLI is only a fallback if the public HTTP request fails. Checksums are unsigned.

Installation retains old builds under `~/.local/lib/spotify-tui`, activates a matching binary pair through `current`, and creates command links in `~/.local/bin`. No sudo and no account data is installed. For a different prefix: `bash install.sh --prefix /your/prefix`. `--dry-run` makes no changes.

Future updates can be installed with Ctrl-U inside the player or `spotify-tui update`. Startup checks don't install anything by themselves. See [setup](SETUP.md#tray-and-updates).

For manual verification, download the archive and SHA256SUMS into an empty directory, then run `sha256sum --ignore-missing --check SHA256SUMS` before extracting it. Both binary version commands must report the chosen version. Never replace executables while they're running; the managed installer creates a new directory instead.

Source structure and build instructions are in [setup](SETUP.md), the [plan](../PLAN.md) and implementation notes. The source downloader can still use authenticated GitHub CLI, but public source archives are also available from GitHub.
