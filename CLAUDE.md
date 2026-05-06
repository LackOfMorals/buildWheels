# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project does

`buildwheels` is a Go CLI that fetches pre-built binary archives from GitHub Releases and packages them as platform-specific Python wheels (`.whl` files) for distribution on PyPI. The goal is to let users install any GoReleaser-distributed binary via `pip`/`pipx`/`uv` without a Go toolchain.

Go module: `github.com/neo4j-labs/buildwheels`. No external dependencies — stdlib only.

## Commands

```bash
# Build
go build -o buildwheels

# Test
go test ./...
go test -v -run TestBuildWheel  # single test

# Lint (requires golangci-lint)
golangci-lint run

# Run
go run . -repo owner/repo -binary-name mybinary
go run . -repo owner/repo -platforms linux_x86_64,darwin_arm64
PYPI_TOKEN=pypi-xxxx go run . -repo owner/repo -upload
go run . -repo owner/repo -debug 2>build.log

# Release: push a tag — GoReleaser handles the rest via GitHub Actions
git tag v1.2.3 && git push --tags
```

## Architecture

The tool runs a linear pipeline orchestrated in `main.go`:

1. **Config** (`config.go`) — CLI flags parsed into a `Config` struct
2. **Asset resolution** (`platform.go`) — maps GoReleaser `OS_Arch` keys to Python wheel platform tags and selects matching GitHub release assets
3. **Download + extract** (`download.go`, `archive.go`) — fetches `.tar.gz`/`.zip` archives with optional disk caching; extracts the target binary
4. **Wheel construction** (`wheel.go`) — builds a compliant `.whl` (zip) containing the binary, a Python shim package, and PEP 566 METADATA
5. **Upload** (`pypi.go`) — optional push to PyPI-compatible index via legacy multipart upload

Supporting files: `github.go` (GitHub REST API), `files.go` (license/description resolution from local path or fetched URL), `log.go` (structured `slog` logging, all output to stderr).

### Platform mapping (platform.go)

Six supported targets (`OS_Arch` → Python wheel tag):

| GoReleaser key   | Wheel platform tag            | Archive |
|------------------|-------------------------------|---------|
| `linux_x86_64`   | `manylinux_2_17_x86_64`       | tar.gz  |
| `linux_arm64`    | `manylinux_2_17_aarch64`      | tar.gz  |
| `darwin_x86_64`  | `macosx_10_9_x86_64`          | tar.gz  |
| `darwin_arm64`   | `macosx_11_0_arm64`           | tar.gz  |
| `windows_x86_64` | `win_amd64`                   | zip     |
| `windows_arm64`  | `win_arm64`                   | zip     |

### Wheel construction constraints (wheel.go)

Wheels must work with strict installers (`uv`, older `pip`):
- **No data descriptors** — `Flags=0` in each zip local file header; PyPI rejects wheels that use them
- **CRC32 + size in both 32-bit and 64-bit fields** — prevents zip64 extensions that confuse some installers
- **RECORD file written last** — two-pass build: collect entries, compute SHA-256, write RECORD
- **Wheel tag**: `py30-none-<platform>` (any CPython 3.x)
- **SPDX license expression** required in METADATA (e.g., `MIT`, `Apache-2.0`)

### Shim package

Each wheel includes a minimal Python package:
- `_shim.py` — uses `os.execv` on Unix, `subprocess` on Windows to exec the bundled binary
- `__init__.py` — version metadata
- Registered via `console_scripts` entry point so `pip`/`pipx`/`uv` wire up the binary on install

### Tests

All tests are offline — `httptest` servers stand in for GitHub and PyPI APIs. No fixtures or network access required. Tests live alongside source as `*_test.go`.
