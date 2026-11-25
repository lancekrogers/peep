#!/usr/bin/env python3
"""PDF preview through real poppler in a PTY: native and cell paths, change detection, no disk writes."""
import base64
import os
from pathlib import Path
import re
import struct
import subprocess
import tempfile
from tui_session import Session, ROOT

CELL = (10, 20)
WORK = ROOT/'tmp'
WORK.mkdir(exist_ok=True)

def pdf(pages):
    """A hand-assembled PDF: one coloured page per (colour, words) pair, standard fonts only."""
    out, offsets = bytearray(b'%PDF-1.4\n'), []
    def obj(body):
        offsets.append(len(out))
        out.extend(b'%d 0 obj\n%s\nendobj\n' % (len(offsets), body))
    kids = b' '.join(b'%d 0 R' % (4+2*i) for i in range(len(pages)))
    obj(b'<< /Type /Catalog /Pages 2 0 R >>')
    obj(b'<< /Type /Pages /Kids [%s] /Count %d >>' % (kids, len(pages)))
    obj(b'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>')
    for i, (rgb, words) in enumerate(pages):
        stream = b'%.2f %.2f %.2f rg 0 0 612 792 re f 0 0 0 rg BT /F1 36 Tf 60 600 Td (%s) Tj ET' % (*rgb, words.encode())
        obj(b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>' % (5+2*i))
        obj(b'<< /Length %d >>\nstream\n%s\nendstream' % (len(stream), stream))
    xref = len(out)
    out.extend(b'xref\n0 %d\n0000000000 65535 f \n' % (len(offsets)+1))
    out.extend(b''.join(b'%010d 00000 n \n' % o for o in offsets))
    out.extend(b'trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n' % (len(offsets)+1, xref))
    return bytes(out)

ORIGINAL = pdf([((.9, .3, .3), 'Alpha harbour lights'), ((.3, .8, .4), 'Bravo satellite pass'), ((.3, .4, .9), 'Charlie morning log')])
CHANGED = pdf([((.9, .9, .2), 'Delta rewritten page one'), ((.2, .9, .9), 'Echo rewritten page two'), ((.9, .2, .9), 'Foxtrot rewritten page three')])

def pngs(log):
    """Every complete Kitty PNG upload, in the order peep transmitted them."""
    found, encoded = [], None
    for header, payload in re.findall(rb'\x1b_G([^;\x1b]*);?([^\x1b]*)\x1b\\', log):
        if b'a=t' in header:
            encoded = b''
        if encoded is None or b'a=p' in header or b'a=d' in header:
            continue
        encoded += payload
        if b'm=0' in header:
            found.append(base64.b64decode(encoded))
            encoded = None
    return found

def size(png):
    return struct.unpack('>II', png[16:24])

def pdftoppm(path, page, width, height):
    n = str(page)
    return subprocess.check_output(['pdftoppm', '-png', '-f', n, '-l', n, '-singlefile', '-cropbox', '-scale-dimension-before-rotation',
                                    '-scale-to-x', str(width), '-scale-to-y', str(height), str(path)], timeout=20)

def placements(log):
    return [dict(item.split(b'=') for item in header.split(b',') if b'=' in item) for header in re.findall(rb'\x1b_G(a=p[^;\x1b]*)\x1b\\', log)]

def shown(s, needle, since, timeout=10):
    """Wait for needle anywhere in the output after since, ignoring styling."""
    for _ in range(int(timeout/.1)):
        if needle in re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]', b'', bytes(s.log[since:])): return
        s.read(.1)
    raise AssertionError(f'{s.name}: missing {needle!r}')

def uploaded(s, count, timeout=15):
    """Wait until peep has transmitted at least count complete PNGs."""
    for _ in range(int(timeout/.1)):
        if len(pngs(bytes(s.log))) >= count: return pngs(bytes(s.log))
        s.read(.1)
    raise AssertionError(f'{s.name}: expected {count} PNG uploads')

def poppler_running(path):
    rows = subprocess.check_output(['ps', '-axo', 'command=']).decode(errors='replace').splitlines()
    return [row for row in rows if str(path) in row and re.search(r'\bpdf(info|toppm|totext)\b', row)]

