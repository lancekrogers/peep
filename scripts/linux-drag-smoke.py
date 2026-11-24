#!/usr/bin/env python3
"""Drag from peep into a local GTK receiver under an isolated Xvfb display."""
import os
from pathlib import Path
import subprocess
import sys
import time
from urllib.parse import unquote, urlparse
from tui_session import Session, ROOT

folder = ROOT / 'tmp/linux-drag-fixtures'
folder.mkdir(parents=True, exist_ok=True)
path = folder / "screenshot ' 雪.png"
# A synthetic image avoids touching user files or any external application.
subprocess.run(['ffmpeg','-v','error','-f','lavfi','-i','color=c=teal:s=80x60',
                '-frames:v','1','-threads','1','-y',str(path)],check=True)
received = folder / 'received.txt'
received.unlink(missing_ok=True)
sink = folder / 'sink.py'
sink.write_text('''import gi,sys
from pathlib import Path
gi.require_version('Gtk','3.0')
from gi.repository import Gtk,Gdk
window=Gtk.Window(title='Peep isolated drop receiver')
window.set_default_size(320,220)
window.move(600,100)
label=Gtk.Label(label='Drop synthetic image here')
window.add(label)
window.drag_dest_set(Gtk.DestDefaults.ALL,[Gtk.TargetEntry.new('text/uri-list',0,0)],Gdk.DragAction.COPY)
def receive(widget,context,x,y,data,info,timestamp):
    Path(sys.argv[1]).write_text('\\n'.join(data.get_uris()))
    Gtk.drag_finish(context,True,False,timestamp)
window.connect('drag-data-received',receive)
window.show_all()
Gtk.main()
''')
receiver = subprocess.Popen(['/usr/bin/python3',str(sink),str(received)])
session = Session('linux-native-drag',str(path),env_override={'SSH_CONNECTION':'','SSH_TTY':'','WAYLAND_DISPLAY':''})

def xdo(*args):
    return subprocess.check_output(['xdotool',*map(str,args)],text=True).strip()
def window(title):
    until=time.monotonic()+8
    while time.monotonic()<until:
        result=subprocess.run(['xdotool','search','--onlyvisible','--name',title],capture_output=True,text=True)
        if result.returncode==0: return result.stdout.splitlines()[-1]
        session.read(.1)
    raise AssertionError('missing window: '+title)

try:
    session.expect(b'screenshot')
    target=window('^Peep isolated drop receiver$')
    session.send(b'\x07')
    source=window('^dragon$')
    xdo('windowmove',source,80,100)
    geometry=dict(line.split('=',1) for line in xdo('getwindowgeometry','--shell',source).splitlines())
    start=(int(geometry['X'])+int(geometry['WIDTH'])//2,int(geometry['Y'])+int(geometry['HEIGHT'])//2)
    xdo('mousemove','--sync',*start)
    xdo('mousedown',1)
    for step in range(1,21):
        xdo('mousemove','--sync',int(start[0]+(750-start[0])*step/20),int(start[1]+(200-start[1])*step/20))
        time.sleep(.05)
    xdo('mouseup',1)
    until=time.monotonic()+8
    while not received.exists() and time.monotonic()<until: session.read(.1)
    uri=received.read_text().strip()
    assert unquote(urlparse(uri).path)==str(path),uri
    assert path.is_file(), 'drag removed original'
    session.finish()
    print('PASS: peep -> native dragon window -> GTK receiver, exact file URI and original retained')
except BaseException:
    session.abort()
    raise
finally:
    receiver.terminate()
    receiver.wait(timeout=5)
