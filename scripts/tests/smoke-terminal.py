#!/usr/bin/env python3
"""Exercise the actual packaged demo through a PTY without credentials/network."""
import fcntl
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

binary=Path(sys.argv[1]).resolve()
version=subprocess.check_output([str(binary),'--version'],text=True).strip().split()[-1]
experience=tuple(map(int,version.split('.'))) >= (0,5,0)
hold=tuple(map(int,version.split('.'))) >= (0,5,1)
with tempfile.TemporaryDirectory(prefix='spotify-pty-') as temporary:
    env=dict(os.environ, HOME=temporary, TERM='xterm-256color')
    for key in ('XDG_CONFIG_HOME','XDG_DATA_HOME','XDG_CACHE_HOME','XDG_STATE_HOME','XDG_RUNTIME_DIR'):
        env[key]=str(Path(temporary)/key)
    master,slave=pty.openpty()
    fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',30,110,0,0))
    before=termios.tcgetattr(slave)
    process=subprocess.Popen([str(binary),'--demo'],stdin=slave,stdout=slave,stderr=slave,env=env,start_new_session=True)
    output=bytearray()
    def drain(seconds):
        deadline=time.monotonic()+seconds
        while time.monotonic()<deadline:
            if select.select([master],[],[],.05)[0]:
                try:output.extend(os.read(master,65536))
                except OSError:return
    try:
        drain(.6)
        for keys in (b'\x1b[B',b'\r',b'\t',b'/sample',b'\x1b',b'?'):
            os.write(master,keys);drain(.1)
        if experience:
            os.write(master,b'\x1b');drain(.1)
            for keys in (b'/Radiohead\r',b'/',b'\x1b[B',b'\r',b'/\x18',b'\x1b',b'i',b'\x1b[6~',b'\x1b[F'):
                os.write(master,keys);drain(.15)
            assert b'instructions' in output.lower(), 'instructions did not render'
            assert b'RECENT SEARCHES' in output, 'search history did not render'
            os.write(master,b'\x1b');drain(.1)
            for keys in (b'\x1b[1;5C',b'\x1b[1;5D',b'\x1b[<0;45;6M',b'\x1b[<0;45;6m'):
                os.write(master,keys);drain(.1)
        if hold:
            os.write(master,b'\x1b[?31u');drain(.1)
            os.write(master,b'\x1b[105;1:1u');drain(.2)
            assert b'Release to close' in output, 'held instructions did not open'
            os.write(master,b'\x1b[105;1:2u');drain(.1)
            os.write(master,b'\x1b[105;1:3u');drain(.2)
        fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',24,70,0,0))
        os.kill(process.pid,signal.SIGWINCH)
        drain(.15)
        os.write(master,b'\x1bq');drain(.4)
        if process.poll() is None:os.write(master,b'q')
        process.wait(timeout=5)
        assert process.returncode==0,output[-2000:]
        assert b'Spotify' in output or b'SPOTIFY' in output,output[-1000:]
        after=termios.tcgetattr(slave)
        assert before[3] & (termios.ECHO|termios.ICANON)==after[3] & (termios.ECHO|termios.ICANON),'terminal flags not restored'
        assert not any(Path(temporary).iterdir()),'demo wrote account/runtime files'
        print('Native terminal demo, navigation/search/help/resize/quit, terminal restoration and no account writes: PASS')
    finally:
        if process.poll() is None:process.kill();process.wait()
        os.close(master);os.close(slave)
