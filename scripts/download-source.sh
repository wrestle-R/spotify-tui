#!/usr/bin/env bash
set -euo pipefail

destination=""
source_ref=main
fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --destination) [[ $# -ge 2 && -n "$2" ]] || fail '--destination needs a directory'; destination=$2; shift 2 ;;
    --ref) [[ $# -ge 2 && -n "$2" ]] || fail '--ref needs a branch, tag, or full commit'; source_ref=$2; shift 2 ;;
    --help|-h)
      printf 'Download Spotify TUI development source, without installing or running it.\nUsage: bash download-source.sh --destination NEW_DIRECTORY [--ref COMMIT_OR_REF]\nRequires authenticated GitHub CLI access to wrestle-R/Spotify-Tui, git, and tar.\nDefault ref: main. Use a full commit SHA to retrieve a specific reviewed version.\n'
      exit 0 ;;
    *) fail "Unknown option: $1" ;;
  esac
done
[[ -n "$destination" ]] || fail 'Specify --destination NEW_DIRECTORY'
[[ "$source_ref" =~ ^[a-zA-Z0-9][a-zA-Z0-9._/-]*$ ]] || fail 'Invalid source ref'
[[ ! -e "$destination" && ! -L "$destination" ]] || fail 'Destination already exists; nothing overwritten'
for tool in gh git tar; do command -v "$tool" >/dev/null || fail "Required tool missing: $tool"; done
gh auth status --hostname github.com >/dev/null 2>&1 || fail 'Sign into GitHub CLI with repository access first'

parent=$(dirname -- "$destination")
name=$(basename -- "$destination")
mkdir -p -- "$parent"
parent=$(cd -- "$parent" && pwd -P)
destination="$parent/$name"
stage=$(mktemp -d -- "$parent/.spotify-tui-download.XXXXXX")
cleanup() { rm -rf -- "$stage"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
gh repo clone wrestle-R/Spotify-Tui "$stage/repository" -- --no-checkout --depth 1
git -C "$stage/repository" fetch --depth 1 -- origin "$source_ref"
commit=$(git -C "$stage/repository" rev-parse --verify 'FETCH_HEAD^{commit}')
if [[ "$source_ref" =~ ^[0-9a-fA-F]{40}$ ]]; then
  [[ "${commit,,}" == "${source_ref,,}" ]] || fail 'Requested commit mismatch'
fi
git -C "$stage/repository" fsck --no-dangling
mkdir -- "$stage/source"
git -C "$stage/repository" archive "$commit" | tar -xf - -C "$stage/source"
printf '%s\n' "$commit" > "$stage/source/DOWNLOAD-COMMIT"
[[ ! -e "$destination" && ! -L "$destination" ]] || fail 'Destination appeared during download; nothing overwritten'
mv -Tn -- "$stage/source" "$destination"
[[ ! -d "$stage/source" ]] || fail 'Destination appeared during activation; nothing overwritten'
printf 'Development source downloaded to %s\nConfirmed Git commit: %s\n' "$destination" "$commit"
printf 'No application was installed or executed. See docs/IMPLEMENTATION.md for test results.\n'
