#!/usr/bin/env python3
"""Private synthetic-only native launch/verification. Never selects UI by app name.

Compile native-window-owner.swift first. launch requires a fresh evidence path;
verify must pass before and after every UI transition, followed by an AX title
check against window_title. A title/bundle ID alone never grants interaction.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import time


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def check_identity(expected, record, owner, lifetime, open_db):
    for key in ('run_id', 'workspace_marker', 'pid', 'executable', 'sha256', 'working_directory', 'data_root', 'database', 'resumes'):
        if record.get(key) != expected.get(key):
            raise RuntimeError('candidate identity mismatch: ' + key)
    if record.get('window_title') != f"YourMove · {expected['run_id']} · PID {expected['pid']}":
        raise RuntimeError('window title does not match launch identity')
    if owner['pid'] != expected['pid'] or owner['executable'] != expected['executable']:
        raise RuntimeError('window owner executable/PID mismatch')
    if lifetime != expected['lifetime']:
        raise RuntimeError('process lifetime mismatch')
    # Window titles may be hidden without Screen Recording permission; fail closed.
    matches = [w for w in owner['windows'] if w['owner_pid'] == expected['pid'] and w['title'] == record['window_title'] and w['layer'] == 0 and w['onscreen']]
    if len(matches) != 1:
        raise RuntimeError('no unique visible window owned by the candidate PID and run ID')
    if not open_db:
        raise RuntimeError('candidate does not have the selected database open')
    return matches[0]['window_id']


def lifetime(pid):
    return subprocess.check_output(['/bin/ps', '-p', str(pid), '-o', 'lstart='], text=True).strip()


def verify(evidence, helper):
    expected = json.loads((evidence / 'launch.json').read_text())
    root = Path(expected['data_root'])
    record = json.loads((root / ('.yourmove-native-' + expected['run_id'] + '.json')).read_text())
    if (root / '.yourmove-synthetic-marker').read_text().strip() != expected['workspace_marker']:
        raise RuntimeError('workspace marker changed')
    if sha(expected['executable']) != expected['sha256']:
        raise RuntimeError('executable changed')
    owner = json.loads(subprocess.check_output([helper, str(expected['pid'])], text=True))
    opened = subprocess.check_output(['/usr/sbin/lsof', '-a', '-p', str(expected['pid']), '-FpnDi', expected['database']], text=True)
    stat = Path(expected['database']).stat()
    # lsof reports the opened vnode inode, compared against the selected DB.
    open_db = ('i' + str(stat.st_ino)) in opened.splitlines()
    window = check_identity(expected, record, owner, lifetime(expected['pid']), open_db)
    result = {'verified_at': time.time(), 'window_id': window, 'owner': owner,
              'database_device': stat.st_dev, 'database_inode': stat.st_ino,
              'resumes_device': Path(expected['resumes']).stat().st_dev,
              'resumes_inode': Path(expected['resumes']).stat().st_ino,
              'window_title': record['window_title'], 'result': 'PASS'}
    with (evidence / 'verification.jsonl').open('a') as f:
        f.write(json.dumps(result) + '\n')
    print(json.dumps(result, indent=2))


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['launch', 'verify'])
    p.add_argument('--evidence', type=Path, required=True)
    p.add_argument('--helper', required=True)
    p.add_argument('--executable', type=Path)
    p.add_argument('--sha256')
    p.add_argument('--workspace', type=Path)
    args = p.parse_args()
    evidence = args.evidence.resolve()
    if args.action == 'launch':
        exe = args.executable.resolve(strict=True)
        root = args.workspace.resolve(strict=True)
        if sha(exe) != args.sha256:
            raise RuntimeError('unexpected executable SHA-256')
        # Harness may only launch within dedicated synthetic fixture roots.
        if not (str(root).startswith('/private/tmp/yourmove-native-') or '/.quality/native-recovery-' in str(root)):
            raise RuntimeError('workspace must be a dedicated synthetic fixture')
        marker = root / '.yourmove-synthetic-marker'
        if not marker.exists():
            if list(root.iterdir()):
                raise RuntimeError('unmarked nonempty workspace refused')
            marker.write_text(secrets.token_hex(16) + '\n')
            marker.chmod(0o600)
        workspace_marker = marker.read_text().strip()
        root.chmod(0o700)
        evidence.mkdir(mode=0o700, parents=True, exist_ok=False)
        # Inventory metadata and hashes are synthetic only; never default storage.
        inventory = {str(f.relative_to(root)): sha(f) for f in root.rglob('*') if f.is_file()}
        (evidence / 'initial-inventory.json').write_text(json.dumps(inventory, indent=2))
        run = secrets.token_hex(16)
        env = os.environ.copy()
        env.update(YOURMOVE_DATA_DIR=str(root), YOURMOVE_NATIVE_RUN_ID=run)
        with (evidence / 'process.log').open('xb') as log:
            child = subprocess.Popen([str(exe)], env=env, cwd=root, stdin=subprocess.DEVNULL,
                                     stdout=log, stderr=log, start_new_session=True)
        expected = dict(run_id=run, workspace_marker=workspace_marker, pid=child.pid,
                        executable=str(exe), sha256=args.sha256, working_directory=str(root),
                        data_root=str(root), database=str(root/'jobs.db'), resumes=str(root/'resumes'),
                        lifetime=lifetime(child.pid), launch_method='direct executable; no LaunchServices')
        (evidence / 'launch.json').write_text(json.dumps(expected, indent=2))
        print(json.dumps(expected, indent=2))
        for _ in range(100):
            if (root / ('.yourmove-native-' + run + '.json')).exists():
                break
            if child.poll() is not None:
                raise RuntimeError('candidate exited during startup; see private process.log')
            time.sleep(0.1)
    if args.action == 'launch':
        for attempt in range(100):
            try:
                verify(evidence, args.helper)
                return
            except RuntimeError as error:
                if str(error) != 'no unique visible window owned by the candidate PID and run ID' or attempt == 99:
                    raise
                time.sleep(0.1)
    else:
        verify(evidence, args.helper)


if __name__ == '__main__':
    main()
