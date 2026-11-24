#!/usr/bin/env python3
"""Start and cancel the built-in AppKit drag window on disposable macOS CI."""
from pathlib import Path
import subprocess
import time

root = Path(__file__).resolve().parents[1]
path = root / 'examples/night-shift/01-images/01-moonrise.jpg'
assert path.is_file()
process = subprocess.Popen(['osascript','-l','JavaScript',str(root/'internal/desktop/drag_macos.js'),str(path)],
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
try:
    time.sleep(3)
    assert process.poll() is None, process.communicate()
finally:
    if process.poll() is None:
        process.terminate()
        process.communicate(timeout=5)
print('PASS: native AppKit drag window starts and terminates; actual drop requires desktop verification')
