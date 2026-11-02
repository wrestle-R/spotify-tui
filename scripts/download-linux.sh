#!/usr/bin/env bash
set -euo pipefail

repository='wrestle-R/spotify-tui-rdp'
version='0.5.1'
destination='./spotify-tui-linux'
dry_run=false
work_dir=''
fail() { printf 'Error: %s\n' "$*" >&2; exit 1; }
cleanup() { if [[ -n "$work_dir" && -d "$work_dir" ]]; then rm -rf -- "$work_dir"; fi; }
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) [[ $# -ge 2 ]] || fail '--version needs a version'; version=$2; shift 2 ;;
    --destination) [[ $# -ge 2 && -n "$2" ]] || fail '--destination needs a directory'; destination=$2; shift 2 ;;
    --dry-run) dry_run=true; shift ;;
    --help|-h)
      printf 'Download and verify the Linux x86_64 release. No install or execution.\nUsage: bash download-linux.sh [--version VERSION] [--destination NEW_DIRECTORY] [--dry-run]\nRequires curl, tar, sha256sum and GNU tools. Private repositories can use authenticated gh.\nTested on Arch Linux; Rust helper requires glibc 2.39 or later.\nSoloist and Spotify setup are separate. Offline downloads are unsupported.\n'
      exit 0 ;;
    *) fail "Unknown option: $1" ;;
  esac
done
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail 'Invalid version'
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || fail 'This download supports Linux x86_64 only; build source for other targets'
[[ ! -e "$destination" && ! -L "$destination" ]] || fail 'Destination exists; nothing overwritten'
archive="spotify-tui_${version}_linux_amd64.tar.gz"
root="spotify-tui_${version}_linux_amd64"
if [[ "$dry_run" == true ]]; then printf 'Would download %s from %s release v%s, verify SHA-256, and extract into %s.\n' "$archive" "$repository" "$version" "$destination"; exit 0; fi
for tool in curl tar sha256sum mktemp awk; do command -v "$tool" >/dev/null || fail "Required tool missing: $tool"; done
parent=$(dirname -- "$destination"); name=$(basename -- "$destination")
mkdir -p -- "$parent"; parent=$(cd -- "$parent" && pwd -P); destination="$parent/$name"
work_dir=$(mktemp -d -- "$parent/.spotify-tui-linux.XXXXXX")
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
base_url="https://github.com/$repository/releases/download/v$version"
if ! { curl --fail --location --silent --show-error --max-time 120 "$base_url/$archive" -o "$work_dir/$archive" && curl --fail --location --silent --show-error --max-time 30 "$base_url/SHA256SUMS" -o "$work_dir/SHA256SUMS"; }; then
 rm -f -- "$work_dir/$archive" "$work_dir/SHA256SUMS"
 command -v gh >/dev/null || fail 'Release download failed; check connectivity'
 gh release download "v$version" --repo "$repository" --pattern "$archive" --pattern SHA256SUMS --dir "$work_dir" || fail 'Release download failed; check repository access and connectivity'
fi
expected=$(awk -v name="$archive" '$2 == name {print $1}' "$work_dir/SHA256SUMS")
[[ "$expected" =~ ^[a-fA-F0-9]{64}$ ]] || fail 'Expected exactly one archive checksum'
printf '%s  %s\n' "$expected" "$archive" > "$work_dir/expected.sha256"
(cd -- "$work_dir" && sha256sum --check --strict expected.sha256) || fail 'Checksum mismatch; nothing extracted'
tar -tzf "$work_dir/$archive" > "$work_dir/entries"
while IFS= read -r entry; do
  case "$entry" in "$root"|"$root/"|"$root/"*) ;; *) fail 'Unexpected archive root' ;; esac
  case "/$entry/" in *'/../'*|*'/./'*) fail 'Unsafe archive path' ;; esac
done < "$work_dir/entries"
tar -tvzf "$work_dir/$archive" > "$work_dir/types"
awk 'substr($0,1,1)!="-" && substr($0,1,1)!="d" {exit 1}' "$work_dir/types" || fail 'Links or special archive entries are not allowed'
mkdir -- "$work_dir/unpacked"
tar -xzf "$work_dir/$archive" --directory "$work_dir/unpacked" --strip-components=1 --no-same-owner --no-same-permissions
for file in spotify-tui spotify-tui-playback install.sh README.md; do [[ -s "$work_dir/unpacked/$file" ]] || fail "Archive missing $file"; done
[[ ! -e "$destination" && ! -L "$destination" ]] || fail 'Destination appeared during download'
mv -Tn -- "$work_dir/unpacked" "$destination"
[[ ! -d "$work_dir/unpacked" ]] || fail 'Destination appeared during activation; nothing overwritten'
printf '\nVerified Linux download saved to %s\nRead README.md, inspect install.sh, then run: bash %q/install.sh\n' "$destination" "$destination"
printf 'No binary has been run. Spotify account and separate Soloist setup are required.\n'