scratch = tempfile.TemporaryDirectory(prefix='pdf-smoke-', dir=WORK)
work = Path(scratch.name)
sessions = []
try:
    docs = work/'docs'; docs.mkdir()
    path = docs/'field.pdf'
    path.write_bytes(ORIGINAL)
    (docs/'notes.txt').write_text('Plain notes beside the PDF.\n')
    isolated = {name: work/name.lower() for name in ('HOME', 'TMPDIR', 'XDG_CACHE_HOME')}
    for folder in isolated.values(): folder.mkdir()
    env = {name: str(folder) for name, folder in isolated.items()}

    # Native: real cell pixels from the PTY, PNG bytes straight from poppler.
    s = Session('pdf-native', '--renderer=kitty', str(path), env_override=env, cell_pixels=CELL); sessions.append(s)
    s.expect(b'PDF \xc2\xb7 page 1/3', timeout=15)
    first = uploaded(s, 1)[-1]; s.read(.5)
    columns = int(placements(bytes(s.log))[-1][b'c'])
    width, height = size(first)
    assert width == columns*CELL[0], (width, columns)
    assert first == pdftoppm(path, 1, width, height), 'page 1 differs from a direct pdftoppm render'
    if os.environ.get('PEEP_PDF_PNG_OUT'):
        Path(os.environ['PEEP_PDF_PNG_OUT']).write_bytes(first)
    s.send(b'\r'); s.expect(b'READ'); s.read(.8)
    before = len(pngs(bytes(s.log)))
    s.send(b'l'); s.expect(b'page 2/3', timeout=10)
    uploads = uploaded(s, before+1); s.read(.8)
    uploads = pngs(bytes(s.log))
    second = uploads[-1]
    assert second == pdftoppm(path, 2, *size(second)), 'page 2 differs from a direct pdftoppm render'
    crops, mark = len(placements(bytes(s.log))), len(s.log)
    s.send(b'jjjj'); s.read(.6)
    moved = [int(p[b'y']) for p in placements(bytes(s.log))[crops:]]
    assert moved and max(moved) > 0, 'scrolling did not move the crop'
    after = re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]', b'', bytes(s.log[mark:]))
    assert b'page 1/3' not in after and b'page 3/3' not in after and len(pngs(bytes(s.log))) == len(uploads), 'scrolling changed the page'

    # Change detection: same path, new content, then a navigation key.
    old_third = pdftoppm(path, 3, *size(second))
    path.write_bytes(CHANGED)
    count = len(pngs(bytes(s.log)))
    s.send(b'l'); s.expect(b'PDF changed on disk', timeout=10)
    uploaded(s, count+1); s.read(.8)
    assert b'page 3/3' in re.sub(rb'\x1b\[[0-?]*[ -/]*[@-~]', b'', bytes(s.log))
    third = pngs(bytes(s.log))[-1]
    assert third == pdftoppm(path, 3, *size(third)) and third != old_third, 'reloaded page does not show the new content'
    s.finish()
    assert not poppler_running(path), 'poppler outlived peep'
    left = [str(p.relative_to(work)) for folder in isolated.values() for p in folder.rglob('*')]
    assert not left, f'files written: {left}'

    # Cells: text by default, R for the half-block picture, search and jumps in text.
    path.write_bytes(ORIGINAL)
    s = Session('pdf-cells', str(path), env_override=env); sessions.append(s)
    for keys, *wanted in ((b'', b'PDF text \xc2\xb7 page 1/3', b'Alpha harbour lights'), (b'\r', b'READ'),
                          (b'R', b'PDF \xc2\xb7 page 1/3', '\u2580'.encode()), (b'l', b'PDF \xc2\xb7 page 2/3'),
                          (b'R', b'PDF text \xc2\xb7 page 2/3', b'Bravo satellite pass'), (b'/satellite', b'Match 1/1'),
                          (b'\r3G', b'page 3/3', b'Charlie morning log')):
        mark = len(s.log)
        if keys: s.send(keys)
        for needle in wanted: shown(s, needle, mark)
    s.finish()

    # Without poppler the PDF explains itself and peep keeps working.
    empty = work/'no-tools'; empty.mkdir()
    s = Session('pdf-no-poppler', str(path), env_override=dict(env, PATH=str(empty))); sessions.append(s)
    shown(s, b'PDF preview needs poppler', 0)
    mark = len(s.log); s.send(b'j'); shown(s, b'Plain notes beside the PDF.', mark)
    s.finish()
    assert not poppler_running(path)
    left = [str(p.relative_to(work)) for folder in isolated.values() for p in folder.rglob('*')]
    assert not left, f'files written: {left}'
    print('PASS: native PNG equals pdftoppm at columns x cell width, page turn, crop scroll, change reload, no files written, cells text/picture/search/jump, missing poppler, cleanup')
finally:
    for s in sessions:
        if s.proc.poll() is None: s.abort()
    scratch.cleanup()
