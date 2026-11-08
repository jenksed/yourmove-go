#!/usr/bin/env python3
"""Build one local NSIS candidate using pinned Wails generators; never publish."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
HERE = ROOT / "release/windows"


def sha256(path):
    with path.open("rb") as source:
        return hashlib.file_digest(source, "sha256").hexdigest()


def capture(*args):
    try:
        return subprocess.check_output(args, cwd=ROOT, text=True, stderr=subprocess.STDOUT).strip()
    except OSError as error:
        raise RuntimeError(f"cannot execute {args[0]!r}: {error}") from error


def validate_product(product):
    if not re.fullmatch(r"\d+\.\d+\.\d+", product["version"]):
        raise ValueError("version must be a three-part numeric release version")
    numbers = [int(n) for n in product["version"].split(".")] + [product["build"]]
    if any(type(n) is not int or n < 0 or n > 65535 for n in numbers):
        raise ValueError("Windows version components must be integers in 0..65535")
    # Values enter NSIS definitions and Go linker flags. Reject code/substitutions.
    for key in ("name", "publisher", "identifier", "description"):
        if not re.fullmatch(r"[A-Za-z0-9 .(),'-]+", product[key]):
            raise ValueError(f"Unsupported metadata characters in {key}")
    if product["name"] != "YourMove" or product["identifier"] != "com.yourmove.go":
        raise ValueError("Changing installation identity requires a reviewed migration")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", type=Path, help="absolute NEW directory outside the repository")
    parser.add_argument("--sign", action="store_true", help="use existing Windows certificate-store identity")
    args = parser.parse_args()
    product = json.loads((HERE / "product.json").read_text())
    validate_product(product)
    output = args.output
    if not output.is_absolute():
        parser.error("output must be absolute")
    output = output.resolve()
    if output == ROOT or ROOT in output.parents or output.exists():
        parser.error("output must be new and outside the repository")
    if capture("git", "status", "--porcelain"):
        parser.error("commit source changes before building a candidate")
    if list(ROOT.glob("*.syso")):
        parser.error("unexpected root .syso resource; refusing to combine or overwrite it")
    for tool in ("go", "node", "pnpm", "makensis", "git"):
        if not shutil.which(tool):
            parser.error(f"missing build prerequisite: {tool}")
    wails = os.environ.get("WAILS3", "wails3")
    if not shutil.which(wails):
        parser.error(f"missing build prerequisite: {wails} (install pinned Wails CLI v3.1.0 or set WAILS3)")
    if capture(wails, "version") != "v3.1.0":
        parser.error("install the pinned Wails CLI v3.1.0 (or set WAILS3)")
    if capture("pnpm", "--version") != "10.14.0":
        parser.error("pnpm 10.14.0 is required")
    if capture("node", "--version") != "v" + (ROOT / "frontend/.nvmrc").read_text().strip():
        parser.error("Node must match frontend/.nvmrc")
    signing_script = HERE / "sign.ps1"
    if args.sign:
        if os.name != "nt":
            parser.error("certificate-store signing and verification require Windows")
        for key in ("YOURMOVE_WINDOWS_CERT_THUMBPRINT", "YOURMOVE_WINDOWS_TIMESTAMP_URL", "YOURMOVE_WINDOWS_SIGNER_SUBJECT"):
            if not os.environ.get(key):
                parser.error(f"missing signing prerequisite: {key}")
        if not shutil.which("signtool.exe"):
            parser.error("Windows SDK signtool.exe must be on PATH")

    output.mkdir(parents=True)
    log = (output / "build.log").open("w", encoding="utf-8")

    def run(*command, cwd=ROOT, env=None):
        print("Running:", command[0], command[1] if len(command) > 1 else "", flush=True)
        log.write("\n" + repr(command) + "\n")
        log.flush()
        subprocess.run(command, cwd=cwd, env=env, check=True, stdout=log, stderr=subprocess.STDOUT)

    def sign(path):
        run("powershell.exe", "-NoProfile", "-NonInteractive", "-File", str(signing_script), str(path))

    commit = capture("git", "rev-parse", "HEAD")
    (output / "source-commit.txt").write_text(commit + "\n")
    manifest_before = capture(sys.executable, str(ROOT / "release/macos/source_manifest.py"))
    (output / "source-manifest.json").write_text(manifest_before + "\n")
    run("pnpm", "install", "--frozen-lockfile", cwd=ROOT / "frontend")
    run("pnpm", "run", "build", cwd=ROOT / "frontend")
    assets = output / "assets"
    run(wails, "generate", "build-assets", "-dir", str(assets), "-name", product["name"],
        "-binaryname", product["name"], "-productname", product["name"],
        "-productidentifier", product["identifier"], "-productversion", product["version"],
        "-productcompany", product["publisher"], "-productdescription", product["description"],
        "-productcopyright", "Copyright YourMove contributors", "-productcomments",
        "Windows release candidate" if args.sign else "Unsigned qualification candidate", "-silent")
    windows = assets / "windows"
    run(wails, "generate", "icons", "-input", str(ROOT / "release/macos/YourMove-icon-1024.png"),
        "-windowsfilename", str(windows / "icon.ico"), "-macfilename", str(assets / "icon.icns"))
    file_version = product["version"] + "." + str(product["build"])
    info = json.loads((windows / "info.json").read_text())
    info["fixed"]["file_version"] = file_version
    info["fixed"]["product_version"] = file_version
    info["info"]["0409"]["FileVersion"] = file_version
    info["info"]["0409"]["OriginalFilename"] = "YourMove.exe"
    (windows / "info.json").write_text(json.dumps(info, indent=2))
    # Wails generates a Win32 manifest, not an MSIX identity.
    manifest = windows / "wails.exe.manifest"
    manifest.write_text(manifest.read_text().replace('version="' + product["version"] + '"', 'version="' + file_version + '"'))
    resource = output / "yourmove_windows_amd64.syso"
    run(wails, "generate", "syso", "-arch", "amd64", "-icon", str(windows / "icon.ico"),
        "-manifest", str(manifest), "-info", str(windows / "info.json"), "-out", str(resource))
    executable = output / "YourMove.exe"
    environment = dict(os.environ, GOOS="windows", GOARCH="amd64", CGO_ENABLED="0")
    flags = ("-s -w -H windowsgui -X github.com/jenksed/yourmove-go/internal/app.AppVersion="
             + product["version"] + " -X github.com/jenksed/yourmove-go/internal/app.AppBuild=" + str(product["build"]))
    # Go needs a physical .syso in the main package. Own it exclusively, remove even
    # on build failure, and never alter an existing resource or another worktree.
    temporary_resource = ROOT / "yourmove_packaging_windows_amd64.syso"
    target = temporary_resource.open("xb")
    try:
        with target:
            target.write(resource.read_bytes())
        run("go", "build", "-trimpath", "-buildvcs=true", "-tags", "desktop,production", "-ldflags", flags,
            "-o", str(executable), ".", env=environment)
    finally:
        temporary_resource.unlink()
    if args.sign:
        sign(executable)

    nsis = windows / "nsis"
    shutil.copyfile(HERE / "project.nsi", nsis / "project.nsi")
    shutil.copyfile(ROOT / "LICENSE", windows / "LICENSE.txt")
    shutil.copyfile(HERE / "INSTALL-README.txt", windows / "README.txt")
    run(wails, "generate", "webview2bootstrapper", "-dir", str(nsis))
    filename = f"YourMove-{product['version']}-windows-x64" + (".exe" if args.sign else "-unsigned.exe")
    installer = output / filename
    # NSIS interprets substitutions even inside quotes. Reject unsafe build paths.
    for path in (output, signing_script):
        if any(c in str(path) for c in ('$','"','\n','\r','\x00')):
            raise ValueError("build paths cannot contain NSIS substitutions, quotes, or control characters")
    command = ["makensis", "-V3", "-DARG_WAILS_AMD64_BINARY=" + str(executable),
               "-DYOURMOVE_FILEVERSION=" + file_version, "-DYOURMOVE_INSTALLER=" + str(installer)]
    if args.sign:
        command.append("-DYOURMOVE_SIGN_SCRIPT=" + str(signing_script))
    run(*command, "project.nsi", cwd=nsis)
    if args.sign:
        sign(installer)
    if not installer.is_file() or installer.stat().st_size == 0:
        raise RuntimeError("NSIS did not produce an installer")
    manifest_after = capture(sys.executable, str(ROOT / "release/macos/source_manifest.py"))
    (output / "source-manifest-after.json").write_text(manifest_after + "\n")
    if manifest_before != manifest_after:
        raise RuntimeError("source changed during build; reject this candidate")
    (output / "build-info.txt").write_text(capture("go", "version", "-m", str(executable)) + "\n")
    # Matching source archive follows the existing macOS release convention.
    source_zip = output / f"YourMove-{product['version']}-source.zip"
    run("git", "archive", "--format=zip", "-o", str(source_zip), "HEAD")
    metadata = dict(product, source_commit=commit, architecture="x64", os="windows",
                    installer=filename, installer_sha256=sha256(installer),
                    signing="timestamped-authenticode" if args.sign else "unsigned-qualification-only",
                    native_acceptance="NOT VERIFIED", browser="separately installed Google Chrome",
                    webview2="Evergreen bootstrapper; internet required if runtime missing",
                    tools={"go": capture("go", "version"), "wails": capture(wails, "version"),
                           "nsis": capture("makensis", "-VERSION"), "node": capture("node", "--version"),
                           "pnpm": capture("pnpm", "--version")})
    (output / "candidate.json").write_text(json.dumps(metadata, indent=2) + "\n")
    winget = output / "winget-unpublished"
    winget.mkdir()
    for template in (HERE / "winget").glob("*.yaml.in"):
        content = template.read_text().replace("@VERSION@", product["version"]).replace("@PUBLISHER@", product["publisher"])
        content = content.replace("@INSTALLER_SHA256@", sha256(installer).upper())
        (winget / template.name).write_text(content)
    names = [filename, executable.name, source_zip.name, "source-manifest.json", "candidate.json",
             "assets/windows/nsis/MicrosoftEdgeWebview2Setup.exe"]
    (output / "SHA256SUMS").write_text("".join(f"{sha256(output / n)}  {n}\n" for n in names))
    log.close()
    print(json.dumps(metadata, indent=2))
    print("Local candidate only. No publication or Windows native acceptance performed.")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, RuntimeError, subprocess.CalledProcessError) as error:
        sys.exit(str(error) + "\nBuild failed; inspect build.log. Do not distribute incomplete output.")
