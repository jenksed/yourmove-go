"""Hash the actual accepted working tree, including uncommitted source/assets."""
import hashlib
import json
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parents[2]
paths = subprocess.check_output(
    ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], cwd=root
).split(b"\0")
files = {}
for raw in sorted(set(paths)):
    if not raw:
        continue
    name = raw.decode()
    if name.startswith((".serena/", "graft/", ".agents/", ".codex/")):
        continue
    path = root / name
    if path.is_file():
        files[name] = hashlib.sha256(path.read_bytes()).hexdigest()
json.dump({
    "git_head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root).decode().strip(),
    "dirty": bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=root)),
    "sha256": files,
}, sys.stdout, indent=2, sort_keys=True)
print()
