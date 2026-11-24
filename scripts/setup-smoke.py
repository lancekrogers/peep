#!/usr/bin/env python3
"""Exercise setup CLI with a fake package manager; never install host packages."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
binary = Path(os.environ.get('PEEP_TEST_BINARY', ROOT / 'bin/peep'))
with tempfile.TemporaryDirectory(prefix='peep-setup-') as temp:
    folder = Path(temp)
    manager = '#!' + sys.executable + '\nimport os,json,sys\nfrom pathlib import Path\n'
    manager += "p=Path(os.environ['PEEP_SETUP_FIXTURE']); (p/'invocation').write_text(json.dumps(sys.argv[1:]))\n"
    manager += "for name in ('ffmpeg','ffprobe','mpv'):\n f=p/name; f.write_text('#!/bin/sh\\nexit 0\\n'); f.chmod(0o700)\n"
    for name in ('brew','apt-get','dnf','pacman','sudo'):
        tool = folder / name
        tool.write_text(manager)
        tool.chmod(0o700)
    env = dict(os.environ, PATH=temp, PEEP_SETUP_FIXTURE=temp)
    def setup(args=(), answer=''):
        return subprocess.run([str(binary),'setup',*args], input=answer, text=True,
                              capture_output=True, env=env, timeout=15)
    for answer in ('','\n','no\n','yes'):
        result = setup(answer=answer)
        assert result.returncode == 0 and 'cancelled' in result.stdout, result
        assert not (folder/'invocation').exists()
    result = setup(answer='yes\n')
    assert result.returncode == 0 and 'Media tools are ready' in result.stdout, result
    assert (folder/'invocation').exists()
    (folder/'invocation').unlink()
    result = setup()
    assert result.returncode == 0 and '[y/N]' not in result.stdout, result
    assert not (folder/'invocation').exists()
    for name in ('ffmpeg','ffprobe','mpv'): (folder/name).unlink()
    result = setup(('--yes',))
    assert result.returncode == 0 and '[y/N]' not in result.stdout, result
    assert (folder/'invocation').exists()
    assert setup(('unexpected-path',)).returncode != 0
    assert not (folder/'bin/peep').exists(), 'setup reinstalled executable'
print('PASS: setup cancel/default/EOF, confirmation, --yes, no-op, argument validation')
