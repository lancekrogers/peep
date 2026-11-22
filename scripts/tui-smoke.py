#!/usr/bin/env python3
"""Real PTY acceptance tests. Uses only Python's standard library and synthetic media."""
import errno
import fcntl
import os
from pathlib import Path
import pty
import re
import select
import signal
import struct
import sys
import subprocess
import termios
import time

ROOT = Path(__file__).resolve().parents[1]
ARTIFACTS = ROOT / 'tmp' / 'acceptance'
ARTIFACTS.mkdir(parents=True, exist_ok=True)

class Session:
    def __init__(self, name, *args, env_override=None):
        self.name = name
        self.master, self.slave = pty.openpty()
        self.original = termios.tcgetattr(self.slave)
        self.resize(110, 30, notify=False)
        env = dict(os.environ, TERM='xterm-256color', COLORTERM='truecolor')
        if env_override is not None: env.update(env_override)
        def terminal():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)
        child = "import subprocess,sys,termios; before=termios.tcgetattr(0); p=subprocess.run(sys.argv[1:]); after=termios.tcgetattr(0); mask=~getattr(termios,'PENDIN',0); before[3]&=mask; after[3]&=mask; restored=after==before; print('PEEP_TERMINAL_RESTORED='+str(restored),flush=True); sys.exit(p.returncode if restored else 99)"
        self.proc = subprocess.Popen([sys.executable, '-c', child, str(ROOT/'bin/peep'), '--audio-output=null', '--renderer=cells', *args],
            cwd=ROOT, stdin=self.slave, stdout=self.slave, stderr=self.slave,
            env=env, preexec_fn=terminal)
        self.log = bytearray()
        self.children = set()
    def resize(self, cols, rows, notify=True):
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack('HHHH', rows, cols, 0, 0))
        if notify: os.kill(self.proc.pid, signal.SIGWINCH)
    def read(self, seconds=.2):
        out = bytearray()
        until = time.monotonic()+seconds
        while time.monotonic()<until:
            if select.select([self.master], [], [], max(0, until-time.monotonic()))[0]:
                try: data=os.read(self.master, 262144)
                except OSError as exc:
                    if exc.errno==errno.EIO: break
                    raise
                if not data: break
                out.extend(data); self.log.extend(data)
        return bytes(out)
    def expect(self, needle, timeout=8):
        out=bytearray();until=time.monotonic()+timeout
        while time.monotonic()<until:
            out.extend(self.read(.1))
            if needle in re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]',b'',out): return bytes(out)
            if self.proc.poll() is not None: break
        raise AssertionError(f'{self.name}: missing {needle!r}; tail={bytes(out[-1800:])!r}')
    def send(self, keys):
        self.read(.15)
        os.write(self.master, keys)
    def record_children(self):
        rows=subprocess.check_output(['ps','-axo','pid=,ppid=']).decode().splitlines()
        edges=[tuple(map(int,line.split())) for line in rows]
        parents={self.proc.pid}
        for _ in range(5):
            parents.update(pid for pid,ppid in edges if ppid in parents)
        self.children |= parents-{self.proc.pid}
    def finish(self):
        self.record_children()
        self.send(b'q')
        self.read(.7)
        self.proc.wait(timeout=5)
        (ARTIFACTS/(self.name+'.ansi')).write_bytes(self.log)
        assert self.proc.returncode==0, bytes(self.log[-1800:])
        assert b'\x1b[?1049l' in self.log, 'alternate screen not restored'
        assert b'PEEP_TERMINAL_RESTORED=True' in self.log, 'terminal attributes not restored'
        for pid in self.children:
            try: os.kill(pid,0)
            except ProcessLookupError: continue
            raise AssertionError(f'child process {pid} survived exit')
        (ARTIFACTS/(self.name+'.ansi')).write_bytes(self.log)
        os.close(self.master);os.close(self.slave)
    def abort(self):
        if self.proc.poll() is None:
            try:
                os.write(self.master, b'\x03')
                self.read(1)
                self.proc.wait(timeout=3)
            except (OSError, subprocess.TimeoutExpired):
                self.record_children()
                for pid in self.children:
                    try: os.kill(pid, signal.SIGTERM)
                    except (ProcessLookupError, PermissionError): pass
                self.proc.kill()
                self.proc.wait(timeout=3)
        (ARTIFACTS/(self.name+'-failure.ansi')).write_bytes(self.log)

