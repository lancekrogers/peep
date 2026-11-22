#!/usr/bin/env python3
"""Document previews in a real PTY. Synthetic files only."""
import os
import re
import shutil
from tui_session import Session, ROOT, ARTIFACTS

folder=ROOT/'tmp/document-fixtures'
folder.mkdir(exist_ok=True)
(folder/'README.md').write_text('# Document preview\n\nA **bold** paragraph.\n\n'+''.join(f'## Section {i}\n\nBody {i}: readable text.\n\n' for i in range(80)))
(folder/'binary.dat').write_bytes(b'\x00\xff\x00')
(folder/'notes').write_text('Plain text without an extension.\n')
fifo=folder/'special.md'
if not fifo.exists():os.mkfifo(fifo)
shutil.copyfile(ROOT/'tmp/fixtures/04-audio.wav',folder/'song.wav')
sessions=[]
try:
 s=Session('document',str(folder/'README.md'));sessions.append(s)
 s.expect(b'Markdown');s.send(b'\r');s.expect(b'READ')
 s.send(b'\x1b[6~');s.read(.3)
 s.send(b'R');s.expect(b'Source');s.send(b'R');s.expect(b'Markdown')
 s.resize(80,22);s.read(.3);s.send(b'\x1b');s.expect(b'BROWSE');s.finish()
 s=Session('document-binary',str(folder/'binary.dat'));sessions.append(s)
 s.expect(b'binary or non-UTF-8');s.finish()
 s=Session('document-special',str(fifo));sessions.append(s)
 s.expect(b'no preview for special files');s.finish()
 s=Session('document-audio','--play',str(folder/'song.wav'));sessions.append(s)
 s.expect(b'[Playing]');s.send(b'/README');s.expect(b'Filter: README');s.send(b'\r');s.send(b'\r');s.expect(b'READ')
 s.send(b' ');s.expect(b'[Paused]');s.send(b']');s.read(.4)
 clean=re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]',b'',s.log)
 assert b'song.wav' in clean and b'Markdown' in clean
 s.send(b'\x1b');s.read(.2);s.finish()
 print('PASS: Markdown focus/scroll/source/resize, binary/FIFO fallback, and audio transport while reading')
finally:
 for s in sessions:
  if s.proc.poll() is None:s.abort()
