#!/usr/bin/env python3
"""Real TUI sharing actions using isolated clipboard/helper executables."""
import json
import os
from pathlib import Path
import sys
import time
from tui_session import Session, ROOT

folder = ROOT / 'tmp/sharing-fixtures'
folder.mkdir(parents=True, exist_ok=True)
path = folder / "space ' snow 雪.txt"
path.write_text('SHARING_FIXTURE\n')
tools = folder / 'tools'
tools.mkdir(exist_ok=True)
clipboard = folder / 'clipboard'
args_file = folder / 'args.json'
clipboard.unlink(missing_ok=True)
args_file.unlink(missing_ok=True)
helper = '#!' + sys.executable + '\nimport sys,time,json\nfrom pathlib import Path\n'
for name in ('pbcopy', 'xclip', 'wl-copy'):
    (tools / name).write_text(helper + f'Path({str(clipboard)!r}).write_bytes(sys.stdin.buffer.read())\n')
    (tools / name).chmod(0o700)
# This tests the handoff and cancellation, without opening a real desktop window.
for name in ('osascript', 'ripdrag'):
    (tools / name).write_text(helper + f'Path({str(args_file)!r}).write_text(json.dumps(sys.argv[1:]))\ntime.sleep(30)\n')
    (tools / name).chmod(0o700)
env = {'PATH': str(tools) + os.pathsep + os.environ['PATH'], 'SSH_CONNECTION':'', 'SSH_TTY':'', 'WAYLAND_DISPLAY':'peep-fixture', 'DISPLAY':':99'}
# Retain externally supplied binary, so the same test can run in Linux containers.
s = Session('desktop-sharing', str(path), env_override=env)
try:
    s.expect(b'SHARING_FIXTURE')
    s.send(b'y'); s.expect(b'Copied path')
    assert clipboard.read_bytes() == os.fsencode(path)
    s.send(b'Y'); s.expect(b'select an image')
    assert clipboard.read_bytes() == os.fsencode(path), 'invalid image changed clipboard'
    s.send(b'\x07'); s.expect(b'Opening drag window')
    deadline = time.monotonic() + 5
    while not args_file.exists() and time.monotonic() < deadline: s.read(.1)
    assert json.loads(args_file.read_text())[-1] == str(path)
    s.send(b'\x07'); s.expect(b'Drag window closed')
    s.send(b'/copyY'); s.read(.2)
    assert clipboard.read_bytes() == os.fsencode(path), 'filter typing triggered sharing'
    s.send(b'\x1b'); s.read(.3)
    s.send(b'\x07'); s.expect(b'Opening drag window')
    s.finish() # Must reap drag process and restore the terminal on quit.
    print('PASS: path payload, invalid image, drag handoff/cancel, input modes, exit cleanup')
except BaseException:
    s.abort()
    raise
