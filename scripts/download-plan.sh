#!/usr/bin/env bash
set -euo pipefail

repository='wrestle-R/Spotify-Tui'
release_tag='planning-v0.1.0'
archive='spotify-tui-plan-v0.1.0.tar.gz'
destination='./Spotify-Tui-plan'
work_dir=''

usage() {
  cat <<'USAGE'
Download the Spotify TUI planning documents and verify their SHA-256 checksum.

Usage: bash download-plan.sh [--destination PATH] [--help]

The destination must not already exist. Requires bash, gh, tar, sha256sum,
mktemp, and an authenticated GitHub account with access to the private repo.
This downloads documentation and a design image. It does not install a player.
USAGE
}

fail() {
  printf 'Error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$work_dir" && -d "$work_dir" ]]; then
    rm -rf -- "$work_dir"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --destination)
      [[ $# -ge 2 && -n "$2" ]] || fail '--destination needs a path.'
      destination=$2
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      fail "Unknown option: $1. Use --help."
      ;;
  esac
done

for dependency in gh tar sha256sum mktemp awk; do
  command -v "$dependency" >/dev/null 2>&1 || fail "Missing dependency: $dependency"
done

[[ ! -e "$destination" && ! -L "$destination" ]] || fail "Destination already exists: $destination"
gh auth status >/dev/null 2>&1 || fail 'Sign in with gh auth login using an account with repository access.'

parent_dir=$(dirname -- "$destination")
mkdir -p -- "$parent_dir"
work_dir=$(mktemp -d -- "$parent_dir/.spotify-tui-plan.XXXXXX")
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

printf 'Downloading planning release %s from %s...\n' "$release_tag" "$repository"
gh release download "$release_tag" \
  --repo "$repository" \
  --pattern "$archive" \
  --pattern 'SHA256SUMS' \
  --dir "$work_dir" || fail 'Download failed. Check network, repository access, and release availability.'

[[ -s "$work_dir/$archive" && -s "$work_dir/SHA256SUMS" ]] || fail 'Release is missing the archive or checksum.'

expected_hash=$(awk -v filename="$archive" '$2 == filename { print $1 }' "$work_dir/SHA256SUMS")
[[ "$expected_hash" =~ ^[0-9a-fA-F]{64}$ ]] || fail 'Checksum file must contain one SHA-256 entry for the expected archive.'
printf '%s  %s\n' "$expected_hash" "$archive" > "$work_dir/expected.sha256"
(
  cd -- "$work_dir"
  sha256sum --check --strict expected.sha256
) || fail 'Checksum verification failed. Nothing was extracted.'

tar -tzf "$work_dir/$archive" > "$work_dir/archive-entries.txt" || fail 'Invalid archive.'
while IFS= read -r entry; do
  case "$entry" in
    Spotify-Tui-plan|Spotify-Tui-plan/|Spotify-Tui-plan/*) ;;
    *) fail 'Archive has an unexpected root path.' ;;
  esac
  case "/$entry/" in
    *'/../'*|*'/./'*) fail 'Archive contains an unsafe path.' ;;
  esac
done < "$work_dir/archive-entries.txt"

tar -tvzf "$work_dir/$archive" > "$work_dir/archive-types.txt" || fail 'Cannot inspect archive types.'
awk 'substr($0,1,1) != "-" && substr($0,1,1) != "d" { exit 1 }' "$work_dir/archive-types.txt" \
  || fail 'Archive contains a symlink or another unsupported entry type.'

mkdir -- "$work_dir/unpacked"
tar -xzf "$work_dir/$archive" \
  --directory "$work_dir/unpacked" \
  --strip-components=1 \
  --no-same-owner \
  --no-same-permissions
[[ -s "$work_dir/unpacked/PLAN.md" && -s "$work_dir/unpacked/README.md" ]] \
  || fail 'Extracted archive is missing the planning documents.'
[[ ! -e "$destination" && ! -L "$destination" ]] || fail 'Destination appeared during download; refusing to overwrite.'
mv -T --no-clobber -- "$work_dir/unpacked" "$destination"
[[ ! -d "$work_dir/unpacked" ]] || fail 'Destination could not be created safely.'

printf '\nVerified planning documents saved to: %s\n' "$destination"
printf 'Read %s/PLAN.md to start.\n' "$destination"
printf 'This is a planning release, not a working music player.\n'
