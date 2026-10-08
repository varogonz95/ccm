# Installation

ccm is a single static binary for Windows, Linux and macOS (amd64 and arm64).

## Prebuilt binaries

Download the archive for your platform from [Releases](https://github.com/varogonz95/ccm/releases): `ccm_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows), plus `checksums.txt`, which has the SHA-256 of each archive. Each archive unpacks into a directory of the same name holding `ccm` and the README.

Linux:

```sh
sha256sum -c --ignore-missing checksums.txt
tar xzf ccm_0.1.0_linux_amd64.tar.gz
```

macOS:

```sh
grep ccm_0.1.0_darwin_arm64.tar.gz checksums.txt | shasum -a 256 -c -
tar xzf ccm_0.1.0_darwin_arm64.tar.gz
```

Windows (PowerShell): compare the output with the line for your archive in `checksums.txt`, then unzip it.

```powershell
(Get-FileHash ccm_0.1.0_windows_amd64.zip -Algorithm SHA256).Hash
Expand-Archive ccm_0.1.0_windows_amd64.zip .
```

## From source

Requires Go 1.22+.

```sh
git clone https://github.com/varogonz95/ccm.git
cd ccm
make build        # produces ./ccm (or: go build -o ccm ./cmd/ccm)
```

## Every platform at once

```sh
make dist         # needs Go
make docker-dist  # needs Docker with buildx, no Go
```

Both write to `dist/`:

```
ccm-darwin-amd64   ccm-linux-amd64   ccm-windows-amd64.exe
ccm-darwin-arm64   ccm-linux-arm64   ccm-windows-arm64.exe
```

`make docker-dist` runs the same `make dist` inside a `golang` container and copies only the binaries out. Builds are `CGO_ENABLED=0`, trimmed and stripped.

## Put it on each machine

Copy the matching binary to every machine that should host sessions and to the machine you drive from (the same binary does both). Rename it to `ccm` (or `ccm.exe`) and put it on your `PATH`.

Machines that host sessions also need Claude Code installed, with `claude` on the `PATH` of the user running the agent (or pass its full path with `ccm agent --claude`).

**Windows:** the first time `ccm agent` runs, allow `ccm.exe` through Windows Defender Firewall for private networks.

Next: [Quick start](Quick-Start.md).
