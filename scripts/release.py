import argparse
import hashlib
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import zipfile


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("version")
    args = parser.parse_args()
    if not re.fullmatch(r"v\d+\.\d+\.\d+", args.version):
        parser.error("version must be vMAJOR.MINOR.PATCH")
    root = Path(__file__).resolve().parent.parent
    subprocess.run(["pnpm", "--dir", "web", "install", "--frozen-lockfile"], cwd=root, check=True)
    subprocess.run(["pnpm", "--dir", "web", "build"], cwd=root, check=True)
    output = root / "bin" / "releases" / args.version
    output.mkdir(parents=True, exist_ok=True)
    assets = []
    for goos in ("linux", "darwin", "windows"):
        for goarch in ("amd64", "arm64"):
            name = f"cloud_{args.version}_{goos}_{goarch}"
            with tempfile.TemporaryDirectory(prefix=name) as temporary:
                directory = Path(temporary)
                binary = directory / ("cloud.exe" if goos == "windows" else "cloud")
                env = dict(os.environ, CGO_ENABLED="0", GOOS=goos, GOARCH=goarch)
                print(f"Building {name}", flush=True)
                subprocess.run(
                    ["go", "build", "-trimpath", "-ldflags=-s -w", "-o", str(binary), "./cmd/cloud"],
                    cwd=root, env=env, check=True,
                )
                files = [(binary, binary.name), (root / "LICENSE", "LICENSE"),
                         (root / "docs" / "releases" / f"{args.version}.md", "README.md")]
                if goos == "windows":
                    archive = output / f"{name}.zip"
                    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as bundle:
                        for source, destination in files:
                            bundle.write(source, destination)
                else:
                    archive = output / f"{name}.tar.gz"
                    with tarfile.open(archive, "w:gz") as bundle:
                        for source, destination in files:
                            bundle.add(source, arcname=destination)
                assets.append(archive)
    checksums = []
    for asset in sorted(assets):
        with asset.open("rb") as source:
            checksum = hashlib.file_digest(source, "sha256").hexdigest()
        checksums.append(f"{checksum}  {asset.name}\n")
    (output / "SHA256SUMS").write_text("".join(checksums))
    print(f"Release archives and SHA256SUMS: {output}")


if __name__ == "__main__":
    main()
