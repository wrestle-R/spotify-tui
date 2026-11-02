#!/usr/bin/env python3
"""Install the actual latest public stable release on a native Linux/Mac runner."""
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import tempfile

REPO = 'wrestle-R/spotify-tui-rdp'
ROOT = Path(__file__).resolve().parents[2]


def run(command, **kwargs):
    return subprocess.run(command, check=True, timeout=300, **kwargs)


def main():
    release = json.loads(subprocess.check_output(
        ['gh', 'release', 'view', '--repo', REPO, '--json', 'tagName,isDraft,isPrerelease'], text=True, timeout=30))
    tag = release['tagName']
    assert re.fullmatch(r'v\d+\.\d+\.\d+', tag) and not release['isDraft'] and not release['isPrerelease']
    version = tag[1:]
    with tempfile.TemporaryDirectory(prefix='public Spotify download ') as temp:
        root = Path(temp)
        run(['gh', 'release', 'download', tag, '--repo', REPO, '--pattern', 'download.py',
             '--pattern', 'SHA256SUMS', '--dir', str(root)])
        records = [line.split() for line in (root / 'SHA256SUMS').read_text().splitlines()]
        expected = [digest for digest, name in records if name == 'download.py']
        assert len(expected) == 1 and re.fullmatch('[a-f0-9]{64}', expected[0])
        assert hashlib.sha256((root / 'download.py').read_bytes()).hexdigest() == expected[0]
        source = root / 'verified download'
        print('Testing public', tag, 'on', platform.system(), platform.machine(), flush=True)
        run(['python3', str(root / 'download.py'), 'download', '--version', version, '--destination', str(source)])
        prefix = root / 'installed prefix'
        # The exact bundled install.sh linked by the website, with an isolated prefix.
        run(['bash', str(source / 'install.sh'), '--prefix', str(prefix)])
        env = dict(os.environ, HOME=str(root / 'empty home'))
        for name, flag in [('spotify-tui', 'version'), ('spotify-tui-playback', '--version')]:
            binary = prefix / 'bin' / name
            assert subprocess.check_output([str(binary), flag], env=env, text=True, timeout=15).strip() == name + ' ' + version
        run(['python3', str(ROOT / 'scripts/tests/smoke-terminal.py'), str(prefix / 'bin/spotify-tui')])
        print('Public checksum, native download, bundled install, matching versions and terminal walkthrough: PASS', flush=True)


if __name__ == '__main__':
    main()
