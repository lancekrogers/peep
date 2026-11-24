#!/usr/bin/env python3
"""Exercise the real AppKit drag handler and its native file-URL pasteboard.

The synthetic mouse event stays inside the helper process; no accessibility
permission, external app, or user clipboard is needed. This checks drag session
creation and payload, not a physical drop into an arbitrary destination app.
"""
from pathlib import Path
import shutil
import subprocess
import tempfile
import time

root = Path(__file__).resolve().parents[1]
helper = root / 'internal/desktop/drag_macos.js'
fixture = root / 'examples/night-shift/01-images/01-moonrise.jpg'
process = subprocess.Popen(['osascript', '-l', 'JavaScript', str(helper), str(fixture)],
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
try:
    time.sleep(3)
    assert process.poll() is None, process.communicate()
finally:
    if process.poll() is None:
        process.terminate()
        process.communicate(timeout=5)

# Invoke the production mouseDown: handler with a native event. The previous
# JS-array bridge crashed here even though starting the window passed.
probe = r'''
    if (!view.acceptsFirstMouse(null)) throw Error('First click cannot drag');
    if (view.mouseDownCanMoveWindow) throw Error('Icon can move the window');
    var point = $.NSMakePoint(180, 110);
    var down = $.NSEvent.mouseEventWithTypeLocationModifierFlagsTimestampWindowNumberContextEventNumberClickCountPressure(
        $.NSEventTypeLeftMouseDown, point, 0, 0, window.windowNumber, null, 1, 1, 1);
    var up = $.NSEvent.mouseEventWithTypeLocationModifierFlagsTimestampWindowNumberContextEventNumberClickCountPressure(
        $.NSEventTypeLeftMouseUp, point, 0, .1, window.windowNumber, null, 2, 1, 0);
    app.postEventAtStart(up, false);
    view.mouseDown(down);
    var board = $.NSPasteboard.pasteboardWithName($.NSDragPboard);
    var value = ObjC.unwrap(board.stringForType($.NSPasteboardTypeFileURL));
    if (value !== ObjC.unwrap(url.absoluteString)) throw Error('Wrong drag file URL: ' + value);
    if (view.draggingSessionSourceOperationMaskForDraggingContext(null, 0) !== $.NSDragOperationCopy)
        throw Error('Drag must offer copy, not move');
    console.log('PASS: native file drag session and exact URL');
'''
source = helper.read_text()
assert source.count('    app.run;') == 1
with tempfile.TemporaryDirectory(prefix='peep-drag-test-') as temp:
    folder = Path(temp)
    script = folder / 'probe.js'
    script.write_text(source.replace('    app.run;', probe))
    image = folder / "screenshot ' snow 雪.jpg"
    shutil.copyfile(fixture, image)
    directory = folder / "folder ' snow 雪"
    directory.mkdir()
    for path in (image, directory):
        result = subprocess.run(['osascript', '-l', 'JavaScript', str(script), str(path)],
                                capture_output=True, text=True, timeout=10)
        assert result.returncode == 0, result.stderr
        assert 'PASS: native file drag session and exact URL' in result.stderr, result.stderr
        assert path.exists(), 'Drag removed source'
print('PASS: window lifecycle, first-click drag, native file/folder sessions, exact file URLs, copy-only operation')
