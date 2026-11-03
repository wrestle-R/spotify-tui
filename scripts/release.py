#!/usr/bin/env python3
"""Portable Linux/macOS package, verified download and managed installation."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import platform
import re
import shutil
import struct
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

VERSION = "0.5.2"
REPOSITORY = "wrestle-R/spotify-tui-rdp"
BINARIES = ("spotify-tui", "spotify-tui-playback")
LIMIT = 128 * 1024 * 1024


def target():
    value = (platform.system(), platform.machine())
    if value == ("Linux", "x86_64"):
        return "linux_amd64"
    if value == ("Darwin", "arm64"):
        return "darwin_arm64"
    raise ValueError("Supported releases: Linux x86_64 and Apple Silicon macOS. Intel Mac Soloist is not available")


def valid_version(version):
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("Expected a stable version such as 0.5.2")
    return version


def executable(path):
    with Path(path).open("rb") as stream:
        header = stream.read(32)
    if target() == "darwin_arm64":
        if len(header) < 16 or header[:4] != b"\xcf\xfa\xed\xfe" or struct.unpack_from("<II", header, 4)[0] != 0x100000C or struct.unpack_from("<I", header, 12)[0] != 2:
            raise ValueError("Expected an Apple Silicon Mach-O executable")
    elif len(header) < 20 or header[:6] != b"\x7fELF\x02\x01" or struct.unpack_from("<H", header, 18)[0] != 62:
        raise ValueError("Expected a Linux x86_64 ELF executable")


def pair_version(source):
    versions = []
    for name in BINARIES:
        path = source / name
        if path.is_symlink() or not path.is_file():
            raise ValueError("Missing regular executable: " + name)
        executable(path)
        option = "version" if name == "spotify-tui" else "--version"
        result = subprocess.check_output([str(path.resolve()), option], timeout=10, text=True).strip()
        if not result.startswith(name + " "):
            raise ValueError("Invalid version response: " + name)
        versions.append(valid_version(result[len(name) + 1:]))
    if versions[0] != versions[1]:
        raise ValueError("Go/Rust binary versions differ")
    return versions[0]


def package(version, project):
    valid_version(version)
    if pair_version(project / "dist") != version:
        raise ValueError("Build both binaries with the requested version first")
    name = "spotify-tui_" + version + "_" + target()
    release = project / "dist/release"
    release.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".package-", dir=project / "dist") as temp:
        root = Path(temp) / name
        root.mkdir()
        for binary in BINARIES:
            shutil.copy2(project / "dist" / binary, root / binary)
        shutil.copy2(project / "scripts/install-local.sh", root / "install.sh")
        shutil.copy2(project / "scripts/release.py", root / "release.py")
        for file in ("README.md", "PLAN.md", "CHANGELOG.md"):
            shutil.copy2(project / file, root / file)
        for directory in ("docs", "design"):
            shutil.copytree(project / directory, root / directory, ignore=shutil.ignore_patterns("__pycache__"))
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=project, text=True).strip()
        runtime = {"version": version, "target": target(), "commit": commit,
                   "provider": "Download the matching official Spotify Soloist separately; never bundled",
                   "tray": "Linux StatusNotifier only",
                   "macos_signing": "ad-hoc signed, not Developer ID signed or notarized" if target().startswith("darwin") else None}
        (root / "RUNTIME.json").write_text(json.dumps(runtime, indent=2) + "\n")
        (root / "RUNTIME.txt").write_text("Linux requires glibc >=2.39; macOS release requires Apple Silicon and macOS 15 or later.\nPython 3.9+ is required for the installer/downloader only.\nSoloist, Spotify Premium and account setup are separate.\nMac tray unavailable. macOS binaries are ad-hoc signed, not notarized.\n")
        archive = release / (name + ".tar.gz")
        with tarfile.open(archive, "w:gz") as output:
            output.add(root, arcname=name)
    shutil.copy2(project / "scripts/release.py", release / "download.py")
    shutil.copy2(project / "scripts/download-linux.sh", release / "download-linux.sh")
    checksums = []
    for file in sorted(release.iterdir()):
        if file.name.endswith(".tar.gz") or file.name in ("download.py", "download-linux.sh"):
            checksums.append(hashlib.sha256(file.read_bytes()).hexdigest() + "  " + file.name)
    (release / "SHA256SUMS").write_text("\n".join(checksums) + "\n")
    print("Packaged", archive)
    return archive


def replace_link(target_path, link):
    temporary = link.parent / ("." + link.name + "." + str(os.getpid()))
    try:
        temporary.symlink_to(target_path)
        os.replace(temporary, link)
    finally:
        temporary.unlink(missing_ok=True)


def install(source, prefix, dry_run=False):
    source = source.expanduser().resolve()
    prefix = prefix.expanduser().resolve()
    version = pair_version(source)
    if dry_run:
        print("Would install", version, "under", prefix)
        return
    app = prefix / "lib/spotify-tui"
    app.mkdir(parents=True, exist_ok=True)
    (prefix / "bin").mkdir(parents=True, exist_ok=True)
    with (app / ".update.lock").open("a") as lock:
        os.chmod(lock.name, 0o600)
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        for name in BINARIES:
            link = prefix / "bin" / name
            if link.exists() or link.is_symlink():
                if not link.is_symlink() or not link.resolve().is_relative_to(app) or link.resolve().name != name:
                    raise ValueError("Refusing unrelated installed file: " + str(link))
        current = app / "current"
        if current.exists() or current.is_symlink():
            if not current.is_symlink() or not current.resolve().is_relative_to(app):
                raise ValueError("Invalid current build pointer")
        elif (prefix / "bin/spotify-tui").is_symlink():
            old = [(prefix / "bin" / name).resolve().parent for name in BINARIES]
            if old[0] != old[1]:
                raise ValueError("Existing Go/Rust installation is inconsistent")
            current.symlink_to(old[0])
        stage = Path(tempfile.mkdtemp(prefix="build." + version + "-", dir=app))
        activated = False
        try:
            for name in BINARIES:
                shutil.copy2(source / name, stage / name)
                (stage / name).chmod(0o755)
            # Bootstrapping the pointer first keeps both links resolvable.
            if not current.is_symlink():
                current.symlink_to(stage)
                activated = True
            for name in BINARIES:
                replace_link(current / name, prefix / "bin" / name)
            replace_link(stage, current)
            activated = True
        finally:
            if not activated:
                shutil.rmtree(stage)
    print("Installed", version, "in", stage)
    print("Run:", prefix / "bin/spotify-tui")
    print("Previous builds retained. Account/keyring/provider data unchanged.")


def verify_checksum(archive, sums):
    if sums.stat().st_size > 1024 * 1024:
        raise ValueError("Oversized checksum file")
    matches = [line.split()[0] for line in sums.read_text().splitlines()
               if len(line.split()) == 2 and line.split()[1].lstrip("*") == archive.name]
    if len(matches) != 1 or not re.fullmatch(r"[a-fA-F0-9]{64}", matches[0]):
        raise ValueError("Missing or duplicate archive checksum")
    if archive.stat().st_size > LIMIT or hashlib.sha256(archive.read_bytes()).hexdigest() != matches[0].lower():
        raise ValueError("Checksum mismatch or oversized archive")


def extract(archive, destination, root):
    seen = set()
    with tarfile.open(archive, "r:gz") as source:
        members = []
        total = 0
        for member in source:
            if len(members) >= 2000:
                raise ValueError("Too many archive members")
            members.append(member)
            path = PurePosixPath(member.name)
            if member.name.rstrip("/") != str(path) or path.is_absolute() or ".." in path.parts or path.parts[0] != root:
                raise ValueError("Unsafe archive path")
            if not (member.isdir() or member.isfile()) or member.name in seen:
                raise ValueError("Links, duplicate or special archive members are not allowed")
            seen.add(member.name)
            total += member.size
            if member.size < 0 or member.size > 64 * 1024 * 1024 or total > 200 * 1024 * 1024:
                raise ValueError("Unpacked release too large")
        for member in members:
            relative = PurePosixPath(member.name).parts[1:]
            if not relative:
                continue
            path = destination.joinpath(*relative)
            if member.isdir():
                path.mkdir(parents=True, exist_ok=True)
            else:
                path.parent.mkdir(parents=True, exist_ok=True)
                with source.extractfile(member) as incoming, path.open("xb") as outgoing:
                    shutil.copyfileobj(incoming, outgoing)
                path.chmod(0o755 if path.name in (*BINARIES, "install.sh") else 0o644)


def fetch(url, path, limit=LIMIT):
    request = urllib.request.Request(url, headers={"User-Agent": "Spotify-Tui-downloader"})
    with urllib.request.urlopen(request, timeout=120) as response, path.open("wb") as output:
        remaining = limit
        while True:
            data = response.read(min(1024 * 1024, remaining + 1))
            if not data:
                break
            remaining -= len(data)
            if remaining < 0:
                raise ValueError("Download exceeds size limit")
            output.write(data)


def download(version, destination, dry_run=False):
    valid_version(version)
    name = "spotify-tui_" + version + "_" + target()
    destination = destination.expanduser().absolute()
    if destination.exists() or destination.is_symlink():
        raise ValueError("Destination exists; nothing overwritten")
    if dry_run:
        print("Would download and verify", name + ".tar.gz", "to", destination)
        return
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".spotify-download-", dir=destination.parent) as temp:
        work = Path(temp)
        archive = work / (name + ".tar.gz")
        sums = work / "SHA256SUMS"
        try:
            fetch("https://github.com/" + REPOSITORY + "/releases/download/v" + version + "/" + archive.name, archive)
            fetch("https://github.com/" + REPOSITORY + "/releases/download/v" + version + "/SHA256SUMS", sums, 1024 * 1024)
        except (OSError, ValueError):
            if not shutil.which("gh"):
                raise
            archive.unlink(missing_ok=True)
            sums.unlink(missing_ok=True)
            subprocess.run(["gh", "release", "download", "v" + version, "--repo", REPOSITORY,
                            "--pattern", archive.name, "--pattern", "SHA256SUMS", "--dir", str(work)], check=True)
        verify_checksum(archive, sums)
        unpacked = work / "unpacked"
        unpacked.mkdir()
        extract(archive, unpacked, name)
        for required in (*BINARIES, "install.sh", "release.py", "README.md"):
            if not (unpacked / required).is_file():
                raise ValueError("Incomplete archive: " + required)
        # Reserve a new private destination rather than replacing a racing directory.
        destination.mkdir(mode=0o700)
        try:
            shutil.copytree(unpacked, destination, dirs_exist_ok=True)
        except BaseException:
            shutil.rmtree(destination)
            raise
    print("Verified download saved to", destination)
    print("Inspect install.sh and release.py, then run: bash", destination / "install.sh")
    print("No binary was executed. SHA-256 checks integrity, not a publisher signature.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    pack = commands.add_parser("package")
    pack.add_argument("--version", default=VERSION)
    pack.add_argument("--project", type=Path, default=Path(__file__).resolve().parents[1])
    setup = commands.add_parser("install")
    setup.add_argument("--from", dest="source", type=Path, default=Path(__file__).resolve().parent)
    setup.add_argument("--prefix", type=Path, default=Path.home() / ".local")
    setup.add_argument("--dry-run", action="store_true")
    down = commands.add_parser("download")
    down.add_argument("--version", default=VERSION)
    down.add_argument("--destination", type=Path, default=Path("spotify-tui-download"))
    down.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    try:
        if args.command == "package":
            package(args.version, args.project.resolve())
        elif args.command == "install":
            install(args.source, args.prefix, args.dry_run)
        else:
            download(args.version, args.destination, args.dry_run)
    except (OSError, ValueError, subprocess.SubprocessError, tarfile.TarError) as error:
        print("Error:", error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