sessions=[]
try:
    s=Session('video','--play','tmp/fixtures/03-video.mp4');sessions.append(s)
    s.expect(b'[Playing]');s.expect(b'00:01 / 00:08')
    assert b'\x1b[38;2;' in s.log, 'video pixels missing'
    s.send(b' ');s.expect(b'[Paused]')
    s.send(b']');s.expect(b'00:06 / 00:08')
    s.send(b'[');s.expect(b'00:01 / 00:08')
    s.send(b't');s.expect(b'TIMELINE');s.send(b'\x1b[C');s.expect(b'00:06 / 00:08');s.send(b'\x1b[D');s.expect(b'00:01 / 00:08');s.send(b't')
    s.send(b'\x1b[<0;56;26M');s.expect(b'00:04 / 00:08');s.send(b'\x1b[<0;56;26m')
    s.send(b'?');s.expect(b'BROWSE');s.send(b'?')
    s.resize(70,20);s.read(.5);s.resize(110,30);s.read(.5)
    s.send(b's');s.expect(b'[Stopped] 00:00')
    s.send(b' ');s.expect(b'[Playing]');s.finish()
    s=Session('jpeg','tmp/fixtures/06-photo.jpg');sessions.append(s)
    s.expect(b'\xe2\x96\x80');s.send(b'f');s.read(.2);s.send(b'\x1b');s.expect(b'Files');s.finish()
    s=Session('image','--play','tmp/fixtures/01-image.png');sessions.append(s)
    s.expect(b'[Playing]');s.send(b' ');s.expect(b'[Paused]')
    assert b'\x1b[38;2;' in s.log, 'image pixels missing'
    s.send(b's');s.expect(b'[Stopped] 00:00');s.finish()
    s=Session('gif','--play','tmp/fixtures/02-animation.gif');sessions.append(s)
    s.expect(b'[Playing]');s.expect(b'00:01 / 00:03');s.send(b' ');s.expect(b'[Paused]');s.finish()
    s=Session('audio','--play','tmp/fixtures/04-audio.wav');sessions.append(s)
    s.expect(b'[Playing]')
    if b'AUDIO SPECTRUM' not in s.log:s.expect(b'AUDIO SPECTRUM')
    s.expect(b'00:01 / 00:08')
    (ARTIFACTS/'audio-playing.ansi').write_bytes(s.log)
    assert b'\xe2\x96\x88' in s.log, 'music spectrum bars absent'
    s.send(b'j');s.read(.3)
    s.send(b' ');s.expect(b'[Paused]');s.send(b's');s.expect(b'[Stopped] 00:00');s.finish()
    s=Session('broken','--play','tmp/fixtures/05-broken.png');sessions.append(s)
    s.expect(b'[Error]');s.finish()
    s=Session('browse','tmp/fixtures');sessions.append(s)
    s.expect(b'Parent directory');s.send(b'/video');s.expect(b'Filter: video');s.send(b'\r');s.send(b'\r');s.expect(b'[Playing]');s.finish()
    s=Session('search','tmp/fixtures');sessions.append(s)
    s.expect(b'Search files & folders');s.send(b'\x06fldr');s.expect(b'Matches');s.send(b'\r');s.send(b'\r');s.expect(b'Files \xc2\xb7 1');s.finish()
    s=Session('navigation','tmp/fixtures/folder');sessions.append(s)
    s.expect(b'Parent directory');s.send(b'\r');s.expect(b'01-image.png')
    s.send(b'\r');s.expect(b'Files \xc2\xb7 1');s.send(b'\x7f');s.expect(b'01-image.png');s.finish()
    s=Session('native','--renderer=kitty','tmp/fixtures/07-detail.jpg');sessions.append(s)
    data=s.expect(b'a=t,f=100')
    if b'a=p,' not in data:s.expect(b'a=p,')
    assert b'\xe2\x96\x80' not in s.log, 'native preview fell back to cells'
    s.send(b'?');s.expect(b'a=d,d=I');s.send(b'?');s.expect(b'a=t,f=100')
    s.resize(90,25);s.expect(b'a=p,');s.finish()
    s=Session('native-video','--renderer=kitty','--play','tmp/fixtures/03-video.mp4');sessions.append(s)
    s.expect(b'[Playing]');s.expect(b'00:01 / 00:08')
    assert s.log.count(b'a=t,f=100')>=2, 'native video did not advance frames'
    assert b'\xe2\x96\x80' not in s.log, 'native video used cell pixels'
    s.send(b' ');s.expect(b'[Paused]');s.send(b']');s.expect(b'00:06 / 00:08')
    s.send(b's');s.expect(b'[Stopped] 00:00');s.finish()
    probe_env = dict(TERM_PROGRAM='unknown', KITTY_WINDOW_ID='', TMUX='', STY='')
    s=Session('detect-native','--renderer=auto','tmp/fixtures/07-detail.jpg',env_override=probe_env);sessions.append(s)
    data=s.expect(b'a=q',timeout=8)
    query_id=re.search(rb'\x1b_Gi=(\d+),s=1,v=1,a=q',data).group(1)
    # Fragment the reply and interleave real startup search input.
    os.write(s.master,b'/detail'+b'\x1b_Gi='+query_id+b';')
    time.sleep(.005)
    os.write(s.master,b'OK\x1b\\\r')
    s.expect(b'a=t,f=100')
    assert b'Filter: detail' in re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]',b'',s.log), 'startup keys lost'
    s.send(b'\x1b');s.read(.2);s.finish()
    s=Session('detect-fallback','--renderer=auto','tmp/fixtures/07-detail.jpg',env_override=dict(probe_env,TERM_PROGRAM='ghostty'));sessions.append(s)
    data=s.expect(b'a=q',timeout=8)
    query_id=re.search(rb'\x1b_Gi=(\d+),s=1,v=1,a=q',data).group(1)
    os.write(s.master,b'\x1b_Gi='+query_id+b';EINVAL: unsupported\x1b\\')

    if b'\xe2\x96\x80' not in s.log:s.expect(b'\xe2\x96\x80')
    s.read(.5)
    assert b'a=t,f=100' not in s.log, 'negative query selected native'
    s.finish()
    s=Session('detect-timeout','--renderer=auto','tmp/fixtures/07-detail.jpg',env_override=probe_env);sessions.append(s)
    s.expect(b'a=q',timeout=8)
    if b'\xe2\x96\x80' not in s.log:s.expect(b'\xe2\x96\x80')
    s.read(.5)
    assert b'a=t,f=100' not in s.log, 'unanswered query selected native'
    s.finish()
    s=Session('custom-seek','--seek-step=1s','--play','tmp/fixtures/03-video.mp4');sessions.append(s)
    s.expect(b'[Playing]');s.send(b' ');s.expect(b'[Paused]');s.send(b'0');s.read(.3)
    s.send(b']');s.expect(b'00:01 / 00:08')
    assert b'-1s' in s.log, 'custom jump label missing'
    s.send(b',');s.expect(b'Seek step 5s');s.send(b']');s.expect(b'00:06 / 00:08')
    s.send(b't');s.send(b'\x1b[D');s.expect(b'00:01 / 00:08');s.finish()
    s=Session('mixed','--play','tmp/fixtures/01-image.png','tmp/fixtures/02-animation.gif','tmp/fixtures/03-video.mp4','tmp/fixtures/04-audio.wav');sessions.append(s)
    s.expect(b'[Playing]');s.expect(b'NOW PLAYING  02-animation.gif',timeout=8)
    s.expect(b'NOW PLAYING  03-video.mp4',timeout=6)
    s.send(b' ');s.expect(b'[Paused]');s.send(b'n');s.expect(b'NOW PLAYING  04-audio.wav')
    assert b'[Paused]' in s.log[-30000:]
    s.finish()
    print('PASS: video, image, GIF, audio, corrupt media, filtering, pause/seek/stop, resize, help, terminal restoration, and process cleanup')
finally:
    for s in sessions:
        if s.proc.poll() is None:s.abort()
