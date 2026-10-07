# Installation

clawsh is a single static binary for Windows, Linux and macOS (amd64 and arm64).

## Prebuilt binaries

Download the archive for your platform from [Releases](https://github.com/varogonz95/clawsh/releases): `clawsh_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows). `checksums.txt` has the SHA-256 of each archive.

```sh
sha256sum -c --ignore-missing checksums.txt
tar xzf clawsh_0.1.0_linux_amd64.tar.gz
```

## From source

Requires Go 1.22+.

```sh
git clone https://github.com/varogonz95/clawsh.git
cd clawsh
make build        # produces ./clawsh (or: go build -o clawsh ./cmd/clawsh)
```

## Every platform at once

```sh
make dist         # needs Go
make docker-dist  # needs Docker with buildx, no Go
```

Both write to `dist/`:

```
clawsh-darwin-amd64   clawsh-linux-amd64   clawsh-windows-amd64.exe
clawsh-darwin-arm64   clawsh-linux-arm64   clawsh-windows-arm64.exe
```

`make docker-dist` runs the same `make dist` inside a `golang` container and copies only the binaries out. Builds are `CGO_ENABLED=0`, trimmed and stripped.

## Put it on each machine

Copy the matching binary to every machine that should host sessions and to the machine you drive from (the same binary does both). Rename it to `clawsh` (or `clawsh.exe`) and put it on your `PATH`.

Machines that host sessions also need Claude Code installed, with `claude` on the `PATH` of the user running the agent (or pass its full path with `clawsh agent --claude`).

**Windows:** the first time `clawsh agent` runs, allow `clawsh.exe` through Windows Defender Firewall for private networks.

Next: [Quick start](Quick-Start.md).
